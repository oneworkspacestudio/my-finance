//go:build windows

package finance

import (
	"bytes"
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

const paperTestRoot = "C:/Projects/.cache/finance-investments-20261105"

func paperHostFixture(t *testing.T) *App {
	t.Helper()
	if e := os.MkdirAll(paperTestRoot, 0700); e != nil {
		t.Fatal(e)
	}
	data, e := os.MkdirTemp(paperTestRoot, "paper-go-")
	if e != nil {
		t.Fatal(e)
	}
	h, e := apphost.New(apphost.Config{ID: "finance", Name: "Fictional paper fixture", Version: "fixture", Data: data, Listen: "127.0.0.1:25211", Origin: "http://127.0.0.1:25211/finance"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		h.Close()
		absolute, _ := filepath.Abs(data)
		root, _ := filepath.Abs(paperTestRoot)
		if strings.HasPrefix(strings.ToLower(absolute), strings.ToLower(root+string(filepath.Separator))) {
			_ = os.RemoveAll(absolute)
		}
	})
	a := &App{h: h}
	for _, routes := range []func() error{a.connectionRoutes, a.snapshotRoutes, a.aiRoutes, a.investmentRoutes, a.brokerRoutes} {
		if e = routes(); e != nil {
			t.Fatal(e)
		}
	}
	for _, user := range []string{"fixture-owner", "fixture-member"} {
		_, e = h.DB.Exec(`INSERT INTO host_users(id,issuer,subject,name,role)VALUES(?,'local:finance',?,'Fictional investor','member')`, user, user)
		if e != nil {
			t.Fatal(e)
		}
		_, e = h.DB.Exec(`INSERT INTO host_sessions(id,user_id,csrf,expires)VALUES(?,?,?,?)`, apphost.Hash(aiFixtureToken(user)), user, "fictional-csrf", time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano))
		if e != nil {
			t.Fatal(e)
		}
	}
	return a
}
func paperHTTP(a *App, user, method, path string, body any, alter func(*http.Request)) *httptest.ResponseRecorder {
	var raw []byte
	if s, ok := body.(string); ok {
		raw = []byte(s)
	} else if body != nil {
		raw, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, "http://127.0.0.1:25211/finance/api/finance/investments/paper"+path, bytes.NewReader(raw))
	if user != "" {
		r.AddCookie(&http.Cookie{Name: "finance_session", Value: aiFixtureToken(user)})
	}
	r.Header.Set("Origin", "http://127.0.0.1:25211")
	r.Header.Set("X-CSRF-Token", "fictional-csrf")
	if alter != nil {
		alter(r)
	}
	w := httptest.NewRecorder()
	a.h.Handler().ServeHTTP(w, r)
	return w
}
func paperHTTPState(t *testing.T, w *httptest.ResponseRecorder) paperState {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("paper request failed %d: %s", w.Code, w.Body.String())
	}
	var response struct {
		Simulation paperState `json:"simulation"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &response); e != nil {
		t.Fatal(e)
	}
	return response.Simulation
}
func paperHTTPSetup(t *testing.T, a *App, user string) paperState {
	return paperHTTPState(t, paperHTTP(a, user, "POST", "/setup", paperSetupInput{Confirm: true, Config: paperFixtureConfig(), Prices: paperFixtureState().Prices}, nil))
}
func paperStoreFakeKey(t *testing.T, a *App, user string) {
	t.Helper()
	plain, _ := json.Marshal(aiKeyEnvelope{User: user, Provider: "openai", Key: "fictional-paper-key"})
	secret, e := protectSession(plain)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.h.DB.Exec(`INSERT INTO finance_ai_keys VALUES(?,'openai','gpt-4.1-mini',?)`, user, secret); e != nil {
		t.Fatal(e)
	}
}
func paperHTTPRun(t *testing.T, a *App, user string, s paperState, n int) paperState {
	return paperHTTPState(t, paperHTTP(a, user, "POST", "/run", paperRunInput{Revision: s.Revision, Provider: "openai", Decisions: n, Consent: true}, nil))
}
func TestPaperHTTPProtectionIsolationAndRevision(t *testing.T) {
	a := paperHostFixture(t)
	if w := paperHTTP(a, "", "GET", "", nil, nil); w.Code != 401 {
		t.Fatal("anonymous paper read", w.Code)
	}
	for _, alter := range []func(*http.Request){func(r *http.Request) { r.Header.Del("X-CSRF-Token") }, func(r *http.Request) { r.Header.Set("Origin", "https://evil.invalid") }} {
		w := paperHTTP(a, "fixture-owner", "POST", "/setup", paperSetupInput{Confirm: true, Config: paperFixtureConfig(), Prices: paperFixtureState().Prices}, alter)
		if w.Code != 403 {
			t.Fatal("CSRF/origin bypass", w.Code)
		}
	}
	s := paperHTTPSetup(t, a, "fixture-owner")
	w := paperHTTP(a, "fixture-owner", "GET", "", nil, nil)
	if w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "sessionHash") || strings.Contains(w.Body.String(), "fixture-owner") {
		t.Fatal("private metadata/caching boundary")
	}
	w = paperHTTP(a, "fixture-member", "GET", "", nil, nil)
	if !strings.Contains(w.Body.String(), `"configured":false`) {
		t.Fatal("cross-account portfolio exposed", w.Body.String())
	}
	if w = paperHTTP(a, "fixture-owner", "GET", "", nil, func(r *http.Request) { r.Header.Set("X-Workspace-Expected-Account", "fixture-member") }); w.Code != 409 {
		t.Fatal("split identity accepted")
	}
	var cipher []byte
	if e := a.h.DB.QueryRow(`SELECT secret FROM finance_paper_investments WHERE user_id='fixture-owner'`).Scan(&cipher); e != nil || bytes.Contains(cipher, []byte("SIM-EQUITY")) {
		t.Fatal("portfolio plaintext stored", e)
	}
	if _, e := a.h.DB.Exec(`INSERT INTO finance_paper_investments VALUES('fixture-member',?,?)`, s.Revision, cipher); e != nil {
		t.Fatal(e)
	}
	if w = paperHTTP(a, "fixture-member", "GET", "", nil, nil); w.Code != 422 {
		t.Fatal("ciphertext crossed account binding", w.Code)
	}
	if w = paperHTTP(a, "fixture-owner", "POST", "/prices", paperPricesInput{Revision: s.Revision - 1, Prices: s.Prices}, nil); w.Code != 412 {
		t.Fatal("stale mutation accepted")
	}
	if w = paperHTTP(a, "fixture-owner", "POST", "/setup", `{"revision":1,"confirm":true,"config":{},"prices":[],"endpoint":"https://evil.invalid"}`, nil); w.Code != 400 {
		t.Fatal("unknown JSON field accepted")
	}
}
func TestPaperAttemptPersistedBeforeProviderAndPauseDiscardsResult(t *testing.T) {
	a := paperHostFixture(t)
	user := "fixture-owner"
	s := paperHTTPSetup(t, a, user)
	paperStoreFakeKey(t, a, user)
	s = paperHTTPRun(t, a, user, s, 2)
	started := make(chan struct{})
	finished := make(chan *httptest.ResponseRecorder, 1)
	a.ai.client.Transport = aiRoundTrip(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	go func() { finished <- paperHTTP(a, user, "POST", "/step", paperStepInput{s.Revision, s.Run.ID}, nil) }()
	<-started
	v, e := a.paperLoad(user)
	if e != nil || v.State.Attempts != 1 || v.State.Status != "thinking" {
		t.Fatal("attempt not persisted before HTTP", e, v.State)
	}
	if w := paperHTTP(a, user, "POST", "/step", paperStepInput{s.Revision, s.Run.ID}, nil); w.Code != 412 {
		t.Fatal("replayed step accepted")
	}
	paused := paperHTTPState(t, paperHTTP(a, user, "POST", "/pause", paperStepInput{s.Revision, s.Run.ID}, nil))
	if paused.Status != "paused" || paused.Attempts != 1 {
		t.Fatal("stale-revision pause failed", paused)
	}
	select {
	case w := <-finished:
		if w.Code != 200 {
			t.Fatal("paused stale result did not reconcile", w.Code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("pause failed to cancel in-flight provider")
	}
	v, e = a.paperLoad(user)
	if e != nil || v.State.Cash != v.State.Config.Initial || v.State.Run != nil || v.State.Attempts != 1 {
		t.Fatal("cancelled result committed", e, v.State)
	}
	a.ai.mu.Lock()
	busy := a.ai.usage[user].Busy
	a.ai.mu.Unlock()
	if busy {
		t.Fatal("AI quota left busy after completion")
	}
}
func TestPaperValidDecisionFailureCountAndSessionRevocation(t *testing.T) {
	a := paperHostFixture(t)
	user := "fixture-owner"
	s := paperHTTPSetup(t, a, user)
	paperStoreFakeKey(t, a, user)
	s = paperHTTPRun(t, a, user, s, 1)
	calls := 0
	a.ai.client.Transport = aiRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Authorization") != "Bearer fictional-paper-key" {
			t.Error("wrong user's provider key")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(paperProviderFixture("openai", `{"actions":[{"side":"buy","instrument":"SIM-EQUITY","units":10}],"reason":"Fictional bounded decision."}`)))}, nil
	})
	s = paperHTTPState(t, paperHTTP(a, user, "POST", "/step", paperStepInput{s.Revision, s.Run.ID}, nil))
	if calls != 1 || s.Attempts != 1 || s.Cash != 98770 || s.Status != "paused" {
		t.Fatal("valid finite decision failed", s, calls)
	}
	// No automatic retry or paid step without a new consented run.
	if w := paperHTTP(a, user, "POST", "/step", paperStepInput{s.Revision, "old"}, nil); w.Code != 400 || calls != 1 {
		t.Fatal("unapproved repeated call")
	}
	a.ai.mu.Lock()
	delete(a.ai.usage, user)
	a.ai.mu.Unlock()
	s = paperHTTPRun(t, a, user, s, 1)
	a.ai.client.Transport = aiRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader("PRIVATE provider diagnostic"))}, nil
	})
	w := paperHTTP(a, user, "POST", "/step", paperStepInput{s.Revision, s.Run.ID}, nil)
	if w.Code != 502 || strings.Contains(w.Body.String(), "PRIVATE") || calls != 2 {
		t.Fatal("provider failure leaked or retried", w.Code, calls)
	}
	v, _ := a.paperLoad(user)
	if v.State.Attempts != 2 || v.State.Status != "paused" || v.State.Cash != 98770 {
		t.Fatal("failed attempt not retained atomically", v.State)
	}
	a.ai.mu.Lock()
	delete(a.ai.usage, user)
	a.ai.mu.Unlock()
	s = paperHTTPRun(t, a, user, v.State, 1)
	a.ai.client.Transport = aiRoundTrip(func(*http.Request) (*http.Response, error) {
		_, e := a.h.DB.Exec(`DELETE FROM host_sessions WHERE user_id=?`, user)
		if e != nil {
			t.Error(e)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(paperProviderFixture("openai", `{"actions":[{"side":"buy","instrument":"SIM-EQUITY","units":1}],"reason":"Should be discarded."}`)))}, nil
	})
	w = paperHTTP(a, user, "POST", "/step", paperStepInput{s.Revision, s.Run.ID}, nil)
	if w.Code != 502 {
		t.Fatal("revoked session result accepted", w.Code)
	}
	v, _ = a.paperLoad(user)
	if v.State.Attempts != 3 || v.State.Cash != 98770 || v.State.Run != nil {
		t.Fatal("logout did not discard result", v.State)
	}
}
func TestPaperRestartPausesInterruptedAttemptAndPreservesExistingState(t *testing.T) {
	a := paperHostFixture(t)
	user := "fixture-owner"
	s := paperHTTPSetup(t, a, user)
	paperStoreFakeKey(t, a, user)
	s = paperHTTPRun(t, a, user, s, 2)
	v, e := a.paperLoad(user)
	if e != nil {
		t.Fatal(e)
	}
	v.State.Attempts = 1
	v.State.Status = "thinking"
	v.State.Run.Remaining--
	v.State.Revision++
	if e = a.paperSave(v); e != nil {
		t.Fatal(e)
	}
	var keyBefore []byte
	if e = a.h.DB.QueryRow(`SELECT secret FROM finance_ai_keys WHERE user_id=?`, user).Scan(&keyBefore); e != nil {
		t.Fatal(e)
	}
	config := a.h.Config
	a.h.Close()
	h, e := apphost.New(config, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer h.Close()
	restarted := &App{h: h}
	for _, routes := range []func() error{restarted.aiRoutes, restarted.investmentRoutes, restarted.brokerRoutes} {
		if e = routes(); e != nil {
			t.Fatal(e)
		}
	}
	v, e = restarted.paperLoad(user)
	if e != nil || v.State.Status != "paused" || v.State.Attempts != 1 || v.State.Run != nil || v.State.Revision != s.Revision+2 {
		t.Fatal("restart resumed/forgot attempt", e, v.State)
	}
	var keyAfter []byte
	if e = h.DB.QueryRow(`SELECT secret FROM finance_ai_keys WHERE user_id=?`, user).Scan(&keyAfter); e != nil || !bytes.Equal(keyBefore, keyAfter) {
		t.Fatal("restart changed saved key", e)
	}
	var schema int
	if e = h.DB.QueryRow(`SELECT max(version) FROM host_migrations`).Scan(&schema); e != nil || schema != 6 {
		t.Fatal("schema6 not installed", schema, e)
	}
}
