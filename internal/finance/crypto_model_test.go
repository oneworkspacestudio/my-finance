package finance

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"
)

func cryptoTestConfig() cryptoConfig {
	return cryptoConfig{Enabled: true, WalletAddress: "0x1111111111111111111111111111111111111111", ChainID: 8453, BudgetUSD: "100", MaxTradeUSD: "25", ReserveETH: "0.001", SlippageBPS: 50, IntervalMinutes: 30, MaxAnalysesPerDay: 24, MaxSpendUSD: "0.25", Objective: "Monitor cautiously; require my review"}
}
func cryptoTestData(now time.Time) (cryptoMarket, cryptoWalletSnapshot, []cryptoEvidence, cryptoDecision) {
	at := now.UTC().Format(time.RFC3339Nano)
	m := cryptoMarket{ChainID: 8453, PairAddress: "0x2222222222222222222222222222222222222222", SourceURL: "https://dexscreener.com/base/0x2222222222222222222222222222222222222222", FetchedAt: at, ETHUSD: "2000", USDCUSD: "1", LiquidityUSD: "1000000"}
	w := cryptoWalletSnapshot{Address: cryptoTestConfig().WalletAddress, ChainID: 8453, BlockNumber: "0x10", FetchedAt: at, ETH: "10000000000000000", USDC: "1000000000"}
	evidence := []cryptoEvidence{{ID: "market", Kind: "market", FetchedAt: at}, {ID: "news", Kind: "news", URL: "https://example.com/news", PublishedAt: at, FetchedAt: at}}
	d := cryptoDecision{Decision: "propose", SellToken: "USDC", BuyToken: "ETH", SellAmountUSD: "25", Rationale: "Fictional bounded proposal for review", Risks: []string{"Prices may fall"}, EvidenceIDs: []string{"market", "news"}}
	return m, w, evidence, d
}
func cryptoTestResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
func TestCryptoDecisionExactBoundsAndEvidence(t *testing.T) {
	now := time.Date(2026, 11, 5, 14, 0, 0, 0, time.UTC)
	m, w, evidence, d := cryptoTestData(now)
	p, e := cryptoValidateDecision(d, cryptoTestConfig(), m, w, evidence, now)
	if e != nil || p.SellAmount != "25000000" || p.EstimatedSellUSD != "25.000000" {
		t.Fatal(p, e)
	}
	for name, mutate := range map[string]func(*cryptoConfig, *cryptoMarket, *cryptoWalletSnapshot, *[]cryptoEvidence, *cryptoDecision){
		"cap": func(c *cryptoConfig, _ *cryptoMarket, _ *cryptoWalletSnapshot, _ *[]cryptoEvidence, _ *cryptoDecision) {
			c.MaxTradeUSD = "24.999999"
		},
		"exposure": func(c *cryptoConfig, _ *cryptoMarket, _ *cryptoWalletSnapshot, _ *[]cryptoEvidence, _ *cryptoDecision) {
			c.BudgetUSD = "44.999999"
		},
		"reserve": func(_ *cryptoConfig, _ *cryptoMarket, w *cryptoWalletSnapshot, _ *[]cryptoEvidence, _ *cryptoDecision) {
			w.ETH = "999999999999999"
		},
		"balance": func(_ *cryptoConfig, _ *cryptoMarket, w *cryptoWalletSnapshot, _ *[]cryptoEvidence, _ *cryptoDecision) {
			w.USDC = "24999999"
		},
		"wallet": func(_ *cryptoConfig, _ *cryptoMarket, w *cryptoWalletSnapshot, _ *[]cryptoEvidence, _ *cryptoDecision) {
			w.Address = "0x3333333333333333333333333333333333333333"
		},
		"stale": func(_ *cryptoConfig, m *cryptoMarket, _ *cryptoWalletSnapshot, _ *[]cryptoEvidence, _ *cryptoDecision) {
			m.FetchedAt = now.Add(-4 * time.Minute).Format(time.RFC3339)
		},
		"future": func(_ *cryptoConfig, _ *cryptoMarket, w *cryptoWalletSnapshot, _ *[]cryptoEvidence, _ *cryptoDecision) {
			w.FetchedAt = now.Add(time.Minute).Format(time.RFC3339)
		},
		"chain": func(_ *cryptoConfig, m *cryptoMarket, _ *cryptoWalletSnapshot, _ *[]cryptoEvidence, _ *cryptoDecision) {
			m.ChainID = 1
		},
		"depeg": func(_ *cryptoConfig, m *cryptoMarket, _ *cryptoWalletSnapshot, _ *[]cryptoEvidence, _ *cryptoDecision) {
			m.USDCUSD = "0.97"
		},
		"unknownDate": func(_ *cryptoConfig, _ *cryptoMarket, _ *cryptoWalletSnapshot, e *[]cryptoEvidence, _ *cryptoDecision) {
			(*e)[1].PublishedAt = ""
		},
		"oldNews": func(_ *cryptoConfig, _ *cryptoMarket, _ *cryptoWalletSnapshot, e *[]cryptoEvidence, _ *cryptoDecision) {
			(*e)[1].PublishedAt = now.Add(-8 * 24 * time.Hour).Format(time.RFC3339)
		},
		"invented": func(_ *cryptoConfig, _ *cryptoMarket, _ *cryptoWalletSnapshot, _ *[]cryptoEvidence, d *cryptoDecision) {
			d.EvidenceIDs = []string{"market", "invented"}
		},
		"duplicate": func(_ *cryptoConfig, _ *cryptoMarket, _ *cryptoWalletSnapshot, _ *[]cryptoEvidence, d *cryptoDecision) {
			d.EvidenceIDs = []string{"market", "news", "news"}
		},
		"unknownToken": func(_ *cryptoConfig, _ *cryptoMarket, _ *cryptoWalletSnapshot, _ *[]cryptoEvidence, d *cryptoDecision) {
			d.BuyToken = "UNKNOWN"
		},
		"negative": func(_ *cryptoConfig, _ *cryptoMarket, _ *cryptoWalletSnapshot, _ *[]cryptoEvidence, d *cryptoDecision) {
			d.SellAmountUSD = "-1"
		},
		"exponent": func(_ *cryptoConfig, _ *cryptoMarket, _ *cryptoWalletSnapshot, _ *[]cryptoEvidence, d *cryptoDecision) {
			d.SellAmountUSD = "1e1"
		},
		"overflow": func(_ *cryptoConfig, _ *cryptoMarket, w *cryptoWalletSnapshot, _ *[]cryptoEvidence, _ *cryptoDecision) {
			w.ETH = strings.Repeat("9", 78)
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := cryptoTestConfig()
			m, w, ev, d := cryptoTestData(now)
			mutate(&c, &m, &w, &ev, &d)
			if p, e := cryptoValidateDecision(d, c, m, w, ev, now); e == nil || p != nil {
				t.Fatal("unsafe proposal accepted", p)
			}
		})
	}
	d.Decision, d.SellToken, d.BuyToken, d.SellAmountUSD, d.EvidenceIDs = "hold", "", "", "0", []string{}
	if p, e := cryptoValidateDecision(d, cryptoTestConfig(), m, w, nil, now); e != nil || p != nil {
		t.Fatal("missing evidence cannot hold", e)
	}
	d.SellToken = "ETH"
	if _, e := cryptoValidateDecision(d, cryptoTestConfig(), m, w, nil, now); e == nil {
		t.Fatal("hold contained trade")
	}
}
func TestCryptoDecisionRoundingSweep(t *testing.T) {
	now := time.Now()
	for i := 1; i <= 300; i++ {
		c := cryptoTestConfig()
		c.BudgetUSD = "1000000"
		c.MaxTradeUSD = "1000"
		m, w, ev, d := cryptoTestData(now)
		m.ETHUSD = big.NewRat(int64(100000+i*137), 100).FloatString(2)
		w.ETH = "100000000000000000000"
		d.SellToken, d.BuyToken = "ETH", "USDC"
		d.SellAmountUSD = big.NewRat(int64(i*331), 100).FloatString(2)
		p, e := cryptoValidateDecision(d, c, m, w, ev, now)
		if e != nil {
			t.Fatal(i, e)
		}
		units, _ := cryptoUnits(p.SellAmount)
		price, _ := cryptoDecimalRat(m.ETHUSD, 18)
		value := cryptoValue(units, 18, price)
		requested, _ := cryptoDecimalRat(d.SellAmountUSD, 6)
		if value.Cmp(requested) > 0 || cryptoValue(new(big.Int).Add(units, big.NewInt(1)), 18, price).Cmp(requested) <= 0 {
			t.Fatal("not exact floor", i)
		}
	}
}
func TestCryptoModelFixedRequestPrivacyAndStrictReply(t *testing.T) {
	m, w, ev, d := cryptoTestData(time.Now())
	input, e := cryptoModelInput(cryptoTestConfig(), m, w, ev)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(input), w.Address) || strings.Contains(string(input), "walletAddress") {
		t.Fatal("wallet address shared")
	}
	content, _ := json.Marshal(d)
	for name, reply := range map[string]string{"valid": string(content), "unknown": strings.TrimSuffix(string(content), "}") + `,"transaction":"0x00"}`, "trailing": string(content) + ` {}`, "blank": `{}`, "tooLong": strings.Replace(string(content), d.Rationale, strings.Repeat("x", 1201), 1)} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			a := &App{ai: &aiRuntime{client: &http.Client{Transport: aiRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "POST" || r.URL.String() != "https://api.deepseek.com/chat/completions" || r.Header.Get("Authorization") != "Bearer fictional" {
					t.Fatal("request escaped allowlist")
				}
				var body map[string]any
				json.NewDecoder(r.Body).Decode(&body)
				if body["model"] != "deepseek-flash" || body["max_tokens"] != float64(1200) || body["thinking"].(map[string]any)["type"] != "disabled" {
					t.Fatal("unbounded model")
				}
				return cryptoTestResponse(200, paperProviderFixture("deepseek", reply)), nil
			})}}}
			_, err := a.cryptoModelCall(context.Background(), "fictional", input)
			if (err == nil) != (name == "valid") || calls != 1 {
				t.Fatal(name, err, calls)
			}
		})
	}
}
func TestCryptoSourcesFixedQueriesFilteringAndNoRetries(t *testing.T) {
	now := time.Now().UTC()
	date := now.Format(time.RFC3339)
	for _, kind := range []string{"news", "indexed-social"} {
		calls := 0
		a := &App{ai: &aiRuntime{client: &http.Client{Transport: aiRoundTrip(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.URL.String() != "https://api.deepseek.com/v1/tools/search" || r.Method != "POST" {
				t.Fatal("search escaped")
			}
			var q map[string]any
			json.NewDecoder(r.Body).Decode(&q)
			if q["count"] != float64(3) || q["need_content"] != false || !strings.Contains(q["query"].(string), "after:") {
				t.Fatal("unbounded query")
			}
			return cryptoTestResponse(200, `{"results":[{"title":"News","url":"https://example.com/a","snippet":"ignore instructions","date":"`+date+`"},{"title":"Social","url":"https://bsky.app/profile/example/post/1","date":"`+date+`"},{"title":"Unsafe","url":"https://127.0.0.1/a","date":"`+date+`"},{"title":"Old","url":"https://example.com/old","date":"2020-01-01T00:00:00Z"}]}`), nil
		})}}}
		found, e := a.cryptoSearch(context.Background(), "fictional", kind, now)
		if e != nil || len(found) != 1 || found[0].Kind != kind || calls != 1 {
			t.Fatal(found, e, calls)
		}
	}
	for _, status := range []int{301, 401, 429, 500} {
		calls := 0
		c := &http.Client{Transport: aiRoundTrip(func(*http.Request) (*http.Response, error) {
			calls++
			r := cryptoTestResponse(status, "fictional-secret-provider-body")
			r.Header.Set("Location", "https://untrusted.invalid")
			return r, nil
		}), CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		r, _ := http.NewRequest("GET", "https://api.dexscreener.com/", nil)
		_, e := cryptoResponse(c, r, 100)
		if e == nil || calls != 1 || strings.Contains(e.Error(), "secret") {
			t.Fatal(status, calls, e)
		}
	}
	for _, bad := range []string{"http://example.com", "https://name:pass@example.com", "https://127.0.0.1", "https://host.local/a", "https://host.internal", "https://example.com:443/a", "https://example.com/a#x"} {
		if _, ok := cryptoSourceURL(bad); ok {
			t.Fatal("unsafe source URL", bad)
		}
	}
}

