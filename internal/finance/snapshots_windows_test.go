//go:build windows

package finance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func snapshotFixture() serverSnapshot {
	b := []byte("SQLite format 3\x00strictly fictional metadata")
	sum := sha256.Sum256(b)
	return serverSnapshot{Schema: 1, Actual: "26.10.0", Created: "2026-10-28T12:00:00Z", Files: []snapshotFile{{Path: "server-files/account.sqlite", Data: b, SHA256: hex.EncodeToString(sum[:])}}}
}
func TestServerSnapshotEnvelopeAndPathGuards(t *testing.T) {
	s := snapshotFixture()
	sealed, e := sealSnapshot(s, "synthetic snapshot passphrase")
	if e != nil {
		t.Fatal(e)
	}
	restored, e := openSnapshot(sealed, "synthetic snapshot passphrase")
	if e != nil || !bytes.Equal(restored.Files[0].Data, s.Files[0].Data) {
		t.Fatal("snapshot roundtrip", e)
	}
	if bytes.Contains(sealed, []byte("fictional")) {
		t.Fatal("plaintext snapshot")
	}
	if _, e = openSnapshot(sealed, "other synthetic recovery phrase"); e == nil {
		t.Fatal("wrong password")
	}
	sealed[len(sealed)-1] ^= 1
	if _, e = openSnapshot(sealed, "synthetic snapshot passphrase"); e == nil {
		t.Fatal("changed snapshot")
	}
	s.Files[0].Path = "../secrets.dpapi"
	if validateServerSnapshot(s) == nil {
		t.Fatal("snapshot traversal")
	}
	s = snapshotFixture()
	s.Files[0].SHA256 = "wrong"
	if validateServerSnapshot(s) == nil {
		t.Fatal("unchecked snapshot bytes")
	}
}
func TestTieredSnapshotRetentionAndChangedFilesPreserved(t *testing.T) {
	h, e := apphost.New(apphost.Config{ID: "finance", Name: "Snapshot fixture", Version: "fixture", Data: t.TempDir(), Listen: "127.0.0.1:25178", Origin: "http://127.0.0.1:25178/finance"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer h.Close()
	a := &App{h: h}
	if e = a.connectionRoutes(); e != nil {
		t.Fatal(e)
	}
	if e = a.snapshotRoutes(); e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(h.Config.Data, "snapshots")
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		d := now.AddDate(0, 0, i)
		if e = a.saveSnapshotTier(root, "daily", d.Format("2006-01-02"), 2, []byte("fictional sealed snapshot bytes"), d); e != nil {
			t.Fatal(e)
		}
	}
	entries, e := os.ReadDir(root)
	if e != nil || len(entries) != 2 {
		t.Fatal("retention", len(entries), e)
	}
	var name string
	if e = h.DB.QueryRow(`SELECT name FROM finance_snapshot_files ORDER BY period LIMIT 1`).Scan(&name); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, name), []byte("owner modified file"), 0600); e != nil {
		t.Fatal(e)
	}
	d := now.AddDate(0, 0, 8)
	if e = a.saveSnapshotTier(root, "daily", d.Format("2006-01-02"), 1, []byte("new fictional sealed snapshot"), d); e == nil {
		t.Fatal("changed old file silently removed")
	}
	if b, e := os.ReadFile(filepath.Join(root, name)); e != nil || string(b) != "owner modified file" {
		t.Fatal("changed file was not retained")
	}
}
