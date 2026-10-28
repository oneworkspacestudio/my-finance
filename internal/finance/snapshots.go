package finance

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const maxSnapshot = 128 << 20

var snapshotMagic = []byte("MYFINANCE-SNAPSHOT-1\x00")
var snapshotPath = regexp.MustCompile(`^(?:\.migrate|server-files/account\.sqlite|user-files/(?:file-[0-9a-f-]{36}\.blob|group-[0-9a-f-]{36}\.sqlite))$`)
var snapshotName = regexp.MustCompile(`^MyFinance-server-(?:daily|weekly|monthly)-[0-9TZ-]+-[0-9a-f]{8}\.mfs$`)

type snapshotFile struct {
	Path   string `json:"path"`
	Data   []byte `json:"data"`
	SHA256 string `json:"sha256"`
}
type serverSnapshot struct {
	Schema  int            `json:"schema"`
	Actual  string         `json:"actual"`
	Created string         `json:"created"`
	Files   []snapshotFile `json:"files"`
}
type snapshotSettings struct {
	Enabled        bool   `json:"enabled"`
	CopyToDrive    bool   `json:"copyToDrive"`
	Daily          int    `json:"daily"`
	Weekly         int    `json:"weekly"`
	Monthly        int    `json:"monthly"`
	Next           string `json:"next"`
	LastSuccess    string `json:"lastSuccess"`
	LastError      string `json:"lastError"`
	HasRecoveryKey bool   `json:"hasRecoveryKey"`
}

