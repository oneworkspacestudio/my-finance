package finance

import (
	"archive/zip"
	"bytes"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"strings"
	"testing"
	"time"
)

func exportFixture() []byte {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	f, _ := z.Create("db.sqlite")
	f.Write([]byte("SQLite format 3\x00strictly fictional test ledger"))
	f, _ = z.Create("metadata.json")
	f.Write([]byte(`{"budgetName":"Fictional household"}`))
	z.Close()
	return b.Bytes()
}
func TestBackupAuthenticatedRoundtripAndRejection(t *testing.T) {
	plain := exportFixture()
	sealed, e := encryptExport(plain, "synthetic recovery phrase")
	if e != nil {
		t.Fatal(e)
	}
	restored, e := decryptExport(sealed, "synthetic recovery phrase")
	if e != nil || !bytes.Equal(restored, plain) {
		t.Fatal("roundtrip failed", e)
	}
	if bytes.Contains(sealed, []byte("Fictional")) {
		t.Fatal("plaintext exposed")
	}
	if _, e = decryptExport(sealed, "different recovery phrase"); e == nil {
		t.Fatal("wrong passphrase accepted")
	}
	sealed[len(sealed)-1] ^= 1
	if _, e = decryptExport(sealed, "synthetic recovery phrase"); e == nil {
		t.Fatal("tampering accepted")
	}
	if _, e = decryptExport(sealed[:len(sealed)-10], "synthetic recovery phrase"); e == nil {
		t.Fatal("truncation accepted")
	}
}
func TestBackupRejectsServerSecrets(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	f, _ := z.Create("account.sqlite")
	f.Write([]byte("secret"))
	z.Close()
	if validateExport(b.Bytes()) == nil {
		t.Fatal("server database accepted")
	}
	if _, e := encryptExport(exportFixture(), "short"); e == nil {
		t.Fatal("weak passphrase accepted")
	}
}
func TestConsentBoundToUserSessionExpiryAndSingleUse(t *testing.T) {
	a := &App{states: map[string]consent{}}
	owner := apphost.Session{User: apphost.User{ID: "synthetic-owner"}, CSRF: "session-a"}
	a.addState("nonce", owner)
	foreign := owner
	foreign.User.ID = "other"
	if a.checkState("nonce", foreign, true) {
		t.Fatal("foreign user accepted")
	}
	foreign = owner
	foreign.CSRF = "other-session"
	if a.checkState("nonce", foreign, true) {
		t.Fatal("foreign session accepted")
	}
	if !a.checkState("nonce", owner, true) {
		t.Fatal("valid consent denied")
	}
	if a.checkState("nonce", owner, true) {
		t.Fatal("replay accepted")
	}
	a.states["expired"] = consent{owner.User.ID, owner.CSRF, time.Now().Add(-time.Second), false}
	if a.checkState("expired", owner, true) {
		t.Fatal("expired consent accepted")
	}
	if a.checkState("unknown", owner, true) {
		t.Fatal("unknown state accepted")
	}
}
func TestSubpathRebasePreservesNativeRoutes(t *testing.T) {
	s := rebase("let serverURL=window.location.origin,result;BrowserRouter,{children:x};`/static/worker.js`;${window.location.origin}/enablebanking/auth_callback", "/finance/ledger")
	for _, p := range []string{`window.location.origin+"/finance/ledger"`, `basename:"/finance/ledger"`, `/finance/ledger/static/worker.js`, `/finance/ledger/enablebanking/auth_callback`} {
		if !strings.Contains(s, p) {
			t.Fatalf("missing %s", p)
		}
	}
}
