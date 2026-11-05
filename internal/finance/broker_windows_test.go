//go:build windows

package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const brokerFixtureSettings = `{"environment":"demo","apiKey":"fictional-broker-key","apiSecret":"fictional-broker-secret","readOnlyConsent":true}`

func brokerHostFixture(t *testing.T) *App {
	t.Helper()
	a := aiHostFixture(t)
	if err := a.investmentRoutes(); err != nil {
		t.Fatal(err)
	}
	if err := a.brokerRoutes(); err != nil {
		t.Fatal(err)
	}
	return a
}
func brokerFixtureRequest(a *App, user, method, path, body string, alter func(*http.Request)) *httptest.ResponseRecorder {
	return aiFixtureRequest(a, user, method, "unused", body, func(r *http.Request) {
		r.URL.Path = "/finance/api/finance/investments/broker/" + path
		if alter != nil {
			alter(r)
		}
	})
}
func brokerSaveFixture(t *testing.T, a *App, user string) {
	t.Helper()
	if w := brokerFixtureRequest(a, user, "POST", "settings", brokerFixtureSettings, nil); w.Code != 200 {
		t.Fatal("fixture key setup", w.Code, w.Body.String())
	}
}

func TestBrokerSettingsAuthenticationProtectionAndAccountIsolation(t *testing.T) {
	a := brokerHostFixture(t)
	for _, method := range []string{"GET", "POST"} {
		if w := brokerFixtureRequest(a, "", method, "settings", brokerFixtureSettings, nil); w.Code != 401 {
			t.Fatal("anonymous access", w.Code)
		}
	}
	for name, alter := range map[string]func(*http.Request){
		"csrf":    func(r *http.Request) { r.Header.Del("X-CSRF-Token") },
		"origin":  func(r *http.Request) { r.Header.Set("Origin", "https://evil.invalid") },
		"account": func(r *http.Request) { r.Header.Set("X-Workspace-Expected-Account", "other-user") },
	} {
		t.Run(name, func(t *testing.T) {
			for _, path := range []string{"settings", "refresh"} {
				if w := brokerFixtureRequest(a, "fixture-owner", "POST", path, brokerFixtureSettings, alter); w.Code != 403 && w.Code != 409 {
					t.Fatal("request guard bypassed", path, w.Code)
				}
			}
		})
	}
	w := brokerFixtureRequest(a, "fixture-owner", "POST", "settings", brokerFixtureSettings, nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "fictional-broker") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("key response", w.Code, w.Body.String())
	}
	var cipher []byte
	if err := a.h.DB.QueryRow(`SELECT secret FROM finance_broker_keys WHERE user_id='fixture-owner'`).Scan(&cipher); err != nil || bytes.Contains(cipher, []byte("fictional-broker")) {
		t.Fatal("credential encryption", err)
	}
	key, _, err := a.loadBrokerKey("fixture-owner")
	if err != nil || key.Key != "fictional-broker-key" || key.Secret != "fictional-broker-secret" || key.Environment != "demo" {
		t.Fatal("protected credential round trip", err)
	}
	if _, _, err := a.loadBrokerKey("fixture-member"); err == nil {
		t.Fatal("member accessed owner credentials")
	}
	w = brokerFixtureRequest(a, "fixture-member", "GET", "settings", "", nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), `"configured":true`) {
		t.Fatal("owner connection disclosed to member")
	}
	brokerSaveFixture(t, a, "fixture-member")
	if _, err := a.h.DB.Exec(`UPDATE finance_broker_keys SET secret=? WHERE user_id='fixture-member'`, cipher); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.loadBrokerKey("fixture-member"); err == nil {
		t.Fatal("ciphertext account substitution accepted")
	}
	if _, err := a.h.DB.Exec(`UPDATE finance_broker_keys SET environment='live' WHERE user_id='fixture-owner'`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.loadBrokerKey("fixture-owner"); err == nil {
		t.Fatal("ciphertext environment substitution accepted")
	}
	brokerSaveFixture(t, a, "fixture-owner")
	if w := brokerFixtureRequest(a, "fixture-member", "POST", "settings", `{"remove":true}`, nil); w.Code != 200 {
		t.Fatal("own disconnect failed")
	}
	if _, _, err := a.loadBrokerKey("fixture-owner"); err != nil {
		t.Fatal("member deletion affected owner", err)
	}
	log, err := os.ReadFile(filepath.Join(a.h.Config.Data, "app.log"))
	if err != nil || bytes.Contains(log, []byte("fictional-broker")) {
		t.Fatal("credential log leakage", err)
	}
}

