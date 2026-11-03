//go:build windows

package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
)

func aiHostFixture(t *testing.T) *App {
	t.Helper()
	h, e := apphost.New(apphost.Config{ID: "finance", Name: "Fictional AI fixture", Version: "fixture", Data: t.TempDir(), Listen: "127.0.0.1:25201", Origin: "http://127.0.0.1:25201/finance"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { h.Close() })
	a := &App{h: h}
	if e = a.connectionRoutes(); e != nil {
		t.Fatal(e)
	}
	if e = a.snapshotRoutes(); e != nil {
		t.Fatal(e)
	}
	if e = a.aiRoutes(); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"fixture-owner", "fixture-member"} {
		role := "member"
		if id == "fixture-owner" {
			role = "owner"
		}
		_, e = h.DB.Exec(`INSERT INTO host_users(id,issuer,subject,name,role)VALUES(?,'local:finance',?,'Fictional user',?)`, id, id, role)
		if e != nil {
			t.Fatal(e)
		}
		token := aiFixtureToken(id)
		_, e = h.DB.Exec(`INSERT INTO host_sessions(id,user_id,csrf,expires)VALUES(?,?,?,?)`, apphost.Hash(token), id, "fictional-csrf", time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano))
		if e != nil {
			t.Fatal(e)
		}
	}
	return a
}
func aiFixtureToken(user string) string { return user + strings.Repeat("x", 96-len(user)) }
func aiFixtureRequest(a *App, user, method, path, body string, alter func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:25201/finance/api/finance/ai/"+path, strings.NewReader(body))
	if user != "" {
		r.AddCookie(&http.Cookie{Name: "finance_session", Value: aiFixtureToken(user)})
	}
	r.Header.Set("Origin", "http://127.0.0.1:25201")
	r.Header.Set("X-CSRF-Token", "fictional-csrf")
	if alter != nil {
		alter(r)
	}
	w := httptest.NewRecorder()
	a.h.Handler().ServeHTTP(w, r)
	return w
}
func TestAISettingsAuthenticationCSRFKeyProtectionAndUserIsolation(t *testing.T) {
	a := aiHostFixture(t)
	input := `{"provider":"openai","model":"gpt-4.1-mini","apiKey":"fictional-owner-key","remove":false}`
	if w := aiFixtureRequest(a, "", "POST", "settings", input, nil); w.Code != 401 {
		t.Fatal("anonymous key access", w.Code)
	}
	for name, alter := range map[string]func(*http.Request){"csrf": func(r *http.Request) { r.Header.Del("X-CSRF-Token") }, "origin": func(r *http.Request) { r.Header.Set("Origin", "https://evil.invalid") }, "identity": func(r *http.Request) { r.Header.Set("X-Workspace-Expected-Account", "other-user") }} {
		t.Run(name, func(t *testing.T) {
			w := aiFixtureRequest(a, "fixture-owner", "POST", "settings", input, alter)
			if w.Code != 403 && w.Code != 409 {
				t.Fatal("guard accepted", w.Code)
			}
		})
	}
	w := aiFixtureRequest(a, "fixture-owner", "POST", "settings", input, nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "fictional-owner-key") {
		t.Fatal("key save/response", w.Code, w.Body.String())
	}
	var cipher []byte
	if e := a.h.DB.QueryRow(`SELECT secret FROM finance_ai_keys WHERE user_id='fixture-owner'`).Scan(&cipher); e != nil || bytes.Contains(cipher, []byte("fictional-owner-key")) {
		t.Fatal("key was not encrypted", e)
	}
	model, key, e := a.aiCredential("fixture-owner", "openai")
	if e != nil || model != "gpt-4.1-mini" || key != "fictional-owner-key" {
		t.Fatal("protected key unavailable", e)
	}
	w = aiFixtureRequest(a, "fixture-member", "GET", "settings", "", nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), `"configured":true`) {
		t.Fatal("owner key visible to member", w.Body.String())
	}
	if _, _, e = a.aiCredential("fixture-member", "openai"); e == nil {
		t.Fatal("member used owner key")
	}
	w = aiFixtureRequest(a, "fixture-member", "POST", "settings", `{"provider":"openai","model":"gpt-4.1-mini","apiKey":"fictional-member-key"}`, nil)
	if w.Code != 200 {
		t.Fatal("member cannot own an independent key", w.Body.String())
	}
	if _, e = a.h.DB.Exec(`UPDATE finance_ai_keys SET secret=? WHERE user_id='fixture-member'`, cipher); e != nil {
		t.Fatal(e)
	}
	if _, _, e = a.aiCredential("fixture-member", "openai"); e == nil {
		t.Fatal("ciphertext substitution crossed identity")
	}
	w = aiFixtureRequest(a, "fixture-member", "POST", "settings", `{"provider":"openai","remove":true}`, nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if _, _, e = a.aiCredential("fixture-owner", "openai"); e != nil {
		t.Fatal("member deletion removed owner key")
	}
	w = aiFixtureRequest(a, "fixture-owner", "POST", "settings", `{"provider":"openai","model":"gpt-4.1-mini","apiKey":""}`, nil)
	if w.Code != 200 {
		t.Fatal("retained key update failed")
	}
	log, e := os.ReadFile(filepath.Join(a.h.Config.Data, "app.log"))
	if e != nil || bytes.Contains(log, []byte("fictional-owner-key")) {
		t.Fatal("key leaked to logs", e)
	}
}
func TestAIHTTPRejectsUnknownFieldsTrailingJSONAndMissingConsent(t *testing.T) {
	a := aiHostFixture(t)
	for _, body := range []string{`{"provider":"openai","apiKey":"fictional-test-key","url":"https://evil.invalid"}`, `{"provider":"openai","apiKey":"fictional-test-key"} {}`, `{"provider":"openai","apiKey":"fictional-test-key","user_id":"fixture-owner"}`, `{"provider":"https://evil.invalid","apiKey":"fictional-test-key"}`, `{"provider":"openai","apiKey":"fictional-test-key\r\nsecret"}`} {
		w := aiFixtureRequest(a, "fixture-member", "POST", "settings", body, nil)
		if w.Code != 400 {
			t.Fatal("invalid key request accepted", w.Code)
		}
	}
	w := aiFixtureRequest(a, "fixture-owner", "POST", "analyse", `{"provider":"openai","consent":false}`, nil)
	if w.Code != 400 {
		t.Fatal("missing consent accepted", w.Code)
	}
	w = aiFixtureRequest(a, "fixture-owner", "POST", "plan", `{"summary":{"currency":"GBP","transactions":[]}}`, nil)
	if w.Code != 400 {
		t.Fatal("raw transaction field accepted")
	}
	in := aiFixtureInput()
	now := time.Now().UTC()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -3, 0)
	for i := range in.Summary.Months {
		in.Summary.Months[i].Month = first.AddDate(0, i, 0).Format("2006-01")
	}
	raw, _ := json.Marshal(in)
	w = aiFixtureRequest(a, "fixture-member", "POST", "plan", string(raw), nil)
	if w.Code != 200 {
		t.Fatal("valid draft failed", w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("financial response cached")
	}
}

