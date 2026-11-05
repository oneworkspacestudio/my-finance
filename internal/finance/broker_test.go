package finance

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const brokerFixtureSummary = `{"id":987654321,"currency":"GBP","totalValue":310.15,"cash":{"availableToTrade":110.10,"reservedForOrders":0,"inPies":0},"investments":{"currentValue":200.05,"totalCost":205,"unrealizedProfitLoss":-4.95,"realizedProfitLoss":3}}`
const brokerFixturePositions = `[{"instrument":{"ticker":"FICTIONAL_US_EQ","name":"Fictional company","currency":"USD","isin":"FICTIONALISIN"},"quantity":0.123456789123456789,"currentPrice":125.45,"averagePricePaid":126,"walletImpact":{"currency":"GBP","currentValue":200.05,"unrealizedProfitLoss":-4.95},"createdAt":"2026-10-01T00:00:00Z"}]`

func brokerFixtureKey(environment string) brokerKey {
	return brokerKey{Version: 1, User: "fictional-user", Environment: environment, Key: "fictional-api-key", Secret: "fictional-api-secret"}
}
func brokerResponse(r *http.Request, code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}
}
func brokerFixtureRuntime(summary, positions string) *brokerRuntime {
	return &brokerRuntime{usage: map[string]brokerUsage{}, client: &http.Client{Transport: aiRoundTrip(func(r *http.Request) (*http.Response, error) {
		body := summary
		if r.URL.Path == "/api/v0/equity/positions" {
			body = positions
		}
		return brokerResponse(r, 200, body), nil
	})}}
}

func TestBrokerReadOnlyEndpointsAuthenticationAndMinimalSnapshot(t *testing.T) {
	for _, environment := range []string{"demo", "live"} {
		t.Run(environment, func(t *testing.T) {
			calls := []string{}
			b := brokerFixtureRuntime(brokerFixtureSummary, brokerFixturePositions)
			delegate := b.client.Transport
			b.client.Transport = aiRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls = append(calls, r.URL.Path)
				key, secret, ok := r.BasicAuth()
				if r.Method != http.MethodGet || r.URL.Scheme != "https" || r.URL.Host != environment+".trading212.com" || r.URL.RawQuery != "" || r.URL.User != nil || r.Body != nil || !ok || key != "fictional-api-key" || secret != "fictional-api-secret" || r.Header.Get("Accept") != "application/json" {
					t.Error("upstream request escaped the read-only boundary")
				}
				return delegate.RoundTrip(r)
			})
			got, err := b.snapshot(context.Background(), brokerFixtureKey(environment))
			if err != nil || len(calls) != 2 || calls[0] != "/api/v0/equity/account/summary" || calls[1] != "/api/v0/equity/positions" {
				t.Fatal("snapshot request contract", err, calls)
			}
			if got.Environment != environment || !got.ReadOnly || got.Currency != "GBP" || got.CashAvailable != "110.10" || got.TotalValue != "310.15" || got.InvestedValue != "200.05" || got.UnrealizedProfitLoss != "-4.95" || len(got.Positions) != 1 || got.Positions[0].Quantity != "0.123456789123456789" {
				t.Fatal("snapshot lost exact values", got)
			}
			if _, err := time.Parse(time.RFC3339, got.AsOf); err != nil {
				t.Fatal("missing snapshot time")
			}
			raw, _ := json.Marshal(got)
			for _, private := range []string{"987654321", "fictional-api", "fictional-user", "FICTIONALISIN", "walletImpact", "currentPrice", "averagePricePaid", "USD"} {
				if strings.Contains(string(raw), private) {
					t.Fatalf("unnecessary upstream value exposed: %s", private)
				}
			}
		})
	}
}

func TestBrokerCannotRequestArbitraryEndpointsOrEnvironments(t *testing.T) {
	b := brokerFixtureRuntime(brokerFixtureSummary, brokerFixturePositions)
	calls := 0
	b.client.Transport = aiRoundTrip(func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New("unexpected network") })
	for _, path := range []string{"/equity/orders/market", "/equity/account/cash", "/equity/positions?url=evil", "https://evil.invalid", "/equity/positions/../orders", "//evil.invalid"} {
		if err := b.get(context.Background(), brokerFixtureKey("live"), path, &map[string]any{}); err == nil {
			t.Fatal("unsupported path accepted", path)
		}
	}
	for _, environment := range []string{"", "LIVE", "https://evil.invalid", "live.trading212.com@evil.invalid"} {
		if err := b.get(context.Background(), brokerFixtureKey(environment), "/equity/positions", &map[string]any{}); err == nil {
			t.Fatal("unsupported environment accepted")
		}
	}
	if calls != 0 {
		t.Fatal("unsupported request reached transport")
	}
}