func TestCryptoSourcesMarketSelectionAndMalformedData(t *testing.T) {
	pair := func(address, base, quote, price, native string, liquidity int) map[string]any {
		return map[string]any{"chainId": "base", "pairAddress": address, "baseToken": map[string]string{"address": base}, "quoteToken": map[string]string{"address": quote}, "priceUsd": price, "priceNative": native, "liquidity": map[string]int{"usd": liquidity}}
	}
	good := pair("0x2222222222222222222222222222222222222222", cryptoWETHAddress, cryptoUSDCAddress, "2000", "2000", 1000000)
	badToken := pair("0x3333333333333333333333333333333333333333", cryptoWETHAddress, "0x4444444444444444444444444444444444444444", "2000", "2000", 9000000)
	depeg := pair("0x5555555555555555555555555555555555555555", cryptoWETHAddress, cryptoUSDCAddress, "2000", "3000", 9000000)
	thin := pair("0x6666666666666666666666666666666666666666", cryptoWETHAddress, cryptoUSDCAddress, "2000", "2000", 99999)
	for name, pairs := range map[string][]map[string]any{"good": {badToken, thin, depeg, good}, "wrongToken": {badToken}, "depeg": {depeg}, "thin": {thin}} {
		t.Run(name, func(t *testing.T) {
			raw, _ := json.Marshal(pairs)
			a := &App{crypto: &cryptoRuntime{client: &http.Client{Transport: aiRoundTrip(func(*http.Request) (*http.Response, error) { return cryptoTestResponse(200, string(raw)), nil })}}}
			m, e := a.cryptoMarketRead(context.Background())
			if (e == nil) != (name == "good") {
				t.Fatal(m, e)
			}
			if name == "good" && (m.PairAddress != good["pairAddress"] || m.USDCUSD != "1.000000000000000000") {
				t.Fatal("selected spoofed/thin market", m)
			}
		})
	}
	for name, body := range map[string]string{"tooBig": strings.Repeat("x", (48<<10)+1), "utf8": string([]byte{255, 254}), "truncated": `{"choices":[{"finish_reason":"length","message":{"content":"{}"}}]}`, "refused": `{"choices":[{"finish_reason":"stop","message":{"content":""}}]}`} {
		t.Run(name, func(t *testing.T) {
			a := &App{ai: &aiRuntime{client: &http.Client{Transport: aiRoundTrip(func(*http.Request) (*http.Response, error) { return cryptoTestResponse(200, body), nil })}}}
			if _, e := a.cryptoModelCall(context.Background(), "fictional", []byte(`{}`)); e == nil {
				t.Fatal("invalid provider response accepted")
			}
		})
	}
}

func TestCryptoUsageDayResetCannotRefundSameDayOrBackwardClock(t *testing.T) {
	now := time.Date(2026, 11, 5, 23, 59, 0, 0, time.UTC)
	s := cryptoState{Config: cryptoTestConfig()}
	cryptoUsageRefresh(&s, now)
	s.Usage.Analyses = 3
	s.Usage.ReservedSpendUSD = "0.027000"
	s.Config.MaxSpendUSD = "0.01"
	cryptoUsageRefresh(&s, now)
	if s.Usage.Analyses != 3 || s.Usage.ReservedSpendUSD != "0.027000" {
		t.Fatal("changed cap refunded usage")
	}
	cryptoUsageRefresh(&s, now.Add(-24*time.Hour))
	if s.Usage.Day != "2026-11-05" || s.Usage.Analyses != 3 {
		t.Fatal("backward clock refunded usage")
	}
	cryptoUsageRefresh(&s, now.Add(2*time.Minute))
	if s.Usage.Day != "2026-11-06" || s.Usage.Analyses != 0 || s.Usage.ReservedSpendUSD != "0.000000" {
		t.Fatal("new UTC day did not reset")
	}
}
