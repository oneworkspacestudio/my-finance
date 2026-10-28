package finance

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"golang.org/x/crypto/argon2"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxExport = 64 << 20

var backupMagic = []byte("MYFINANCE-BACKUP-1\x00")

type archiveRequest struct {
	Password    string `json:"password"`
	File        []byte `json:"file"`
	CopyToDrive bool   `json:"copyToDrive"`
}

func backupKey(password string, salt []byte) (cipher.AEAD, error) {
	if len(password) < 12 || len(password) > 256 {
		return nil, errors.New("Use a recovery passphrase of 12–256 bytes")
	}
	key := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	defer clear(key)
	block, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(block)
}
func encryptExport(plain []byte, password string) ([]byte, error) {
	if e := validateExport(plain); e != nil {
		return nil, e
	}
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return nil, e
	}
	a, e := backupKey(password, salt)
	if e != nil {
		return nil, e
	}
	nonce := make([]byte, a.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return nil, e
	}
	header := append(append(append([]byte{}, backupMagic...), salt...), nonce...)
	return a.Seal(header, nonce, plain, header), nil
}
func decryptExport(encrypted []byte, password string) ([]byte, error) {
	n := len(backupMagic)
	if len(encrypted) < n+16+12+16 || len(encrypted) > maxExport+4096 || !bytes.Equal(encrypted[:n], backupMagic) {
		return nil, errors.New("Not a supported My Finance backup")
	}
	a, e := backupKey(password, encrypted[n:n+16])
	if e != nil {
		return nil, e
	}
	header := encrypted[:n+16+12]
	plain, e := a.Open(nil, encrypted[n+16:n+28], encrypted[n+28:], header)
	if e != nil {
		return nil, errors.New("The recovery passphrase is wrong or the backup has been changed")
	}
	if e = validateExport(plain); e != nil {
		clear(plain)
		return nil, e
	}
	return plain, nil
}
func validateExport(b []byte) error {
	if len(b) > maxExport {
		return errors.New("Budget export exceeds the 64 MiB preview limit")
	}
	z, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		return errors.New("Choose a native Actual budget ZIP export")
	}
	if len(z.File) != 2 {
		return errors.New("Budget export must contain only db.sqlite and metadata.json; server data and credentials are not accepted")
	}
	seen := map[string]bool{}
	var total uint64
	for _, f := range z.File {
		if (f.Name != "db.sqlite" && f.Name != "metadata.json") || seen[f.Name] || f.Mode()&os.ModeSymlink != 0 {
			return errors.New("Unexpected or duplicate budget export member")
		}
		seen[f.Name] = true
		total += f.UncompressedSize64
		if total > maxExport {
			return errors.New("Uncompressed budget exceeds the preview limit")
		}
		in, e := f.Open()
		if e != nil {
			return e
		}
		content, e := io.ReadAll(io.LimitReader(in, int64(f.UncompressedSize64)+1))
		in.Close()
		if e != nil || len(content) != int(f.UncompressedSize64) {
			return errors.New("Budget export failed its ZIP integrity check")
		}
		if f.Name == "db.sqlite" && !bytes.HasPrefix(content, []byte("SQLite format 3\x00")) {
			return errors.New("Budget export has no SQLite ledger")
		}
		if f.Name == "metadata.json" && !json.Valid(content) {
			return errors.New("Budget metadata is invalid")
		}
	}
	return nil
}
func decodeArchive(w http.ResponseWriter, r *http.Request, in *archiveRequest) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 90<<20))
	d.DisallowUnknownFields()
	if d.Decode(in) != nil {
		apphost.Problem(w, 400, "Choose a budget export/backup and recovery passphrase")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		apphost.Problem(w, 400, "Submit one backup at a time")
		return false
	}
	return true
}
func (a *App) encryptBackup(w http.ResponseWriter, r *http.Request) {
	var in archiveRequest
	if !decodeArchive(w, r, &in) {
		return
	}
	defer clear(in.File)
	result, e := encryptExport(in.File, in.Password)
	in.Password = ""
	if e != nil {
		apphost.Problem(w, 400, e.Error())
		return
	}
	sum := sha256.Sum256(result)
	if in.CopyToDrive {
		if e = a.copyBackup(result); e != nil {
			apphost.Problem(w, 400, e.Error())
			return
		}
	}
	a.h.Audit(apphost.Current(r).User.ID, "finance.encrypted-export", "budget")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="MyFinance.mfb"`)
	w.Header().Set("X-Backup-SHA256", hex.EncodeToString(sum[:]))
	w.Write(result)
}
func (a *App) decryptBackup(w http.ResponseWriter, r *http.Request) {
	var in archiveRequest
	if !decodeArchive(w, r, &in) {
		return
	}
	result, e := decryptExport(in.File, in.Password)
	in.Password = ""
	if e != nil {
		apphost.Problem(w, 400, e.Error())
		return
	}
	defer clear(result)
	a.h.Audit(apphost.Current(r).User.ID, "finance.verified-restore-export", "budget")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="MyFinance-restored-budget.zip"`)
	w.Write(result)
}
func (a *App) copyBackup(b []byte) error {
	root := os.Getenv("MYFINANCE_BACKUP_DIR")
	if !filepath.IsAbs(root) {
		return errors.New("My Drive backup folder has not been configured by Workspace")
	}
	// This is a fixed owner-only destination, never a browser-supplied filesystem path.
	if e := safeAncestors(root); e != nil {
		return e
	}
	if e := os.MkdirAll(root, 0700); e != nil {
		return e
	}
	if e := safeAncestors(root); e != nil {
		return e
	}
	name := "MyFinance-" + time.Now().UTC().Format("20060102T150405.000000000") + ".mfb"
	f, e := os.OpenFile(filepath.Join(root, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return errors.New("My Drive backup could not be created")
	}
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	verify, e := os.ReadFile(filepath.Join(root, name))
	if e != nil || !bytes.Equal(verify, b) {
		return errors.New("My Drive backup failed verification")
	}
	keep := 7
	if n, e := strconv.Atoi(os.Getenv("MYFINANCE_BACKUP_RETENTION")); e == nil && n >= 1 && n <= 366 {
		keep = n
	}
	entries, e := os.ReadDir(root)
	if e != nil {
		return e
	}
	names := []string{}
	for _, v := range entries {
		if !v.IsDir() && strings.HasPrefix(v.Name(), "MyFinance-") && strings.HasSuffix(v.Name(), ".mfb") {
			names = append(names, v.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for _, v := range names[minimum(keep, len(names)):] {
		p := filepath.Join(root, v)
		if e = safeAncestors(p); e != nil {
			return e
		}
		if e = os.Remove(p); e != nil {
			return e
		}
	}
	return nil
}
func minimum(a, b int) int {
	if a < b {
		return a
	}
	return b
}