func TestBrokerRejectsPartialNullAndInvalidFigures(t *testing.T) {
	for name, summary := range map[string]string{
		"null": "null", "empty": "{}", "missing cash": `{"currency":"GBP","totalValue":0,"investments":{"currentValue":0,"unrealizedProfitLoss":0}}`,
		"null cash":     strings.Replace(brokerFixtureSummary, `110.10`, `null`, 1),
		"quoted cash":   strings.Replace(brokerFixtureSummary, `110.10`, `"110.10"`, 1),
		"negative cash": strings.Replace(brokerFixtureSummary, `110.10`, `-1`, 1),
		"overflow":      strings.Replace(brokerFixtureSummary, `110.10`, `1e100`, 1),
		"currency":      strings.Replace(brokerFixtureSummary, `GBP`, `gbp`, 1),
		"trailing":      brokerFixtureSummary + `{}`, "truncated": brokerFixtureSummary[:30],
		"invalid utf8": strings.Replace(brokerFixtureSummary, `GBP`, "GB\xff", 1),
	} {
		t.Run("summary/"+name, func(t *testing.T) {
			if _, err := brokerFixtureRuntime(summary, brokerFixturePositions).snapshot(context.Background(), brokerFixtureKey("demo")); err == nil {
				t.Fatal("invalid summary accepted")
			}
		})
	}
	for name, positions := range map[string]string{
		"null": "null", "object": "{}", "missing fields": "[{}]", "null item": "[null]",
		"null quantity":     strings.Replace(brokerFixturePositions, `0.123456789123456789`, `null`, 1),
		"quoted quantity":   strings.Replace(brokerFixturePositions, `0.123456789123456789`, `"0.5"`, 1),
		"negative quantity": strings.Replace(brokerFixturePositions, `0.123456789123456789`, `-1`, 1),
		"control name":      strings.Replace(brokerFixturePositions, `Fictional company`, `Fictional\ncompany`, 1),
		"blank ticker":      strings.Replace(brokerFixturePositions, `FICTIONAL_US_EQ`, ` `, 1),
		"duplicate":         "[" + strings.Trim(brokerFixturePositions, "[]") + "," + strings.Trim(brokerFixturePositions, "[]") + "]",
	} {
		t.Run("positions/"+name, func(t *testing.T) {
			if _, err := brokerFixtureRuntime(brokerFixtureSummary, positions).snapshot(context.Background(), brokerFixtureKey("demo")); err == nil {
				t.Fatal("invalid positions accepted")
			}
		})
	}
	got, err := brokerFixtureRuntime(brokerFixtureSummary, "[]").snapshot(context.Background(), brokerFixtureKey("demo"))
	if err != nil || got.Positions == nil || len(got.Positions) != 0 {
		t.Fatal("valid empty portfolio rejected", err)
	}
}

func TestBrokerRemoteFailuresAreBoundedPrivateAndNotRetried(t *testing.T) {
	for _, code := range []int{301, 302, 307, 401, 403, 408, 429, 500} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			calls := 0
			b := brokerFixtureRuntime("", "")
			b.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			b.client.Transport = aiRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				resp := brokerResponse(r, code, "SECRET PROVIDER DIAGNOSTIC")
				resp.Header.Set("Location", "https://untrusted.invalid/collect")
				return resp, nil
			})
			_, err := b.snapshot(context.Background(), brokerFixtureKey("demo"))
			if err == nil || calls != 1 || strings.Contains(err.Error(), "SECRET") {
				t.Fatal("unsafe remote failure", err, calls)
			}
		})
	}
	for _, body := range []string{strings.Repeat(" ", (1<<20)+1), brokerFixtureSummary + " {}"} {
		if _, err := brokerFixtureRuntime(body, "[]").snapshot(context.Background(), brokerFixtureKey("demo")); err == nil {
			t.Fatal("unbounded/multiple response accepted")
		}
	}
	b := brokerFixtureRuntime("", "")
	calls := 0
	b.client.Transport = aiRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("SECRET transport diagnostic")
	})
	if _, err := b.snapshot(context.Background(), brokerFixtureKey("demo")); err == nil || calls != 1 || strings.Contains(err.Error(), "SECRET") {
		t.Fatal("unsafe transport failure", err, calls)
	}
}

func TestBrokerRateLimitAndNumericBounds(t *testing.T) {
	b := &brokerRuntime{usage: map[string]brokerUsage{}}
	now := time.Now()
	if !b.begin("a", now) || b.begin("a", now.Add(time.Minute)) || !b.begin("b", now) {
		t.Fatal("in-flight or user isolation failed")
	}
	b.end("a")
	if b.begin("a", now.Add(29*time.Second)) || !b.begin("a", now.Add(30*time.Second)) {
		t.Fatal("refresh interval not enforced")
	}
	for _, value := range []string{"0", "110.10", "0.123456789123456789", "1e12", "1e-100"} {
		if !validBrokerNumber(brokerJSONNumber(value), false) {
			t.Fatal("valid exact decimal rejected", value)
		}
	}
	for _, value := range []string{"", "null", "NaN", "Inf", "01", "+1", "-1", "1e13", "1e9999", strings.Repeat("1", 41)} {
		if validBrokerNumber(brokerJSONNumber(value), false) {
			t.Fatal("invalid amount accepted", value)
		}
	}
	if !validBrokerNumber("-4.95", true) {
		t.Fatal("negative profit/loss rejected")
	}
}
