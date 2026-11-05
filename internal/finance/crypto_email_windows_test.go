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
	"sync/atomic"
	"testing"
	"time"
)

func cryptoEmailFixture(t *testing.T) *App {
	t.Helper()
	a := cryptoMonitorFixture(t)
	for _, routes := range []func() error{a.cryptoPushRoutes, a.cryptoEmailRoutes} {
		if e := routes(); e != nil {
			t.Fatal(e)
		}
	}
	return a
}
func cryptoEmailRequest(a *App, user, method, path string, body any, alter func(*http.Request)) *httptest.ResponseRecorder {
	var raw []byte
	if s, ok := body.(string); ok {
		raw = []byte(s)
	} else if body != nil {
		raw, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, "http://127.0.0.1:25201/finance/api/finance/crypto/email"+path, bytes.NewReader(raw))
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
func cryptoEmailInput(revision int64, enabled bool) map[string]any {
	return map[string]any{"revision": revision, "enabled": enabled, "consent": enabled, "from": "finance@sender.example", "to": "owner@recipient.example", "apiKey": "fictional-resend-key"}
}
func cryptoEmailSetup(t *testing.T, a *App) cryptoEmailView {
	t.Helper()
	w := cryptoEmailRequest(a, "fixture-owner", "POST", "", cryptoEmailInput(0, true), nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var v cryptoEmailView
	if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
		t.Fatal(e)
	}
	return v
}
func cryptoEmailJob(t *testing.T, a *App, proposal string) (string, string, int) {
	t.Helper()
	var id, status string
	var attempts int
	if e := a.h.DB.QueryRow(`SELECT id,status,attempts FROM finance_crypto_email_outbox WHERE proposal_id=?`, proposal).Scan(&id, &status, &attempts); e != nil {
		t.Fatal(e)
	}
	return id, status, attempts
}
func TestCryptoEmailHTTPIsolationConsentAndEncryptedBinding(t *testing.T) {
	a := cryptoEmailFixture(t)
	if w := cryptoEmailRequest(a, "", "GET", "", nil, nil); w.Code != 401 {
		t.Fatal("anonymous", w.Code)
	}
	in := cryptoEmailInput(0, true)
	for name, alter := range map[string]func(*http.Request){"csrf": func(r *http.Request) { r.Header.Del("X-CSRF-Token") }, "origin": func(r *http.Request) { r.Header.Set("Origin", "https://evil.invalid") }, "account": func(r *http.Request) { r.Header.Set("X-Workspace-Expected-Account", "fixture-member") }} {
		w := cryptoEmailRequest(a, "fixture-owner", "POST", "", in, alter)
		if w.Code != 403 && w.Code != 409 {
			t.Fatal(name, w.Code)
		}
	}
	in["consent"] = false
	if w := cryptoEmailRequest(a, "fixture-owner", "POST", "", in, nil); w.Code != 400 {
		t.Fatal("enabled without consent", w.Code)
	}
	for _, body := range []string{`{"unknown":true}`, `{} {}`} {
		if w := cryptoEmailRequest(a, "fixture-owner", "POST", "", body, nil); w.Code != 400 {
			t.Fatal("non-strict JSON", w.Code)
		}
	}
	v := cryptoEmailSetup(t, a)
	if !v.Configured || !v.Enabled || v.Revision != 1 || v.Provider != "resend" {
		t.Fatal(v)
	}
	w := cryptoEmailRequest(a, "fixture-owner", "GET", "", nil, nil)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "fictional-resend-key") {
		t.Fatal("private response", w.Code, w.Body.String())
	}
	w = cryptoEmailRequest(a, "fixture-member", "GET", "", nil, nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "owner@recipient.example") || !strings.Contains(w.Body.String(), `"configured":false`) {
		t.Fatal("cross-user data", w.Body.String())
	}
	if w := cryptoEmailRequest(a, "fixture-owner", "POST", "", cryptoEmailInput(0, true), nil); w.Code != 412 {
		t.Fatal("stale revision", w.Code)
	}
	var secret []byte
	if e := a.h.DB.QueryRow(`SELECT secret FROM finance_crypto_email WHERE user_id='fixture-owner'`).Scan(&secret); e != nil {
		t.Fatal(e)
	}
	for _, plain := range []string{"owner@recipient.example", "fictional-resend-key", "finance@sender.example"} {
		if bytes.Contains(secret, []byte(plain)) {
			t.Fatal("unprotected email secret")
		}
	}
	if _, e := a.h.DB.Exec(`INSERT INTO finance_crypto_email(user_id,revision,secret)VALUES('fixture-member',1,?)`, secret); e != nil {
		t.Fatal(e)
	}
	if _, _, e := a.cryptoEmailLoad("fixture-member"); e == nil {
		t.Fatal("copied envelope accepted")
	}
	before := a.h.Config.Origin
	a.h.Config.Origin = "https://other.example/finance"
	_, _, e := a.cryptoEmailLoad("fixture-owner")
	a.h.Config.Origin = before
	if e == nil {
		t.Fatal("origin binding ignored")
	}
	if _, e := a.h.DB.Exec(`UPDATE finance_crypto_email SET revision=2 WHERE user_id='fixture-owner'`); e != nil {
		t.Fatal(e)
	}
	if _, _, e := a.cryptoEmailLoad("fixture-owner"); e == nil {
		t.Fatal("revision binding ignored")
	}
}
func TestCryptoEmailRemoveRetainsRevisionThrottleAndClearsQueuedRecipient(t *testing.T) {
	a := cryptoEmailFixture(t)
	cryptoEmailSetup(t, a)
	w := cryptoEmailRequest(a, "fixture-owner", "POST", "/test", map[string]any{}, nil)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = cryptoEmailRequest(a, "fixture-owner", "POST", "/test", map[string]any{}, nil); w.Code != 429 {
		t.Fatal("test unthrottled", w.Code)
	}
	if w = cryptoEmailRequest(a, "fixture-owner", "POST", "", map[string]any{"revision": 1, "remove": true}, nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	c, v, e := a.cryptoEmailLoad("fixture-owner")
	if e != nil || v.Configured || v.Enabled || v.Revision != 2 || c.APIKey != "" {
		t.Fatal(c, v, e)
	}
	var pending, lastTest int
	if e = a.h.DB.QueryRow(`SELECT count(*) FROM finance_crypto_email_outbox WHERE status='pending'`).Scan(&pending); e != nil || pending != 0 {
		t.Fatal("old recipient queued", pending, e)
	}
	a.h.DB.QueryRow(`SELECT last_test FROM finance_crypto_email WHERE user_id='fixture-owner'`).Scan(&lastTest)
	if lastTest == 0 {
		t.Fatal("remove erased throttle")
	}
	in := cryptoEmailInput(2, true)
	in["apiKey"] = ""
	if w = cryptoEmailRequest(a, "fixture-owner", "POST", "", in, nil); w.Code != 400 {
		t.Fatal("removed key retained", w.Code)
	}
	in["apiKey"] = "fictional-new-email-key"
	in["to"] = "second@recipient.example"
	if w = cryptoEmailRequest(a, "fixture-owner", "POST", "", in, nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = cryptoEmailRequest(a, "fixture-owner", "POST", "/test", map[string]any{}, nil); w.Code != 429 {
		t.Fatal("remove/re-add bypassed throttle", w.Code)
	}
	in = cryptoEmailInput(3, false)
	in["apiKey"] = ""
	if w = cryptoEmailRequest(a, "fixture-owner", "POST", "", in, nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	c, v, e = a.cryptoEmailLoad("fixture-owner")
	if e != nil || v.Enabled || c.APIKey != "fictional-new-email-key" {
		t.Fatal("empty-key keep/disable", v, e)
	}
	if w = cryptoEmailRequest(a, "fixture-owner", "POST", "/test", map[string]any{}, nil); w.Code != 400 {
		t.Fatal("disabled tests enabled", w.Code)
	}
}
func TestCryptoEmailExactGenericRequestAndBoundedIdempotentRetries(t *testing.T) {
	for _, kind := range []string{"accepted", "rejected", "rate", "malformed", "redirect"} {
		t.Run(kind, func(t *testing.T) {
			a := cryptoEmailFixture(t)
			cryptoEmailSetup(t, a)
			proposal := "test-synthetic-email"
			if kind == "accepted" {
				proposal = "fictional-current-proposal"
				v := cryptoEnableFixture(t, a, cryptoTestConfig())
				v.State.Proposals = []cryptoProposal{{ID: proposal, Revision: v.State.Revision, Status: "pending", SellToken: "USDC", BuyToken: "ETH", SellAmount: "1000000", ExpiresAt: time.Now().Add(time.Minute).Format(time.RFC3339Nano)}}
				if e := a.cryptoSave(v, v.State.Revision); e != nil {
					t.Fatal(e)
				}
			}
			if e := a.cryptoEmailQueue("fixture-owner", proposal); e != nil {
				t.Fatal(e)
			}
			id, _, _ := cryptoEmailJob(t, a, proposal)
			calls := 0
			var firstBody []byte
			a.email.client.Transport = aiRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "POST" || r.URL.String() != "https://api.resend.com/emails" || r.Header.Get("Authorization") != "Bearer fictional-resend-key" || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Idempotency-Key") != "finance-crypto-"+id {
					t.Error("request escaped fixed contract")
				}
				raw, _ := io.ReadAll(r.Body)
				if calls == 1 {
					firstBody = raw
				} else if !bytes.Equal(firstBody, raw) {
					t.Error("retry body changed")
				}
				var body map[string]json.RawMessage
				if json.Unmarshal(raw, &body) != nil || len(body) != 4 {
					t.Error("unexpected provider payload")
				}
				var from, subject, text string
				var to []string
				json.Unmarshal(body["from"], &from)
				json.Unmarshal(body["subject"], &subject)
				json.Unmarshal(body["text"], &text)
				json.Unmarshal(body["to"], &to)
				if from != "finance@sender.example" || len(to) != 1 || to[0] != "owner@recipient.example" || subject != "My Finance crypto review" || !strings.Contains(text, "/#crypto") || !strings.Contains(text, "does not approve a transaction") || strings.Contains(string(raw), proposal) || strings.Contains(string(raw), "sellAmount") || strings.Contains(string(raw), "0x") {
					t.Error("non-generic notification", string(raw))
				}
				_, _, attempts := cryptoEmailJob(t, a, proposal)
				if attempts != calls {
					t.Error("attempt not durable before request", attempts, calls)
				}
				switch kind {
				case "accepted":
					return cryptoTestResponse(201, `{"id":"fictional-accepted-id"}`), nil
				case "rejected":
					return cryptoTestResponse(403, `{"error":"secret-recipient-and-provider-detail"}`), nil
				case "rate":
					return cryptoTestResponse(429, `{}`), nil
				case "malformed":
					return cryptoTestResponse(200, `{"id":`), nil
				default:
					r := cryptoTestResponse(302, `{}`)
					r.Header.Set("Location", "https://untrusted.invalid/collect")
					return r, nil
				}
			})
			for i := 0; i < 4; i++ {
				a.cryptoEmailDispatch(context.Background())
				if _, e := a.h.DB.Exec(`UPDATE finance_crypto_email_outbox SET next_attempt=0 WHERE id=?`, id); e != nil {
					t.Fatal(e)
				}
			}
			_, status, attempts := cryptoEmailJob(t, a, proposal)
			want := "exhausted"
			wantCalls := 3
			if kind == "accepted" {
				want = "accepted"
				wantCalls = 1
			}
			if kind == "rejected" {
				want = "rejected"
				wantCalls = 1
			}
			if status != want || calls != wantCalls || attempts != wantCalls {
				t.Fatal(status, calls, attempts)
			}
			_, v, e := a.cryptoEmailLoad("fixture-owner")
			if e != nil || strings.Contains(v.LastError, "secret-recipient") {
				t.Fatal(v, e)
			}
			if (v.LastAcceptedAt != "") != (kind == "accepted") {
				t.Fatal("provider acceptance reported incorrectly", v)
			}
		})
	}
}
func TestCryptoEmailStaleProposalDisabledUserAndExpiredJobsNeverSend(t *testing.T) {
	for _, kind := range []string{"stale", "paused", "disabled", "expiredJob"} {
		t.Run(kind, func(t *testing.T) {
			a := cryptoEmailFixture(t)
			cryptoEmailSetup(t, a)
			proposal := "test-synthetic-email"
			if kind == "stale" {
				proposal = "missing-proposal"
			}
			if kind == "paused" {
				v := cryptoEnableFixture(t, a, cryptoTestConfig())
				proposal = "synthetic-pending"
				v.State.Proposals = []cryptoProposal{{ID: proposal, Revision: v.State.Revision, Status: "pending", ExpiresAt: time.Now().Add(time.Minute).Format(time.RFC3339Nano)}}
				if e := a.cryptoSave(v, v.State.Revision); e != nil {
					t.Fatal(e)
				}
			}
			if e := a.cryptoEmailQueue("fixture-owner", proposal); e != nil {
				t.Fatal(e)
			}
			id, _, _ := cryptoEmailJob(t, a, proposal)
			if kind == "disabled" {
				if _, e := a.h.DB.Exec(`UPDATE host_users SET role='disabled' WHERE id='fixture-owner'`); e != nil {
					t.Fatal(e)
				}
			}
			if kind == "paused" {
				w := cryptoMonitorRequest(a, "fixture-owner", "POST", "/pause", map[string]any{"revision": 1}, nil)
				if w.Code != 200 {
					t.Fatal(w.Code)
				}
			}
			if kind == "expiredJob" {
				if _, e := a.h.DB.Exec(`UPDATE finance_crypto_email_outbox SET created=? WHERE id=?`, time.Now().Add(-31*time.Minute).Unix(), id); e != nil {
					t.Fatal(e)
				}
			}
			calls := 0
			a.email.client.Transport = aiRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++
				return cryptoTestResponse(200, `{"id":"unexpected"}`), nil
			})
			a.cryptoEmailDispatch(context.Background())
			_, status, attempts := cryptoEmailJob(t, a, proposal)
			if calls != 0 || attempts != 0 || (kind != "expiredJob" && status != "cancelled") {
				t.Fatal("stale notification sent", status, attempts, calls)
			}
		})
	}
}
func TestCryptoEmailChangedSettingsCancelInflightAndDiscardOldResult(t *testing.T) {
	a := cryptoEmailFixture(t)
	cryptoEmailSetup(t, a)
	proposal := "test-old-recipient"
	if e := a.cryptoEmailQueue("fixture-owner", proposal); e != nil {
		t.Fatal(e)
	}
	entered, cancelled, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	a.email.client.Transport = aiRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		close(entered)
		<-r.Context().Done()
		close(cancelled)
		return cryptoTestResponse(201, `{"id":"accepted-after-cancel"}`), nil
	})
	go func() { defer close(done); a.cryptoEmailDispatch(context.Background()) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request not reached")
	}
	// A concurrent dispatcher cannot duplicate the active request.
	a.cryptoEmailDispatch(context.Background())
	if calls.Load() != 1 {
		t.Fatal("overlapping email calls")
	}
	in := cryptoEmailInput(1, true)
	in["to"] = "replacement@recipient.example"
	in["apiKey"] = "fictional-replacement-email-key"
	w := cryptoEmailRequest(a, "fixture-owner", "POST", "", in, nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("old request not cancelled")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("dispatcher did not finish")
	}
	c, v, e := a.cryptoEmailLoad("fixture-owner")
	if e != nil || v.Revision != 2 || v.LastAcceptedAt != "" || v.LastError != "" || c.To != "replacement@recipient.example" {
		t.Fatal("old result polluted new config", v, e)
	}
	_, status, attempts := cryptoEmailJob(t, a, proposal)
	if status != "cancelled" || attempts != 1 || len(a.email.inflight) != 0 {
		t.Fatal(status, attempts)
	}
}
