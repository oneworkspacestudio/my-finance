//go:build windows

package finance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
)

func walletHostFixture(t *testing.T) *App {
	t.Helper()
	a := aiHostFixture(t)
	for _, routes := range []func() error{a.investmentRoutes, a.brokerRoutes, a.cryptoRoutes, a.cryptoWalletRoutes} {
		if e := routes(); e != nil {
			t.Fatal(e)
		}
	}
	if w := aiFixtureRequest(a, "fixture-owner", "POST", "settings", `{"provider":"deepseek","model":"deepseek-flash","apiKey":"fictional-test-key-only"}`, nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	key, e := a.cryptoKeyID("fixture-owner")
	if e != nil {
		t.Fatal(e)
	}
	intent := walletTestIntent()
	v := cryptoEnvelope{Version: 1, User: "fixture-owner", KeyID: key, ConsentedAt: time.Now().Format(time.RFC3339), State: cryptoState{Revision: 1, Config: cryptoConfig{Enabled: true, WalletAddress: intent.WalletAddress, ChainID: 8453, BudgetUSD: intent.BudgetUSD, MaxTradeUSD: intent.MaxTradeUSD, ReserveETH: intent.ReserveETH, SlippageBPS: 50, IntervalMinutes: 15, MaxAnalysesPerDay: 4, MaxSpendUSD: "0.10"}, Proposals: []cryptoProposal{{ID: intent.ProposalID, Revision: 1, Status: "pending", SellToken: intent.SellToken, BuyToken: intent.BuyToken, SellAmount: intent.SellAmount, ExpiresAt: intent.ExpiresAt.UTC().Format(time.RFC3339Nano)}}}}
	cryptoUsageRefresh(&v.State, time.Now())
	if e = a.cryptoSave(v, 0); e != nil {
		t.Fatal(e)
	}
	rpc := walletRPCFixture(t, func(method string, params []json.RawMessage) any {
		switch method {
		case "eth_chainId":
			return "0x2105"
		case "eth_blockNumber":
			return "0x10"
		case "eth_getBlockByNumber":
			return map[string]string{"number": "0x10", "timestamp": "0x" + big.NewInt(time.Now().Unix()).Text(16)}
		case "eth_getBalance":
			return "0xde0b6b3a7640000"
		case "eth_call":
			var call map[string]string
			json.Unmarshal(params[0], &call)
			if call["to"] == cryptoQuoter {
				return "0x" + cryptoIntWord(200000000) + cryptoIntWord(1) + cryptoIntWord(1) + cryptoIntWord(100000)
			}
			if strings.HasPrefix(call["data"], "0xdd62ed3e") {
				return "0x" + cryptoIntWord(0)
			}
			return "0x" + cryptoIntWord(1000000000)
		case "eth_estimateGas":
			return "0x186a0"
		case "eth_gasPrice":
			return "0xf4240"
		default:
			t.Fatal("unexpected RPC", method)
		}
		return nil
	})
	a.wallet.client = rpc.wallet.client
	a.crypto.client = &http.Client{Transport: walletTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || !strings.HasPrefix(r.URL.String(), "https://api.dexscreener.com/token-pairs/v1/base/") {
			t.Fatal("unexpected market request")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[{"chainId":"base","pairAddress":"0x2222222222222222222222222222222222222222","baseToken":{"address":"` + cryptoWETH + `"},"quoteToken":{"address":"` + cryptoUSDC + `"},"priceUsd":"2000","priceNative":"2000","liquidity":{"usd":1000000}}]`))}, nil
	})}
	return a
}
func walletRequest(a *App, user, method, path, body string, alter func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:25201/finance/api/finance/crypto/wallet/"+path, strings.NewReader(body))
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
func TestCryptoWalletRoutesGuardsEncryptedBindingAndExactPreparation(t *testing.T) {
	a := walletHostFixture(t)
	body := `{"proposalId":"fictional-proposal"}`
	if w := walletRequest(a, "", "POST", "quote", body, nil); w.Code != 401 {
		t.Fatal("anonymous", w.Code)
	}
	for name, alter := range map[string]func(*http.Request){"csrf": func(r *http.Request) { r.Header.Del("X-CSRF-Token") }, "origin": func(r *http.Request) { r.Header.Set("Origin", "https://evil.invalid") }, "account": func(r *http.Request) { r.Header.Set("X-Workspace-Expected-Account", "other") }} {
		w := walletRequest(a, "fixture-owner", "POST", "quote", body, alter)
		if w.Code != 403 && w.Code != 409 {
			t.Fatal(name, w.Code)
		}
	}
	w := walletRequest(a, "fixture-owner", "POST", "quote", body, nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var q cryptoWalletQuote
	if json.Unmarshal(w.Body.Bytes(), &q) != nil || q.BuyAmount != "200000000" || q.MinBuyAmount != "199000000" || q.Deadline <= time.Now().Unix() || q.Deadline > time.Now().Unix()+120 {
		t.Fatal("bad live-shaped quote", w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"data"`) {
		t.Fatal("quote leaked executable transaction")
	}
	var cipher []byte
	if e := a.h.DB.QueryRow(`SELECT secret FROM finance_crypto_wallet WHERE user_id='fixture-owner' AND quote_id=?`, q.ID).Scan(&cipher); e != nil || bytes.Contains(cipher, []byte(q.WalletAddress)) {
		t.Fatal("unencrypted record", e)
	}
	_, e := a.h.DB.Exec(`INSERT INTO finance_crypto_wallet(user_id,quote_id,created_at,secret)VALUES('fixture-member',?,'now',?)`, q.ID, cipher)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.cryptoWalletLoad("fixture-member", q.ID); e == nil {
		t.Fatal("cross-user ciphertext substitution")
	}
	request := `{"quoteId":"` + q.ID + `","kind":"swap"}`
	w = walletRequest(a, "fixture-owner", "POST", "prepare", request, nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var prepared struct {
		Transaction cryptoWalletTx `json:"transaction"`
	}
	json.Unmarshal(w.Body.Bytes(), &prepared)
	intent, _ := a.cryptoReviewIntent("fixture-owner", "fictional-proposal")
	expected, e := cryptoSwapData(intent, q.MinBuyAmount, q.Deadline)
	if e != nil || prepared.Transaction.Data != expected || prepared.Transaction.To != cryptoRouter || prepared.Transaction.Value != "0x16345785d8a0000" {
		t.Fatal("prepared transaction differs")
	}
	w = walletRequest(a, "fixture-owner", "POST", "prepare", request, nil)
	if w.Code != 422 {
		t.Fatal("duplicate preparation", w.Code)
	}
	w = walletRequest(a, "fixture-member", "GET", "reviews", "", nil)
	if strings.Contains(w.Body.String(), "fictional-proposal") {
		t.Fatal("cross-user history")
	}
}
func TestCryptoWalletPauseExpiryAndSessionInvalidation(t *testing.T) {
	for _, mode := range []string{"pause", "expiry", "session"} {
		t.Run(mode, func(t *testing.T) {
			a := walletHostFixture(t)
			w := walletRequest(a, "fixture-owner", "POST", "quote", `{"proposalId":"fictional-proposal"}`, nil)
			if w.Code != 200 {
				t.Fatal(w.Body.String())
			}
			var q cryptoWalletQuote
			json.Unmarshal(w.Body.Bytes(), &q)
			v, e := a.cryptoWalletLoad("fixture-owner", q.ID)
			if e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "pause":
				monitor, e := a.cryptoLoad("fixture-owner")
				if e != nil {
					t.Fatal(e)
				}
				monitor.State.Config.Enabled = false
				monitor.State.Revision++
				if e = a.cryptoSave(monitor, 1); e != nil {
					t.Fatal(e)
				}
			case "expiry":
				v.Quote.ExpiresAt = time.Now().Add(-time.Second).Format(time.RFC3339)
				if e = a.cryptoWalletSave(v); e != nil {
					t.Fatal(e)
				}
			case "session":
				v.Session = apphost.Hash("other-session")
				if e = a.cryptoWalletSave(v); e != nil {
					t.Fatal(e)
				}
			}
			w = walletRequest(a, "fixture-owner", "POST", "prepare", `{"quoteId":"`+q.ID+`","kind":"swap"}`, nil)
			if w.Code != 422 {
				t.Fatal(mode, w.Code, w.Body.String())
			}
		})
	}
}

func TestCryptoWalletLogoutDuringRPCPreventsPreparedRequest(t *testing.T) {
	a := walletHostFixture(t)
	w := walletRequest(a, "fixture-owner", "POST", "quote", `{"proposalId":"fictional-proposal"}`, nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var q cryptoWalletQuote
	json.Unmarshal(w.Body.Bytes(), &q)
	transport := a.crypto.client.Transport
	a.crypto.client.Transport = walletTestTransport(func(r *http.Request) (*http.Response, error) {
		if _, e := a.h.DB.Exec(`DELETE FROM host_sessions WHERE user_id='fixture-owner'`); e != nil {
			t.Fatal(e)
		}
		return transport.RoundTrip(r)
	})
	w = walletRequest(a, "fixture-owner", "POST", "prepare", `{"quoteId":"`+q.ID+`","kind":"swap"}`, nil)
	if w.Code != 422 || strings.Contains(w.Body.String(), `"transaction"`) {
		t.Fatal("issued after logout", w.Code, w.Body.String())
	}
	v, e := a.cryptoWalletLoad("fixture-owner", q.ID)
	if e != nil || len(v.Prepared) != 0 {
		t.Fatal("prepared record survived logout", e)
	}
}

func TestCryptoWalletUnresolvedBlocksAcrossProposalsAndWalletSettingChanges(t *testing.T) {
	a := walletHostFixture(t)
	q := walletTestQuote()
	q.Deadline = time.Now().Add(time.Minute).Unix()
	v := cryptoWalletRecord{User: "fixture-owner", Quote: q, Intent: walletTestIntent(), Prepared: map[string]cryptoWalletTx{"swap": {}}, Hashes: map[string]string{}, Status: map[string]string{}}
	save := func() {
		t.Helper()
		if e := a.cryptoWalletSave(v); e != nil {
			t.Fatal(e)
		}
	}
	save()
	if !a.cryptoWalletPending(v.User, "next-proposal", q.WalletAddress) || !a.cryptoWalletUnresolved(v.User, q.WalletAddress) {
		t.Fatal("prepared swap did not reserve wallet across proposals")
	}
	other := "0x2222222222222222222222222222222222222222"
	if a.cryptoWalletPending(v.User, "next-proposal", other) {
		t.Fatal("different wallet incorrectly blocked")
	}
	v.Quote.Deadline = time.Now().Add(-time.Second).Unix()
	save()
	if a.cryptoWalletPending(v.User, "next-proposal", q.WalletAddress) {
		t.Fatal("unbroadcast expired swap reserved forever")
	}
	v.Hashes["swap"] = "0x" + strings.Repeat("a", 64)
	v.Status["swap"] = "unknown"
	save()
	if !a.cryptoWalletPending(v.User, "next-proposal", q.WalletAddress) {
		t.Fatal("unknown submitted swap forgotten after deadline")
	}
	monitor, e := a.cryptoLoad(v.User)
	if e != nil {
		t.Fatal(e)
	}
	monitor.State.Config.WalletAddress = other
	monitor.State.Config.Enabled = false
	monitor.State.Revision++
	if e = a.cryptoSave(monitor, 1); e != nil {
		t.Fatal(e)
	}
	if !a.cryptoWalletPending(v.User, "third-proposal", q.WalletAddress) {
		t.Fatal("settings change cleared unresolved wallet reserve")
	}
	v.Status["swap"] = "confirmed"
	save()
	if a.cryptoWalletPending(v.User, "next-proposal", q.WalletAddress) || a.cryptoWalletUnresolved(v.User, q.WalletAddress) {
		t.Fatal("safe confirmation did not clear cross-proposal reserve")
	}
	if !a.cryptoWalletPending(v.User, q.ProposalID, q.WalletAddress) {
		t.Fatal("confirmed proposal duplicate allowed")
	}
	v.Status["swap"] = "reverted"
	save()
	if a.cryptoWalletPending(v.User, q.ProposalID, q.WalletAddress) {
		t.Fatal("safe revert failed to release reserve")
	}
}

func TestCryptoWalletHistoryPreservesUnresolvedAndFailsClosedAtCapacity(t *testing.T) {
	a := walletHostFixture(t)
	q := walletTestQuote()
	q.ID = "old-unresolved"
	q.Deadline = time.Now().Add(-time.Hour).Unix()
	v := cryptoWalletRecord{User: "fixture-owner", Quote: q, Intent: walletTestIntent(), Prepared: map[string]cryptoWalletTx{"swap": {}}, Hashes: map[string]string{"swap": "0x" + strings.Repeat("a", 64)}, Status: map[string]string{"swap": "unknown"}}
	if e := a.cryptoWalletSave(v); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 99; i++ {
		other := v
		other.Quote.ID = fmt.Sprintf("closed-%03d", i)
		other.Prepared = map[string]cryptoWalletTx{}
		other.Hashes = map[string]string{}
		other.Status = map[string]string{}
		if e := a.cryptoWalletSave(other); e != nil {
			t.Fatal(e)
		}
	}
	if e := a.cryptoWalletMakeRoom(v.User); e != nil {
		t.Fatal(e)
	}
	if _, e := a.cryptoWalletLoad(v.User, "old-unresolved"); e != nil {
		t.Fatal("unresolved record evicted", e)
	}
	records, e := a.cryptoWalletRecords(v.User)
	if e != nil || len(records) != 99 {
		t.Fatal("record bound", len(records), e)
	}
	for _, record := range records {
		record.Prepared = map[string]cryptoWalletTx{"approve": {}}
		record.Hashes = map[string]string{"approve": "0x" + strings.Repeat("b", 64)}
		record.Status = map[string]string{"approve": "submitted"}
		if e = a.cryptoWalletSave(record); e != nil {
			t.Fatal(e)
		}
	}
	v.Quote.ID = "last-unresolved"
	if e = a.cryptoWalletSave(v); e != nil {
		t.Fatal(e)
	}
	if e = a.cryptoWalletMakeRoom(v.User); e == nil {
		t.Fatal("full unresolved history silently evicted an outcome")
	}
	records, e = a.cryptoWalletRecords(v.User)
	if e != nil || len(records) != 100 {
		t.Fatal("protected history changed", e)
	}
}
