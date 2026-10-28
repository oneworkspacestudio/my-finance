//go:build windows

package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderReferenceDPAPIAndMetadataIdempotency(t *testing.T) {
	cleartext := []byte("strictly-fictional-provider-session")
	sealed, e := protectSession(cleartext)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(sealed, cleartext) {
		t.Fatal("plaintext reference stored")
	}
	restored, e := openSession(sealed)
	if e != nil || !bytes.Equal(restored, cleartext) {
		t.Fatal("DPAPI roundtrip", e)
	}
	sealed[len(sealed)-1] ^= 1
	if _, e = openSession(sealed); e == nil {
		t.Fatal("tampered DPAPI reference accepted")
	}
	h, e := apphost.New(apphost.Config{ID: "finance", Name: "My Finance fixture", Version: "fixture", Data: t.TempDir(), Listen: "127.0.0.1:25177", Origin: "http://127.0.0.1:25177/finance"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer h.Close()
	a := &App{h: h}
	if e = a.connectionRoutes(); e != nil {
		t.Fatal(e)
	}
	body := []byte(`{"status":"ok","data":{"session_id":"fictional-reference","valid_until":"2026-11-01T12:00:00Z","aspsp":{"name":"Fictional Test Bank"},"accounts":[{}]}}`)
	s := apphost.Session{User: apphost.User{ID: strings.Repeat("a", 64)}}
	if e = a.captureConnection(body, s); e != nil {
		t.Fatal(e)
	}
	if e = a.captureConnection(body, s); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = h.DB.QueryRow("SELECT count(*) FROM finance_connections").Scan(&count); e != nil || count != 1 {
		t.Fatal("duplicate callback metadata", count, e)
	}
	var protected []byte
	if e = h.DB.QueryRow("SELECT secret FROM finance_connections").Scan(&protected); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(protected, []byte("fictional-reference")) {
		t.Fatal("provider reference was not protected")
	}
}
func TestProviderControlUsesFixedEngineAndVerifiedOwner(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Finance-Capability") != "test-capability" || r.Header.Get("X-Finance-Identity") != "fixture-owner" || r.URL.Path != "/enablebanking/finance-session-status" {
			t.Error("invalid trusted engine request")
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": map[string]string{"providerStatus": "AUTHORIZED", "validUntil": "2026-11-01T12:00:00Z"}})
	}))
	defer server.Close()
	a := &App{engine: server.URL, cap: "test-capability"}
	u := apphost.User{ID: "fixture-owner", Role: "owner"}
	if _, e := a.engineJSON(context.Background(), u, "/enablebanking/finance-session-status", map[string]string{"sessionId": "fictional"}); e != nil {
		t.Fatal(e)
	}
	u.Role = "member"
	if _, e := a.engineJSON(context.Background(), u, "/enablebanking/finance-session-status", nil); e == nil {
		t.Fatal("member used provider controls")
	}
}
