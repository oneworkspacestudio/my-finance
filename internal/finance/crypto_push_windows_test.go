//go:build windows

package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type cryptoPushFixtureTransport func(*http.Request) (*http.Response, error)

func (f cryptoPushFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func pushFixture(t *testing.T) *App {
	t.Helper()
	a := aiHostFixture(t)
	// Exercise the production migration order. The shared AI fixture stops at
	// schema 4, while notification storage is schema 9.
	for _, routes := range []func() error{a.investmentRoutes, a.brokerRoutes, a.cryptoRoutes, a.cryptoWalletRoutes, a.cryptoPushRoutes} {
		if err := routes(); err != nil {
			t.Fatal(err)
		}
	}
	return a
}
func pushRequest(a *App, user, method, path, body string, alter func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:25201/finance/api/finance/crypto/push"+path, strings.NewReader(body))
	if user != "" {
		r.AddCookie(&http.Cookie{Name: "finance_session", Value: aiFixtureToken(user)})
	}
	r.Header.Set("Origin", "http://127.0.0.1:25201")
	r.Header.Set("X-CSRF-Token", "fictional-csrf")
	r.Header.Set("X-Workspace-Expected-Account", user)
	if alter != nil {
		alter(r)
	}
	w := httptest.NewRecorder()
	a.h.Handler().ServeHTTP(w, r)
	return w
}
func registerFictionalPush(t *testing.T, a *App, user string) string {
	t.Helper()
	sub := fictionalPushSubscription(t)
	raw, _ := json.Marshal(sub)
	w := pushRequest(a, user, "POST", "/subscribe", string(raw), nil)
	if w.Code != 200 {
		t.Fatalf("registration %d %s", w.Code, w.Body.String())
	}
	return sub.Endpoint
}
func TestCryptoPushPrivateSubscriptionAndAccountBinding(t *testing.T) {
	a := pushFixture(t)
	if w := pushRequest(a, "", "GET", "", "", nil); w.Code != 401 {
		t.Fatal("anonymous push settings", w.Code)
	}
	sub := fictionalPushSubscription(t)
	raw, _ := json.Marshal(sub)
	for name, alter := range map[string]func(*http.Request){
		"csrf":    func(r *http.Request) { r.Header.Del("X-CSRF-Token") },
		"origin":  func(r *http.Request) { r.Header.Set("Origin", "https://wrong.invalid") },
		"account": func(r *http.Request) { r.Header.Set("X-Workspace-Expected-Account", "someone-else") },
	} {
		t.Run(name, func(t *testing.T) {
			w := pushRequest(a, "fixture-owner", "POST", "/subscribe", string(raw), alter)
			if w.Code != 403 && w.Code != 409 {
				t.Fatal("guard failed", w.Code)
			}
		})
	}
	if w := pushRequest(a, "fixture-owner", "POST", "/subscribe", string(raw), nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var secret []byte
	if err := a.h.DB.QueryRow(`SELECT secret FROM finance_crypto_push_subscriptions`).Scan(&secret); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(secret, []byte(sub.Endpoint)) || bytes.Contains(secret, []byte(sub.Keys.Auth)) {
		t.Fatal("subscription plaintext on disk")
	}
	w := pushRequest(a, "fixture-owner", "GET", "", "", nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), sub.Endpoint) || strings.Contains(w.Body.String(), sub.Keys.Auth) {
		t.Fatal("private subscription returned", w.Code)
	}
	var state struct {
		PublicKey     string `json:"publicKey"`
		Subscriptions int    `json:"subscriptions"`
	}
	if json.Unmarshal(w.Body.Bytes(), &state) != nil || state.PublicKey == "" || state.Subscriptions != 1 {
		t.Fatal("status", w.Body.String())
	}
	if w = pushRequest(a, "fixture-member", "POST", "/subscribe", string(raw), nil); w.Code != 422 {
		t.Fatal("cross-account subscription reassignment", w.Code)
	}
	remove, _ := json.Marshal(map[string]string{"endpoint": sub.Endpoint})
	if w = pushRequest(a, "fixture-member", "POST", "/remove", string(remove), nil); w.Code != 200 {
		t.Fatal(w.Code)
	}
	var count int
	_ = a.h.DB.QueryRow(`SELECT count(*) FROM finance_crypto_push_subscriptions`).Scan(&count)
	if count != 1 {
		t.Fatal("member removed owner's subscription")
	}
	key, e := a.cryptoPushLoadKey("fixture-owner", false)
	if e != nil {
		t.Fatal(e)
	}
	_, _ = a.h.DB.Exec(`UPDATE finance_crypto_push_keys SET secret=(SELECT secret FROM finance_crypto_push_keys WHERE user_id='fixture-owner') WHERE user_id='fixture-member'`)
	// Force a transplanted encrypted key record, even when the member has no key yet.
	var keySecret []byte
	_ = a.h.DB.QueryRow(`SELECT secret FROM finance_crypto_push_keys WHERE user_id='fixture-owner'`).Scan(&keySecret)
	_, _ = a.h.DB.Exec(`INSERT OR REPLACE INTO finance_crypto_push_keys(user_id,secret) VALUES('fixture-member',?)`, keySecret)
	if _, e = a.cryptoPushLoadKey("fixture-member", false); e == nil {
		t.Fatal("transplanted VAPID key accepted")
	}
	if key.Private == "" || strings.Contains(w.Body.String(), key.Private) {
		t.Fatal("private key handling")
	}
}
func TestCryptoPushOutboxDeduplicatesEncryptsAndRemovesExpired(t *testing.T) {
	a := pushFixture(t)
	endpoint := registerFictionalPush(t, a, "fixture-owner")
	a.cryptoQueueNotification("fixture-owner", "test-fixture")
	a.cryptoQueueNotification("fixture-owner", "test-fixture")
	var count int
	_ = a.h.DB.QueryRow(`SELECT count(*) FROM finance_crypto_push_outbox`).Scan(&count)
	if count != 1 {
		t.Fatal("notification duplicate", count)
	}
	calls := 0
	a.push.client.Transport = cryptoPushFixtureTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != endpoint || r.Method != "POST" || r.Header.Get("Content-Encoding") != "aes128gcm" || !strings.HasPrefix(r.Header.Get("Authorization"), "vapid t=") {
			t.Fatal("push protocol mismatch")
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) < 100 || bytes.Contains(body, []byte("crypto-review")) || bytes.Contains(body, []byte("fixture-owner")) {
			t.Fatal("unencrypted notification payload")
		}
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	a.cryptoPushDispatch(context.Background())
	a.cryptoPushDispatch(context.Background())
	if calls != 1 {
		t.Fatal("successful job resent", calls)
	}
	var status string
	_ = a.h.DB.QueryRow(`SELECT status FROM finance_crypto_push_outbox`).Scan(&status)
	if status != "sent" {
		t.Fatal("push acceptance not recorded", status)
	}
	a.cryptoQueueNotification("fixture-owner", "test-expired")
	a.push.client.Transport = cryptoPushFixtureTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 410, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	a.cryptoPushDispatch(context.Background())
	_ = a.h.DB.QueryRow(`SELECT count(*) FROM finance_crypto_push_subscriptions`).Scan(&count)
	if count != 0 {
		t.Fatal("expired subscription retained")
	}
}
func TestCryptoPushTestConsentThrottleAndBoundedRetry(t *testing.T) {
	a := pushFixture(t)
	if w := pushRequest(a, "fixture-owner", "POST", "/test", "{}", nil); w.Code != 400 {
		t.Fatal("test without device", w.Code)
	}
	registerFictionalPush(t, a, "fixture-owner")
	if w := pushRequest(a, "fixture-owner", "POST", "/test", "{}", nil); w.Code != 202 {
		t.Fatal("explicit test", w.Code)
	}
	if w := pushRequest(a, "fixture-owner", "POST", "/test", "{}", nil); w.Code != 429 {
		t.Fatal("unbounded tests", w.Code)
	}
	calls := 0
	a.push.client.Transport = cryptoPushFixtureTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	for i := 0; i < 5; i++ {
		_, _ = a.h.DB.Exec(`UPDATE finance_crypto_push_outbox SET next_attempt=?`, time.Now().Add(-time.Minute).Unix())
		a.cryptoPushDispatch(context.Background())
	}
	if calls != 3 {
		t.Fatal("retry cap", calls)
	}
	_, _ = a.h.DB.Exec(`UPDATE host_users SET role='disabled' WHERE id='fixture-owner'`)
	a.cryptoQueueNotification("fixture-owner", "test-disabled")
	a.cryptoPushDispatch(context.Background())
	if calls != 3 {
		t.Fatal("disabled account notified")
	}
}