func TestAIAnalysisUsesOnlyConsentedUserKeyAndLeavesNoHistory(t *testing.T) {
	a := aiHostFixture(t)
	for _, provider := range []string{"openai", "deepseek"} {
		user := "fixture-owner"
		if provider == "deepseek" {
			user = "fixture-member"
		}
		key := "fictional-" + provider + "-key"
		settings, _ := json.Marshal(aiSettingsInput{Provider: provider, Key: key})
		if w := aiFixtureRequest(a, user, "POST", "settings", string(settings), nil); w.Code != 200 {
			t.Fatal("key setup failed", w.Body.String())
		}
		calls := 0
		a.ai.client = &http.Client{Transport: aiRoundTrip(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.Header.Get("Authorization") != "Bearer "+key {
				t.Error("another account's key used")
			}
			body, _ := io.ReadAll(r.Body)
			if bytes.Contains(body, []byte("fixture-owner")) || bytes.Contains(body, []byte("fixture-member")) || bytes.Contains(body, []byte(key)) {
				t.Error("private identity or key entered provider body")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(aiProviderBody(provider, aiGoodAdvice())))}, nil
		})}
		fixture := aiFixtureInput()
		now := time.Now().UTC()
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -3, 0)
		for i := range fixture.Summary.Months {
			fixture.Summary.Months[i].Month = first.AddDate(0, i, 0).Format("2006-01")
		}
		input := aiAnalyseInput{Summary: fixture.Summary, Profile: fixture.Profile, Provider: provider, Consent: false, Question: "First question\nsecond question\tthird part"}
		raw, _ := json.Marshal(input)
		if w := aiFixtureRequest(a, user, "POST", "analyse", string(raw), nil); w.Code != 400 || calls != 0 {
			t.Fatal("unconsented data sent", w.Code, calls)
		}
		input.Consent = true
		raw, _ = json.Marshal(input)
		w := aiFixtureRequest(a, user, "POST", "analyse", string(raw), nil)
		if w.Code != 200 || calls != 1 || strings.Contains(w.Body.String(), key) || !strings.Contains(w.Body.String(), `"advice"`) {
			t.Fatal("analysis failed or leaked key", w.Code, calls, w.Body.String())
		}
		if w = aiFixtureRequest(a, user, "POST", "analyse", string(raw), nil); w.Code != 429 || calls != 1 {
			t.Fatal("request limit bypassed", w.Code, calls)
		}
	}
	var count int
	if e := a.h.DB.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name LIKE 'finance_ai_%'`).Scan(&count); e != nil || count != 1 {
		t.Fatal("unexpected AI history persistence", e, count)
	}
	if _, e := a.h.DB.Exec(`UPDATE host_sessions SET expires='2000-01-01T00:00:00Z' WHERE user_id='fixture-member'`); e != nil {
		t.Fatal(e)
	}
	if w := aiFixtureRequest(a, "fixture-member", "GET", "settings", "", nil); w.Code != 401 {
		t.Fatal("expired session used credentials", w.Code)
	}
	log, e := os.ReadFile(filepath.Join(a.h.Config.Data, "app.log"))
	if e != nil || bytes.Contains(log, []byte("fictional-openai-key")) || bytes.Contains(log, []byte("Fictional holiday")) || bytes.Contains(log, []byte("First question")) {
		t.Fatal("AI data logged", e)
	}
}

func TestAIProductionClientRefusesProviderRedirect(t *testing.T) {
	a := aiHostFixture(t)
	calls := 0
	a.ai.client.Transport = aiRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://untrusted.invalid/collect"}}, Body: io.NopCloser(strings.NewReader("provider diagnostic should stay private")), Request: r}, nil
	})
	if _, e := a.aiCall(context.Background(), "openai", "gpt-4.1-mini", "fictional-key", aiAnalyseInput{}, aiPlan{}); e == nil || calls != 1 {
		t.Fatal("redirect followed or accepted", e, calls)
	}
}