func TestBrokerSettingsRejectInvalidRequestsWithoutReplacingExistingKey(t *testing.T) {
	a := brokerHostFixture(t)
	brokerSaveFixture(t, a, "fixture-owner")
	for _, body := range []string{
		`null`, `{}`, strings.Replace(brokerFixtureSettings, `"demo"`, `"https://evil.invalid"`, 1),
		strings.Replace(brokerFixtureSettings, `true`, `false`, 1),
		strings.Replace(brokerFixtureSettings, `fictional-broker-key`, `fictional:broker-key`, 1),
		strings.Replace(brokerFixtureSettings, `fictional-broker-secret`, `fictional\nsecret`, 1),
		strings.Replace(brokerFixtureSettings, `fictional-broker-secret`, `short`, 1),
		strings.TrimSuffix(brokerFixtureSettings, "}") + `,"url":"https://evil.invalid"}`,
		strings.TrimSuffix(brokerFixtureSettings, "}") + `,"user_id":"fixture-member"}`,
		brokerFixtureSettings + ` {}`,
		strings.Replace(brokerFixtureSettings, `fictional-broker-secret`, strings.Repeat("a", 1025), 1),
		strings.Replace(brokerFixtureSettings, `fictional-broker-secret`, strings.Repeat("&", 1024), 1),
	} {
		if w := brokerFixtureRequest(a, "fixture-owner", "POST", "settings", body, nil); w.Code != 400 {
			t.Fatal("invalid credential request accepted", w.Code)
		}
		if key, _, err := a.loadBrokerKey("fixture-owner"); err != nil || key.Key != "fictional-broker-key" {
			t.Fatal("invalid request replaced stored credential", err)
		}
	}
	for _, cipher := range [][]byte{[]byte("not a DPAPI blob"), make([]byte, 8193)} {
		if _, err := a.h.DB.Exec(`UPDATE finance_broker_keys SET secret=? WHERE user_id='fixture-owner'`, cipher); err != nil {
			t.Fatal(err)
		}
		if _, _, err := a.loadBrokerKey("fixture-owner"); err == nil {
			t.Fatal("malformed protected secret accepted")
		}
	}
}

func TestBrokerRefreshConsentRateAndNoSnapshotPersistence(t *testing.T) {
	a := brokerHostFixture(t)
	brokerSaveFixture(t, a, "fixture-owner")
	calls := 0
	transport := brokerFixtureRuntime(brokerFixtureSummary, brokerFixturePositions).client.Transport
	a.broker.client.Transport = aiRoundTrip(func(r *http.Request) (*http.Response, error) { calls++; return transport.RoundTrip(r) })
	for _, body := range []string{`null`, `{}`, `{"consent":false}`, `{"consent":true,"url":"https://evil.invalid"}`, `{"consent":true} {}`} {
		if w := brokerFixtureRequest(a, "fixture-owner", "POST", "refresh", body, nil); w.Code != 400 || calls != 0 {
			t.Fatal("invalid/unconsented refresh sent", w.Code, calls)
		}
	}
	if w := brokerFixtureRequest(a, "fixture-member", "POST", "refresh", `{"consent":true}`, nil); w.Code != 422 || calls != 0 {
		t.Fatal("owner credentials reused for member")
	}
	w := brokerFixtureRequest(a, "fixture-owner", "POST", "refresh", `{"consent":true}`, nil)
	if w.Code != 200 || calls != 2 || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), `"cashAvailable":"110.10"`) {
		t.Fatal("read-only snapshot route", w.Code, calls, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "fictional-broker") || strings.Contains(w.Body.String(), "987654321") {
		t.Fatal("private connection identifier returned")
	}
	if w := brokerFixtureRequest(a, "fixture-owner", "POST", "refresh", `{"consent":true}`, nil); w.Code != 429 || calls != 2 {
		t.Fatal("refresh rate limit bypassed")
	}
	if w := brokerFixtureRequest(a, "fixture-owner", "GET", "refresh", "", nil); w.Code != 405 && w.Code != 404 {
		t.Fatal("refresh exposed as unconsented GET", w.Code)
	}
	var count int
	if err := a.h.DB.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name LIKE 'finance_broker_%'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("unexpected broker snapshot persistence", err, count)
	}
	log, err := os.ReadFile(filepath.Join(a.h.Config.Data, "app.log"))
	if err != nil || bytes.Contains(log, []byte("Fictional company")) || bytes.Contains(log, []byte("987654321")) || bytes.Contains(log, []byte("110.10")) {
		t.Fatal("broker snapshot logged", err)
	}
	if _, err := a.h.DB.Exec(`UPDATE host_sessions SET expires='2000-01-01T00:00:00Z' WHERE user_id='fixture-owner'`); err != nil {
		t.Fatal(err)
	}
	if w := brokerFixtureRequest(a, "fixture-owner", "POST", "refresh", `{"consent":true}`, nil); w.Code != 401 || calls != 2 {
		t.Fatal("expired session refreshed broker")
	}
}

