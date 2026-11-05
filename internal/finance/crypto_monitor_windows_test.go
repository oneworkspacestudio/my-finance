//go:build windows

package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
)

func cryptoMonitorFixture(t *testing.T) *App {
	t.Helper()
	a := aiHostFixture(t)
	for _, routes := range []func() error{a.investmentRoutes, a.brokerRoutes, a.cryptoRoutes, a.cryptoWalletRoutes} {
		if e := routes(); e != nil {
			t.Fatal(e)
		}
	}
	cryptoTestKey(t, a, "fictional-monitor-key")
	return a
}
func cryptoTestKey(t *testing.T, a *App, key string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"provider": "deepseek", "model": "deepseek-flash", "apiKey": key})
	w := aiFixtureRequest(a, "fixture-owner", "POST", "settings", string(body), nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
}
func cryptoMonitorRequest(a *App, user, method, path string, body any, alter func(*http.Request)) *httptest.ResponseRecorder {
	var raw []byte
	if s, ok := body.(string); ok {
		raw = []byte(s)
	} else if body != nil {
		raw, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, "http://127.0.0.1:25201/finance/api/finance/crypto"+path, bytes.NewReader(raw))
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
func cryptoEnableFixture(t *testing.T, a *App, c cryptoConfig) cryptoEnvelope {
	t.Helper()
	w := cryptoMonitorRequest(a, "fixture-owner", "POST", "/settings", map[string]any{"revision": 0, "consent": true, "config": c}, nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	v, e := a.cryptoLoad("fixture-owner")
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func cryptoMockPublic(t *testing.T, a *App) {
	t.Helper()
	a.crypto.client = &http.Client{Transport: aiRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || r.URL.String() != "https://api.dexscreener.com/token-pairs/v1/base/"+cryptoWETHAddress || r.Header.Get("Authorization") != "" {
			t.Error("unexpected public market request")
		}
		return cryptoTestResponse(200, `[{"chainId":"base","pairAddress":"0x2222222222222222222222222222222222222222","baseToken":{"address":"`+cryptoWETHAddress+`"},"quoteToken":{"address":"`+cryptoUSDCAddress+`"},"priceUsd":"2000","priceNative":"2000","liquidity":{"usd":1000000}}]`), nil
	})}
	a.wallet.client = walletRPCFixture(t, func(method string, params []json.RawMessage) any {
		switch method {
		case "eth_chainId":
			return "0x2105"
		case "eth_blockNumber":
			return "0x10"
		case "eth_getBlockByNumber":
			return map[string]string{"number": "0x10", "timestamp": "0x" + big.NewInt(time.Now().Unix()).Text(16)}
		case "eth_getBalance":
			return "0x2386f26fc10000"
		case "eth_call":
			return "0x" + cryptoIntWord(1000000000)
		}
		t.Error("unexpected read-only RPC", method)
		return nil
	}).wallet.client
}
func cryptoMockPaid(t *testing.T, a *App, kind string, hook func(*http.Request)) *atomic.Int32 {
	t.Helper()
	calls := &atomic.Int32{}
	a.ai.client = &http.Client{Transport: aiRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		v, e := a.cryptoLoad("fixture-owner")
		if e != nil || v.State.Usage.Analyses != 1 || v.State.Usage.ReservedSpendUSD != "0.009000" || len(v.Attempts) != 1 || v.Attempts[0].Outcome != "reserved" {
			t.Error("paid call before durable reservation", e)
		}
		if r.Header.Get("Authorization") != "Bearer fictional-monitor-key" {
			t.Error("wrong key or isolation")
		}
		if hook != nil {
			hook(r)
		}
		if kind == "failure" {
			return cryptoTestResponse(429, "fictional provider secret"), nil
		}
		if r.URL.String() == "https://api.deepseek.com/v1/tools/search" {
			if kind == "noSources" {
				return cryptoTestResponse(200, `{"results":[]}`), nil
			}
			return cryptoTestResponse(200, `{"results":[{"title":"Fictional news","url":"https://example.com/news","date":"`+time.Now().UTC().Format(time.RFC3339)+`","snippet":"Fictional test evidence"},{"title":"Fictional public social","url":"https://bsky.app/profile/example/post/1","date":"`+time.Now().UTC().Format(time.RFC3339)+`"}]}`), nil
		}
		if r.URL.String() != "https://api.deepseek.com/chat/completions" {
			t.Error("paid request outside allowlist")
		}
		_, _, _, d := cryptoTestData(time.Now())
		d.EvidenceIDs = []string{"market", cryptoEvidenceID("news", "https://example.com/news")}
		if kind == "hold" || kind == "noSources" {
			d.Decision, d.SellToken, d.BuyToken, d.SellAmountUSD, d.EvidenceIDs = "hold", "", "", "0", []string{}
		}
		raw, _ := json.Marshal(d)
		reply := string(raw)
		if kind == "malformed" {
			reply = `{"decision":"propose","sellAmountUsd":"25","execute":true}`
		}
		return cryptoTestResponse(200, paperProviderFixture("deepseek", reply)), nil
	})}
	return calls
}
func TestCryptoMonitorHTTPAuthConsentIsolationAndEncryptedBinding(t *testing.T) {
	a := cryptoMonitorFixture(t)
	if w := cryptoMonitorRequest(a, "", "GET", "", nil, nil); w.Code != 401 {
		t.Fatal("anonymous", w.Code)
	}
	input := map[string]any{"revision": 0, "consent": true, "config": cryptoTestConfig()}
	for name, alter := range map[string]func(*http.Request){"csrf": func(r *http.Request) { r.Header.Del("X-CSRF-Token") }, "origin": func(r *http.Request) { r.Header.Set("Origin", "https://evil.invalid") }, "account": func(r *http.Request) { r.Header.Set("X-Workspace-Expected-Account", "fixture-member") }} {
		w := cryptoMonitorRequest(a, "fixture-owner", "POST", "/settings", input, alter)
		if w.Code != 403 && w.Code != 409 {
			t.Fatal(name, w.Code)
		}
	}
	input["consent"] = false
	if w := cryptoMonitorRequest(a, "fixture-owner", "POST", "/settings", input, nil); w.Code != 400 {
		t.Fatal("missing consent", w.Code)
	}
	if w := cryptoMonitorRequest(a, "fixture-owner", "POST", "/settings", `{"revision":0,"unknown":true}`, nil); w.Code != 400 {
		t.Fatal("unknown field", w.Code)
	}
	v := cryptoEnableFixture(t, a, cryptoTestConfig())
	if w := cryptoMonitorRequest(a, "fixture-owner", "GET", "", nil, nil); w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "fictional-monitor-key") {
		t.Fatal("private response", w.Code, w.Body.String())
	}
	if w := cryptoMonitorRequest(a, "fixture-member", "GET", "", nil, nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"configured":false`) {
		t.Fatal("other user sees monitor", w.Body.String())
	}
	if w := cryptoMonitorRequest(a, "fixture-owner", "POST", "/settings", input, nil); w.Code != 412 {
		t.Fatal("stale revision", w.Code)
	}
	var secret []byte
	if e := a.h.DB.QueryRow(`SELECT secret FROM finance_crypto_monitors WHERE user_id='fixture-owner'`).Scan(&secret); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(secret, []byte(v.State.Config.WalletAddress)) || bytes.Contains(secret, []byte("fictional-monitor-key")) {
		t.Fatal("unencrypted state")
	}
	if _, e := a.h.DB.Exec(`INSERT INTO finance_crypto_monitors VALUES('fixture-member',1,1,'',?)`, secret); e != nil {
		t.Fatal(e)
	}
	if _, e := a.cryptoLoad("fixture-member"); e == nil {
		t.Fatal("cross-user envelope accepted")
	}
	v.State.Usage.ReservedSpendUSD = "garbage"
	if e := a.cryptoSave(v, v.State.Revision); e == nil {
		t.Fatal("corrupt cost ledger accepted")
	}
}
func TestCryptoMonitorDurableReservationFailuresAndPendingEvidence(t *testing.T) {
	for _, kind := range []string{"proposal", "hold", "noSources", "failure", "malformed"} {
		t.Run(kind, func(t *testing.T) {
			a := cryptoMonitorFixture(t)
			cryptoEnableFixture(t, a, cryptoTestConfig())
			cryptoMockPublic(t, a)
			calls := cryptoMockPaid(t, a, kind, nil)
			a.cryptoRun(context.Background(), "fixture-owner")
			v, e := a.cryptoLoad("fixture-owner")
			if e != nil {
				t.Fatal(e)
			}
			wantCalls := int32(3)
			wantStatus := "hold"
			if kind == "proposal" {
				wantStatus = "proposal"
			}
			if kind == "failure" {
				wantCalls = 1
				wantStatus = "error"
			}
			if kind == "malformed" {
				wantStatus = "error"
			}
			if calls.Load() != wantCalls || v.State.Status != wantStatus || v.State.Usage.Analyses != 1 || v.State.Usage.ReservedSpendUSD != "0.009000" {
				t.Fatal(kind, calls.Load(), v.State)
			}
			if strings.Contains(v.State.LastError, "fictional provider secret") {
				t.Fatal("raw provider error persisted")
			}
			if kind == "hold" || kind == "noSources" {
				if v.State.LastAnalysis == "" || len(v.State.Proposals) != 0 {
					t.Fatal("hold invisible or invented proposal")
				}
			}
			if kind == "proposal" {
				if len(v.State.Proposals) != 1 || len(v.State.Evidence) != 3 {
					t.Fatal("missing proposal evidence")
				}
				p := v.State.Proposals[0]
				intent, e := a.cryptoReviewIntent("fixture-owner", p.ID)
				if e != nil || intent.SellAmount != "25000000" {
					t.Fatal(intent, e)
				}
				if _, e = a.cryptoReviewIntent("fixture-member", p.ID); e == nil {
					t.Fatal("cross-account intent")
				}
				if w := cryptoMonitorRequest(a, "fixture-owner", "POST", "/check", map[string]any{"revision": v.State.Revision}, nil); w.Code != 400 {
					t.Fatal("pending overwritten")
				}
				a.cryptoRun(context.Background(), "fixture-owner")
				again, _ := a.cryptoLoad("fixture-owner")
				if calls.Load() != 3 || again.State.Evidence[1] != v.State.Evidence[1] || !a.cryptoIntentCurrent("fixture-owner", intent) {
					t.Fatal("pending evidence changed")
				}
				if w := cryptoMonitorRequest(a, "fixture-owner", "POST", "/dismiss", map[string]any{"revision": v.State.Revision, "proposalId": p.ID}, nil); w.Code != 200 {
					t.Fatal(w.Code, w.Body.String())
				}
				if a.cryptoIntentCurrent("fixture-owner", intent) {
					t.Fatal("dismissed intent live")
				}
			}
		})
	}
}
func TestCryptoMonitorPauseRotationDisableDiscardLateResult(t *testing.T) {
	for _, change := range []string{"pause", "key", "disable"} {
		t.Run(change, func(t *testing.T) {
			a := cryptoMonitorFixture(t)
			initial := cryptoEnableFixture(t, a, cryptoTestConfig())
			cryptoMockPublic(t, a)
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			calls := cryptoMockPaid(t, a, "proposal", func(r *http.Request) {
				if r.URL.Path == "/chat/completions" {
					close(entered)
					<-release
				}
			})
			go func() { defer close(done); a.cryptoRun(context.Background(), "fixture-owner") }()
			select {
			case <-entered:
			case <-time.After(10 * time.Second):
				close(release)
				t.Fatal("model not reached")
			}
			switch change {
			case "pause":
				w := cryptoMonitorRequest(a, "fixture-owner", "POST", "/pause", map[string]any{"revision": initial.State.Revision}, nil)
				if w.Code != 200 {
					t.Fatal(w.Code, w.Body.String())
				}
			case "key":
				cryptoTestKey(t, a, "fictional-replacement-key")
			case "disable":
				if _, e := a.h.DB.Exec(`UPDATE host_users SET role='disabled' WHERE id='fixture-owner'`); e != nil {
					t.Fatal(e)
				}
			}
			close(release)
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("run did not exit")
			}
			v, e := a.cryptoLoad("fixture-owner")
			if e != nil || v.State.Config.Enabled || v.State.Status != "paused" || len(v.State.Proposals) != 0 || v.State.Usage.Analyses != 1 || calls.Load() != 3 {
				t.Fatal("stale result committed", e, v.State, calls.Load())
			}
			if len(a.crypto.inflight) != 0 {
				t.Fatal("inflight cancellation retained")
			}
			if a.ai.usage["fixture-owner"].Busy {
				t.Fatal("AI busy retained")
			}
		})
	}
}
func TestCryptoMonitorRestartPreservesReservationAndSchedulesWithoutReplay(t *testing.T) {
	a := cryptoMonitorFixture(t)
	cryptoEnableFixture(t, a, cryptoTestConfig())
	v, _, _, cancel, e := a.cryptoStart(context.Background(), "fixture-owner")
	if e != nil {
		t.Fatal(e)
	}
	cancel()
	a.aiEnd("fixture-owner")
	config := a.h.Config
	a.h.Close()
	h, e := apphost.New(config, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer h.Close()
	restarted := &App{h: h}
	for _, routes := range []func() error{restarted.aiRoutes, restarted.cryptoRoutes} {
		if e = routes(); e != nil {
			t.Fatal(e)
		}
	}
	after, e := restarted.cryptoLoad("fixture-owner")
	if e != nil {
		t.Fatal(e)
	}
	next, e := time.Parse(time.RFC3339Nano, after.State.NextRun)
	if e != nil || !after.State.Config.Enabled || after.State.Status != "scheduled" || after.State.Revision != v.State.Revision+1 || after.State.Usage.Analyses != 1 || after.State.Usage.ReservedSpendUSD != "0.009000" || after.Attempts[0].Outcome != "interrupted" || next.Before(time.Now().Add(29*time.Minute)) {
		t.Fatal("restart lost opt-in/budget or replayed", after, e)
	}
	if _, _, _, _, e = restarted.cryptoStart(context.Background(), "fixture-owner"); e == nil {
		t.Fatal("restart immediate paid catchup")
	}
}
func TestCryptoMonitorDailyLimitsAndKeyChangeBeforeCalls(t *testing.T) {
	for _, kind := range []string{"cost", "analyses", "model", "key", "disabled"} {
		t.Run(kind, func(t *testing.T) {
			a := cryptoMonitorFixture(t)
			c := cryptoTestConfig()
			if kind == "cost" {
				c.MaxSpendUSD = "0.01"
			}
			if kind == "analyses" {
				c.MaxAnalysesPerDay = 1
			}
			v := cryptoEnableFixture(t, a, c)
			if kind == "cost" || kind == "analyses" {
				v.State.Usage.Analyses = 1
				v.State.Usage.ReservedSpendUSD = "0.009000"
				old := v.State.Revision
				v.State.Revision++
				if e := a.cryptoSave(v, old); e != nil {
					t.Fatal(e)
				}
			}
			if kind == "key" {
				cryptoTestKey(t, a, "fictional-rotated-key")
			}
			if kind == "model" {
				if _, e := a.h.DB.Exec(`UPDATE finance_ai_keys SET model='deepseek-pro' WHERE user_id='fixture-owner'`); e != nil {
					t.Fatal(e)
				}
			}
			if kind == "disabled" {
				if _, e := a.h.DB.Exec(`UPDATE host_users SET role='disabled' WHERE id='fixture-owner'`); e != nil {
					t.Fatal(e)
				}
			}
			if _, _, _, _, e := a.cryptoStart(context.Background(), "fixture-owner"); e == nil {
				t.Fatal("invalid check reserved")
			}
			after, e := a.cryptoLoad("fixture-owner")
			if e != nil {
				t.Fatal(e)
			}
			if after.State.Usage.Analyses != v.State.Usage.Analyses || after.State.Usage.ReservedSpendUSD != v.State.Usage.ReservedSpendUSD {
				t.Fatal("rejection changed costs")
			}
			if kind == "cost" || kind == "analyses" {
				if after.State.Status != "limit" {
					t.Fatal(after.State.Status)
				}
			} else if after.State.Config.Enabled {
				t.Fatal("changed account/key still enabled")
			}
		})
	}
}

func TestCryptoMonitorReceiptRemainsConfirmableAfterPause(t *testing.T) {
	a := cryptoMonitorFixture(t)
	cryptoEnableFixture(t, a, cryptoTestConfig())
	cryptoMockPublic(t, a)
	cryptoMockPaid(t, a, "proposal", nil)
	a.cryptoRun(context.Background(), "fixture-owner")
	v, e := a.cryptoLoad("fixture-owner")
	if e != nil || len(v.State.Proposals) != 1 {
		t.Fatal(e, v.State)
	}
	id := v.State.Proposals[0].ID
	w := cryptoMonitorRequest(a, "fixture-owner", "POST", "/pause", map[string]any{"revision": v.State.Revision}, nil)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	hash := "0x" + strings.Repeat("a", 64)
	for _, status := range []string{"submitted", "unknown", "confirmed", "confirmed"} {
		if e := a.cryptoRecordTransaction("fixture-owner", id, hash, status); e != nil {
			t.Fatal(status, e)
		}
	}
	if e := a.cryptoRecordTransaction("fixture-owner", id, "0x"+strings.Repeat("b", 64), "confirmed"); e == nil {
		t.Fatal("conflicting hash replaced")
	}
	if e := a.cryptoRecordTransaction("fixture-owner", id, hash, "unknown"); e == nil {
		t.Fatal("final receipt downgraded")
	}
	if e := a.cryptoRecordTransaction("fixture-member", id, hash, "confirmed"); e == nil {
		t.Fatal("cross-user receipt")
	}
	after, e := a.cryptoLoad("fixture-owner")
	if e != nil || after.State.Config.Enabled || after.State.Status != "paused" || after.State.Proposals[0].Status != "confirmed" || after.State.Proposals[0].TransactionHash != hash {
		t.Fatal(after, e)
	}
}

func TestCryptoMonitorUnresolvedReceiptBlocksPaidWorkAndFinalReceiptResumes(t *testing.T) {
	for _, status := range []string{"submitted", "unknown"} {
		for _, finalStatus := range []string{"confirmed", "reverted"} {
			t.Run(status+"-"+finalStatus, func(t *testing.T) {
				a := cryptoMonitorFixture(t)
				v := cryptoEnableFixture(t, a, cryptoTestConfig())
				hash := "0x" + strings.Repeat("c", 64)
				v.State.Proposals = []cryptoProposal{{ID: "fictional-old-wallet-swap", Status: status, TransactionHash: hash}}
				// Changing the current wallet must not forget a previous unresolved swap.
				v.State.Config.WalletAddress = "0x3333333333333333333333333333333333333333"
				if e := a.cryptoSave(v, v.State.Revision); e != nil {
					t.Fatal(e)
				}
				if _, _, _, _, e := a.cryptoStart(context.Background(), "fixture-owner"); e == nil {
					t.Fatal("unresolved swap reserved paid work")
				}
				waiting, e := a.cryptoLoad("fixture-owner")
				if e != nil || waiting.State.Status != "receipt" || waiting.State.Usage.Analyses != 0 || waiting.State.Usage.ReservedSpendUSD != "0.000000" || len(waiting.Attempts) != 0 {
					t.Fatal("receipt guard charged", waiting, e)
				}
				if w := cryptoMonitorRequest(a, "fixture-owner", "POST", "/check", map[string]any{"revision": waiting.State.Revision}, nil); w.Code != 400 {
					t.Fatal("manual check bypassed receipt", w.Code)
				}
				if e := a.cryptoRecordTransaction("fixture-owner", "fictional-old-wallet-swap", hash, finalStatus); e != nil {
					t.Fatal(e)
				}
				after, e := a.cryptoLoad("fixture-owner")
				if e != nil || after.State.Status != "scheduled" || after.State.LastError != "" {
					t.Fatal(after, e)
				}
				started, _, _, cancel, e := a.cryptoStart(context.Background(), "fixture-owner")
				if e != nil || started.State.Usage.Analyses != 1 {
					t.Fatal("final receipt did not unblock", e)
				}
				cancel()
				a.cryptoFinishInterrupted("fixture-owner", started.State.Revision, started.KeyID)
				a.aiEnd("fixture-owner")
			})
		}
	}
}