func (a *App) snapshotRoutes() error {
	if e := a.migrate(2, `CREATE TABLE finance_snapshot_schedule(id INTEGER PRIMARY KEY CHECK(id=1),enabled INTEGER NOT NULL,owner_id TEXT NOT NULL,secret BLOB,copy_drive INTEGER NOT NULL,daily INTEGER NOT NULL,weekly INTEGER NOT NULL,monthly INTEGER NOT NULL,next_run TEXT NOT NULL,last_success TEXT NOT NULL,last_error TEXT NOT NULL);INSERT INTO finance_snapshot_schedule VALUES(1,0,'',NULL,0,7,4,6,'','','');CREATE TABLE finance_snapshot_files(root_id TEXT NOT NULL,tier TEXT NOT NULL,period TEXT NOT NULL,name TEXT NOT NULL,sha256 TEXT NOT NULL,created TEXT NOT NULL,PRIMARY KEY(root_id,tier,period));`); e != nil {
		return e
	}
	a.h.Mux.HandleFunc("GET /api/finance/snapshots/settings", a.h.Owner(a.snapshotStatus))
	a.h.Mux.HandleFunc("POST /api/finance/snapshots/settings", a.h.Owner(a.saveSnapshotSettings))
	a.h.Mux.HandleFunc("POST /api/finance/snapshots", a.h.Owner(a.snapshotNow))
	return nil
}
func (a *App) readSnapshotSettings() (snapshotSettings, string, []byte, error) {
	v := snapshotSettings{}
	var owner string
	var key []byte
	e := a.h.DB.QueryRow(`SELECT enabled,owner_id,secret,copy_drive,daily,weekly,monthly,next_run,last_success,last_error FROM finance_snapshot_schedule WHERE id=1`).Scan(&v.Enabled, &owner, &key, &v.CopyToDrive, &v.Daily, &v.Weekly, &v.Monthly, &v.Next, &v.LastSuccess, &v.LastError)
	v.HasRecoveryKey = len(key) > 0
	return v, owner, key, e
}
func (a *App) snapshotStatus(w http.ResponseWriter, r *http.Request) {
	v, _, _, e := a.readSnapshotSettings()
	if e != nil {
		apphost.Problem(w, 500, "Snapshot settings are unavailable")
		return
	}
	apphost.JSON(w, 200, v)
}
func (a *App) saveSnapshotSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled     bool   `json:"enabled"`
		CopyToDrive bool   `json:"copyToDrive"`
		Daily       int    `json:"daily"`
		Weekly      int    `json:"weekly"`
		Monthly     int    `json:"monthly"`
		Password    string `json:"password"`
		Remember    bool   `json:"remember"`
	}
	if !apphost.Decode(w, r, &in) {
		return
	}
	if in.Daily < 1 || in.Daily > 90 || in.Weekly < 1 || in.Weekly > 52 || in.Monthly < 1 || in.Monthly > 24 {
		apphost.Problem(w, 400, "Keep 1–90 daily, 1–52 weekly and 1–24 monthly snapshots")
		return
	}
	if in.CopyToDrive && !filepath.IsAbs(os.Getenv("MYFINANCE_BACKUP_DIR")) {
		apphost.Problem(w, 400, "Configure the My Drive snapshot folder first")
		return
	}
	_, oldOwner, key, e := a.readSnapshotSettings()
	if e != nil {
		apphost.Problem(w, 500, "Snapshot settings are unavailable")
		return
	}
	owner := apphost.Current(r).User.ID
	if in.Enabled {
		if !in.Remember {
			apphost.Problem(w, 400, "Explicitly allow Windows DPAPI to protect the recovery passphrase for scheduled snapshots")
			return
		}
		if in.Password != "" {
			if len(in.Password) < 12 || len(in.Password) > 256 {
				apphost.Problem(w, 400, "Use a recovery passphrase of 12–256 bytes")
				return
			}
			key, e = protectSession([]byte(in.Password))
			in.Password = ""
			if e != nil {
				apphost.Problem(w, 500, "Windows could not protect the recovery passphrase")
				return
			}
		} else if len(key) == 0 || oldOwner != owner {
			apphost.Problem(w, 400, "Enter a recovery passphrase for this Windows/OneAccount owner")
			return
		}
	} else {
		key = nil
	}
	next := ""
	if in.Enabled {
		next = time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	}
	_, e = a.h.DB.Exec(`UPDATE finance_snapshot_schedule SET enabled=?,owner_id=?,secret=?,copy_drive=?,daily=?,weekly=?,monthly=?,next_run=?,last_error='' WHERE id=1`, in.Enabled, owner, key, in.CopyToDrive, in.Daily, in.Weekly, in.Monthly, next)
	if e != nil {
		apphost.Problem(w, 500, "Snapshot settings could not be saved")
		return
	}
	a.h.Audit(owner, "finance.snapshot-schedule-updated", "daily")
	apphost.JSON(w, 200, map[string]bool{"ok": true})
}
func (a *App) snapshotNow(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password    string `json:"password"`
		CopyToDrive bool   `json:"copyToDrive"`
	}
	if !apphost.Decode(w, r, &in) {
		return
	}
	settings, _, _, e := a.readSnapshotSettings()
	if e != nil {
		apphost.Problem(w, 500, "Snapshot settings unavailable")
		return
	}
	settings.CopyToDrive = in.CopyToDrive
	if !a.snapshotMu.TryLock() {
		apphost.Problem(w, 409, "A snapshot is already running; wait for it to finish")
		return
	}
	defer a.snapshotMu.Unlock()
	sealed, e := a.createServerSnapshot(r.Context(), apphost.Current(r).User, in.Password, settings)
	in.Password = ""
	if e != nil {
		apphost.Problem(w, 503, "Snapshot was not completed. Check server/Drive availability or wait for bank setup to finish.")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="MyFinance-server.mfs"`)
	sum := sha256.Sum256(sealed)
	w.Header().Set("X-Backup-SHA256", hex.EncodeToString(sum[:]))
	w.Write(sealed)
}
func (a *App) fetchServerSnapshot(ctx context.Context, u apphost.User) (serverSnapshot, error) {
	snap := serverSnapshot{Schema: 1, Actual: "26.10.0", Created: time.Now().UTC().Format(time.RFC3339)}
	if u.Role != "owner" {
		return snap, errors.New("owner required")
	}
	req, e := http.NewRequestWithContext(ctx, "POST", a.engine+"/finance-internal/snapshot", nil)
	if e != nil {
		return snap, e
	}
	req.Header.Set("X-Finance-Capability", a.cap)
	req.Header.Set("X-Finance-Identity", u.ID)
	req.Header.Set("X-Finance-Owner", "1")
	client := http.Client{Timeout: 45 * time.Second}
	res, e := client.Do(req)
	if e != nil {
		return snap, e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return snap, errors.New("snapshot busy or unavailable")
	}
	var response struct {
		Status string
		Data   struct{ Files []snapshotFile }
	}
	d := json.NewDecoder(io.LimitReader(res.Body, maxSnapshot*2))
	if d.Decode(&response) != nil || response.Status != "ok" {
		return snap, errors.New("invalid native snapshot")
	}
	snap.Files = response.Data.Files
	for i := range snap.Files {
		sum := sha256.Sum256(snap.Files[i].Data)
		snap.Files[i].SHA256 = hex.EncodeToString(sum[:])
	}
	return snap, validateServerSnapshot(snap)
}
func validateServerSnapshot(s serverSnapshot) error {
	if s.Schema != 1 || s.Actual != "26.10.0" || len(s.Files) == 0 || len(s.Files) > 5000 {
		return errors.New("unsupported server snapshot")
	}
	seen := map[string]bool{}
	total := 0
	for _, f := range s.Files {
		if !snapshotPath.MatchString(f.Path) || seen[f.Path] {
			return errors.New("unsafe snapshot path")
		}
		seen[f.Path] = true
		total += len(f.Data)
		if total > maxSnapshot {
			return errors.New("snapshot too large")
		}
		sum := sha256.Sum256(f.Data)
		if f.SHA256 != hex.EncodeToString(sum[:]) {
			return errors.New("snapshot checksum mismatch")
		}
		if strings.HasSuffix(f.Path, ".sqlite") && !bytes.HasPrefix(f.Data, []byte("SQLite format 3\x00")) {
			return errors.New("snapshot database format")
		}
	}
	if !seen["server-files/account.sqlite"] {
		return errors.New("missing server metadata")
	}
	return nil
}
func sealSnapshot(s serverSnapshot, password string) ([]byte, error) {
	if e := validateServerSnapshot(s); e != nil {
		return nil, e
	}
	plain, e := json.Marshal(s)
	if e != nil {
		return nil, e
	}
	defer clear(plain)
	var compressed bytes.Buffer
	z := gzip.NewWriter(&compressed)
	if _, e = z.Write(plain); e != nil {
		return nil, e
	}
	if e = z.Close(); e != nil {
		return nil, e
	}
	salt := make([]byte, 16)
	if _, e = rand.Read(salt); e != nil {
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
	header := append(append(append([]byte{}, snapshotMagic...), salt...), nonce...)
	out := a.Seal(header, nonce, compressed.Bytes(), header)
	clear(compressed.Bytes())
	return out, nil
}
func openSnapshot(b []byte, password string) (serverSnapshot, error) {
	var s serverSnapshot
	n := len(snapshotMagic)
	if len(b) < n+44 || len(b) > maxSnapshot*2 || !bytes.Equal(b[:n], snapshotMagic) {
		return s, errors.New("choose a My Finance server snapshot (.mfs)")
	}
	a, e := backupKey(password, b[n:n+16])
	if e != nil {
		return s, e
	}
	plain, e := a.Open(nil, b[n+16:n+28], b[n+28:], b[:n+28])
	if e != nil {
		return s, errors.New("wrong recovery passphrase or changed snapshot")
	}
	defer clear(plain)
	z, e := gzip.NewReader(bytes.NewReader(plain))
	if e != nil {
		return s, e
	}
	defer z.Close()
	d := json.NewDecoder(io.LimitReader(z, maxSnapshot*2))
	d.DisallowUnknownFields()
	if e = d.Decode(&s); e != nil {
		return s, e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return s, errors.New("unexpected snapshot content")
	}
	return s, validateServerSnapshot(s)
}
func (a *App) createServerSnapshot(ctx context.Context, u apphost.User, password string, settings snapshotSettings) ([]byte, error) {
	if len(password) < 12 || len(password) > 256 {
		return nil, errors.New("invalid recovery passphrase")
	}
	if settings.CopyToDrive && !filepath.IsAbs(os.Getenv("MYFINANCE_BACKUP_DIR")) {
		return nil, errors.New("Drive unavailable")
	}
	snap, e := a.fetchServerSnapshot(ctx, u)
	if e != nil {
		return nil, e
	}
	sealed, e := sealSnapshot(snap, password)
	for i := range snap.Files {
		clear(snap.Files[i].Data)
	}
	if e != nil {
		return nil, e
	}
	roots := []string{filepath.Join(a.h.Config.Data, "snapshots")}
	if settings.CopyToDrive {
		roots = append(roots, os.Getenv("MYFINANCE_BACKUP_DIR"))
	}
	now := time.Now().UTC()
	year, week := now.ISOWeek()
	tiers := []struct {
		kind, period string
		keep         int
	}{{"daily", now.Format("2006-01-02"), settings.Daily}, {"weekly", fmt.Sprintf("%04d-W%02d", year, week), settings.Weekly}, {"monthly", now.Format("2006-01"), settings.Monthly}}
	for _, root := range roots {
		for _, tier := range tiers {
			if e = a.saveSnapshotTier(root, tier.kind, tier.period, tier.keep, sealed, now); e != nil {
				return nil, e
			}
		}
	}
	_, e = a.h.DB.Exec(`UPDATE finance_snapshot_schedule SET last_success=?,last_error='' WHERE id=1`, now.Format(time.RFC3339))
	if e != nil {
		return nil, e
	}
	a.h.Audit(u.ID, "finance.server-snapshot-created", "credential-free")
	return sealed, nil
}
func (a *App) saveSnapshotTier(root, tier, period string, keep int, b []byte, now time.Time) error {
	if keep < 1 {
		return errors.New("invalid retention")
	}
	if e := safeAncestors(root); e != nil {
		return e
	}
	if e := os.MkdirAll(root, 0700); e != nil {
		return e
	}
	if e := safeAncestors(root); e != nil {
		return e
	}
	rootID := apphost.Hash(strings.ToLower(filepath.Clean(root)))
	name := "MyFinance-server-" + tier + "-" + now.Format("2006-01-02T150405Z") + "-" + apphost.ID()[:8] + ".mfs"
	p := filepath.Join(root, name)
	out, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = out.Write(b)
	if e == nil {
		e = out.Sync()
	}
	closeErr := out.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	verify, e := os.ReadFile(p)
	if e != nil || !bytes.Equal(verify, b) {
		return errors.New("snapshot write verification failed")
	}
	sum := sha256.Sum256(b)
	hash := hex.EncodeToString(sum[:])
	var oldName, oldHash string
	_ = a.h.DB.QueryRow(`SELECT name,sha256 FROM finance_snapshot_files WHERE root_id=? AND tier=? AND period=?`, rootID, tier, period).Scan(&oldName, &oldHash)
	if _, e = a.h.DB.Exec(`INSERT INTO finance_snapshot_files(root_id,tier,period,name,sha256,created)VALUES(?,?,?,?,?,?) ON CONFLICT(root_id,tier,period)DO UPDATE SET name=excluded.name,sha256=excluded.sha256,created=excluded.created`, rootID, tier, period, name, hash, now.Format(time.RFC3339)); e != nil {
		return e
	}
	if oldName != "" {
		if e = removeVerifiedSnapshot(root, oldName, oldHash); e != nil {
			return e
		}
	}
	rows, e := a.h.DB.Query(`SELECT period,name,sha256 FROM finance_snapshot_files WHERE root_id=? AND tier=? ORDER BY period DESC`, rootID, tier)
	if e != nil {
		return e
	}
	type entry struct{ period, name, hash string }
	entries := []entry{}
	for rows.Next() {
		var v entry
		if e = rows.Scan(&v.period, &v.name, &v.hash); e != nil {
			rows.Close()
			return e
		}
		entries = append(entries, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, v := range entries[minimum(keep, len(entries)):] {
		if e = removeVerifiedSnapshot(root, v.name, v.hash); e != nil {
			return e
		}
		if _, e = a.h.DB.Exec(`DELETE FROM finance_snapshot_files WHERE root_id=? AND tier=? AND period=?`, rootID, tier, v.period); e != nil {
			return e
		}
	}
	return nil
}
func removeVerifiedSnapshot(root, name, hash string) error {
	if !snapshotName.MatchString(name) {
		return errors.New("unrecognised snapshot receipt")
	}
	p := filepath.Join(root, name)
	if e := safeAncestors(p); e != nil {
		return e
	}
	b, e := os.ReadFile(p)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != hash {
		return errors.New("changed backup preserved")
	}
	return os.Remove(p)
}
func (a *App) snapshotLoop(ctx context.Context) {
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			a.scheduledSnapshot(ctx, time.Now().UTC())
			timer.Reset(time.Minute)
		}
	}
}
func (a *App) scheduledSnapshot(ctx context.Context, now time.Time) {
	settings, owner, key, e := a.readSnapshotSettings()
	if e != nil || !settings.Enabled {
		return
	}
	next, e := time.Parse(time.RFC3339, settings.Next)
	if e == nil && now.Before(next) {
		return
	}
	a.mu.Lock()
	ready := a.ready
	a.mu.Unlock()
	if !ready || !a.snapshotMu.TryLock() {
		return
	}
	defer a.snapshotMu.Unlock()
	allowed := false
	if a.h.Config.Issuer != "" {
		allowed = owner == apphost.Identity(a.h.Config.Issuer, a.h.Config.Owner)
	} else {
		var role string
		allowed = a.h.DB.QueryRow(`SELECT role FROM host_users WHERE id=?`, owner).Scan(&role) == nil && role == "owner"
	}
	message := ""
	if !allowed {
		message = "Owner changed; re-enable the snapshot schedule"
	} else {
		password, err := openSession(key)
		if err != nil {
			message = "Re-enter recovery passphrase under the current Windows account"
		} else {
			_, err = a.createServerSnapshot(ctx, apphost.User{ID: owner, Role: "owner"}, string(password), settings)
			clear(password)
			if err != nil {
				message = "Snapshot not completed; check server/Drive availability or active bank setup"
			}
		}
	}
	delay := 24 * time.Hour
	if message != "" {
		delay = 15 * time.Minute
	}
	_, _ = a.h.DB.Exec(`UPDATE finance_snapshot_schedule SET next_run=?,last_error=? WHERE id=1`, now.Add(delay).Format(time.RFC3339), message)
}

// Restore is deliberately limited to a stopped/new empty server. Existing budgets
// are never overwritten. The original empty/configured server is kept for rollback.
func RestoreServerSnapshot(h *apphost.Host, archive, password string, confirm bool) error {
	if !confirm {
		return errors.New("confirm a clean server restore with --confirm-restore; existing budgets will never be overwritten")
	}
	info, e := os.Stat(archive)
	if e != nil || info.Size() > maxSnapshot*2 {
		return errors.New("snapshot is unavailable or exceeds the supported limit")
	}
	b, e := os.ReadFile(archive)
	if e != nil {
		return e
	}
	snap, e := openSnapshot(b, password)
	if e != nil {
		return e
	}
	defer func() {
		for i := range snap.Files {
			clear(snap.Files[i].Data)
		}
	}()
	root := filepath.Join(h.Config.Data, "actual")
	if e = safeAncestors(root); e != nil {
		return e
	}
	account := filepath.Join(root, "server-files", "account.sqlite")
	if _, e = os.Stat(account); e == nil {
		db, e := sql.Open("sqlite", account)
		if e != nil {
			return e
		}
		var n int
		e = db.QueryRow(`SELECT count(*) FROM files`).Scan(&n)
		db.Close()
		if e != nil || n > 0 {
			return errors.New("this server has budget history; restore into a new private data folder instead")
		}
	}
	if entries, e := os.ReadDir(filepath.Join(root, "user-files")); e == nil && len(entries) > 0 {
		return errors.New("this server contains budget files; choose a new private data folder")
	}
	stage := filepath.Join(h.Config.Data, "restore-stage-"+apphost.ID())
	if e = os.Mkdir(stage, 0700); e != nil {
		return e
	}
	for _, f := range snap.Files {
		dest := filepath.Join(stage, filepath.FromSlash(f.Path))
		if !strings.HasPrefix(dest, stage+string(os.PathSeparator)) {
			return errors.New("restore path escaped")
		}
		if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
			return e
		}
		out, e := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		_, e = out.Write(f.Data)
		out.Close()
		if e != nil {
			return e
		}
	}
	db, e := sql.Open("sqlite", filepath.Join(stage, "server-files", "account.sqlite"))
	if e != nil {
		return e
	}
	var integrity string
	e = db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity)
	if e != nil || integrity != "ok" {
		db.Close()
		return errors.New("restored metadata failed integrity verification")
	}
	for _, table := range []string{"secrets", "auth", "sessions", "pending_openid_requests"} {
		var n int
		if e = db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil || n != 0 {
			db.Close()
			return errors.New("server snapshot contains credentials or login sessions")
		}
	}
	db.Close()
	rollback := ""
	if _, e = os.Stat(root); e == nil {
		rollback = filepath.Join(h.Config.Data, "restore-rollback-"+apphost.ID())
		if !strings.HasPrefix(rollback, h.Config.Data+string(os.PathSeparator)) {
			return errors.New("invalid rollback path")
		}
		if e = os.Rename(root, rollback); e != nil {
			return e
		}
	}
	if e = os.Rename(stage, root); e != nil {
		if rollback != "" {
			_ = os.Rename(rollback, root)
		}
		return e
	}
	return nil
}

// Stable ordering is useful to independent restore-verification tooling.
func snapshotPaths(s serverSnapshot) []string {
	v := []string{}
	for _, f := range s.Files {
		v = append(v, f.Path)
	}
	sort.Strings(v)
	return v
}