func TestBrokerDisconnectReplacementAndEnvironmentChangeInvalidateInflightRead(t *testing.T) {
	for _, action := range []string{"remove", "replace", "environment"} {
		t.Run(action, func(t *testing.T) {
			a := brokerHostFixture(t)
			brokerSaveFixture(t, a, "fixture-owner")
			entered, release := make(chan struct{}), make(chan struct{})
			defer close(release)
			transport := brokerFixtureRuntime(brokerFixtureSummary, brokerFixturePositions).client.Transport
			a.broker.client.Transport = aiRoundTrip(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, "/positions") {
					close(entered)
					select {
					case <-release:
					case <-r.Context().Done():
						return nil, r.Context().Err()
					}
				}
				return transport.RoundTrip(r)
			})
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() { done <- brokerFixtureRequest(a, "fixture-owner", "POST", "refresh", `{"consent":true}`, nil) }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("refresh did not start")
			}
			if w := brokerFixtureRequest(a, "fixture-owner", "POST", "refresh", `{"consent":true}`, nil); w.Code != 429 {
				t.Fatal("concurrent refresh accepted")
			}
			if action == "environment" {
				if _, err := a.h.DB.Exec(`UPDATE finance_broker_keys SET environment='live' WHERE user_id='fixture-owner'`); err != nil {
					t.Fatal(err)
				}
			} else {
				body := `{"remove":true}`
				if action == "replace" {
					body = strings.Replace(brokerFixtureSettings, "fictional-broker-key", "fictional-replacement-key", 1)
				}
				if w := brokerFixtureRequest(a, "fixture-owner", "POST", "settings", body, nil); w.Code != 200 {
					t.Fatal("credential change failed", w.Code)
				}
			}
			release <- struct{}{}
			select {
			case w := <-done:
				if w.Code != 422 || strings.Contains(w.Body.String(), "cashAvailable") || strings.Contains(w.Body.String(), "Fictional company") {
					t.Fatal("obsolete snapshot returned", w.Code, w.Body.String())
				}
			case <-time.After(5 * time.Second):
				t.Fatal("refresh did not finish")
			}
		})
	}
}

func TestBrokerProductionClientHasNoProxyRedirectOrRetries(t *testing.T) {
	a := brokerHostFixture(t)
	transport, ok := a.broker.client.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil || a.broker.client.Timeout != 25*time.Second {
		t.Fatal("unsafe production transport configuration")
	}
	calls := 0
	a.broker.client.Transport = aiRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		resp := brokerResponse(r, 302, "private upstream diagnostic")
		resp.Header.Set("Location", "https://untrusted.invalid/collect")
		return resp, nil
	})
	if _, err := a.broker.snapshot(context.Background(), brokerFixtureKey("demo")); err == nil || calls != 1 || strings.Contains(err.Error(), "private upstream") {
		t.Fatal("production redirect/retry boundary", err, calls)
	}
	// A protected envelope must be for this exact credential version too.
	key := brokerFixtureKey("demo")
	key.User, key.Version = "fixture-owner", 2
	raw, _ := json.Marshal(key)
	cipher, err := protectSession(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.h.DB.Exec(`INSERT INTO finance_broker_keys(user_id,environment,secret) VALUES('fixture-owner','demo',?)`, cipher); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.loadBrokerKey("fixture-owner"); err == nil {
		t.Fatal("unsupported envelope version accepted")
	}
}
