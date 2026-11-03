package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func aiFixtureInput() aiPlanInput {
	return aiPlanInput{Summary: aiSummary{Source: "actual", Currency: "GBP", Months: []aiMonth{{"2026-08", 300000, 200000}, {"2026-09", 280000, 210000}, {"2026-10", 320000, 190000}}, Categories: []aiCategory{{"Income", 900000, 0}, {"Spending", 0, 600000}}, TransactionCount: 30, ExcludedTransfers: 4}, Profile: aiProfile{Essentials: 150000, Emergency: 500000, Debt: "no", Horizon: 10, Risk: "balanced", Goals: []aiGoal{{"Fictional holiday", 120000, 20000, 10}}}}
}
func aiFixtureNow() time.Time { return time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC) }
func TestAICashFlowPlanPreservesMinorUnitsAndAffordability(t *testing.T) {
	in := aiFixtureInput()
	p, e := buildAIPlan(in, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	if p.AverageIncome != 300000 || p.AverageOutgoing != 200000 || p.PlanningIncome != 280000 || p.Surplus != 80000 || p.Deficit != 0 {
		t.Fatal("wrong cash flow", p)
	}
	if p.EmergencyTarget != 450000 || p.EmergencyGap != 0 || !p.InvestmentReady {
		t.Fatal("wrong readiness", p)
	}
	if len(p.Goals) != 1 || p.Goals[0].Required != 10000 || p.Goals[0].Allocated != 10000 || p.Goals[0].MonthsToGoal == nil || *p.Goals[0].MonthsToGoal != 10 {
		t.Fatal("wrong goal", p.Goals)
	}
	var allocated int64
	for _, b := range p.Buckets {
		allocated += b.Monthly
	}
	if allocated != p.PlanningIncome {
		t.Fatal("invented/unallocated money", allocated, p.PlanningIncome)
	}
	in.Profile.Emergency = 0
	p, e = buildAIPlan(in, aiFixtureNow())
	if e != nil || p.InvestmentReady || p.Goals[0].Allocated != 0 || p.Goals[0].MonthsToGoal != nil {
		t.Fatal("emergency priority ignored", p, e)
	}
	in.Profile.Debt = "yes"
	p, e = buildAIPlan(in, aiFixtureNow())
	if e != nil || p.InvestmentReady || p.Buckets[1].Name != "Debt review reserve" {
		t.Fatal("debt priority ignored", p, e)
	}
	in.Profile.Debt = "unknown"
	p, e = buildAIPlan(in, aiFixtureNow())
	if e != nil || p.Goals[0].Allocated != 0 || p.InvestmentReady {
		t.Fatal("unknown debt treated as clear", p, e)
	}
}
func TestAIPlanDeficitRoundingAndIncompleteEvidence(t *testing.T) {
	in := aiFixtureInput()
	in.Summary.Months[0].Income = 100000
	in.Summary.Categories[0].Income = 700000
	p, e := buildAIPlan(in, aiFixtureNow())
	if e != nil || p.Deficit != 100000 || p.Surplus != 0 || p.InvestmentReady || p.Goals[0].Allocated != 0 {
		t.Fatal("deficit hidden", p, e)
	}
	in = aiFixtureInput()
	in.Profile.Essentials = 290000
	p, e = buildAIPlan(in, aiFixtureNow())
	if e != nil || p.SpendingReserve != 290000 || p.Deficit != 10000 || p.Surplus != 0 || p.Goals[0].Allocated != 0 {
		t.Fatal("new essential commitments overfunded goals", p, e)
	}
	in = aiFixtureInput()
	in.Summary.Months[0].Outgoing++
	in.Summary.Categories[1].Outgoing++
	p, e = buildAIPlan(in, aiFixtureNow())
	if e != nil || p.AverageOutgoing != 200001 || p.Surplus != 79999 {
		t.Fatal("spending rounding overcommitted", p, e)
	}
	in = aiFixtureInput()
	in.Summary.Months = in.Summary.Months[2:]
	in.Summary.Categories = []aiCategory{{"All", 320000, 190000}}
	p, e = buildAIPlan(in, aiFixtureNow())
	if e != nil || p.InvestmentReady {
		t.Fatal("one month treated as enough evidence", p, e)
	}
	in = aiFixtureInput()
	p, e = buildAIPlan(in, aiFixtureNow().AddDate(0, 1, 0))
	if e != nil || p.InvestmentReady {
		t.Fatal("stale evidence treated as current", p, e)
	}
	in = aiFixtureInput()
	in.Summary.Source = "manual"
	in.Summary.TransactionCount = 0
	p, e = buildAIPlan(in, aiFixtureNow())
	if e != nil || p.InvestmentReady || p.Surplus != 80000 || !strings.Contains(strings.Join(p.Warnings, " "), "manual estimates") {
		t.Fatal("manual estimates rejected or treated as verified", p, e)
	}
}

func TestAIInvestmentDiscussionRequiresMoneyAfterSavingsGoals(t *testing.T) {
	for _, required := range []int64{80000, 80001} {
		in := aiFixtureInput()
		in.Profile.Goals = []aiGoal{{Name: "Fictional priority goal", Target: required, Saved: 0, Months: 1}}
		plan, err := buildAIPlan(in, aiFixtureNow())
		if err != nil || plan.Surplus != 80000 || plan.Goals[0].Allocated != 80000 || plan.InvestmentReady {
			t.Fatal("fully allocated or underfunded goals left investment readiness enabled", plan, err)
		}
		for _, bucket := range plan.Buckets {
			if bucket.Name == "Uncommitted buffer" {
				t.Fatal("invented money after savings allocation")
			}
		}
	}
}
func TestAIRejectsInvalidOrUnreconciledSummary(t *testing.T) {
	for name, change := range map[string]func(*aiPlanInput){
		"source":             func(i *aiPlanInput) { i.Summary.Source = "unlabelled" },
		"currency":           func(i *aiPlanInput) { i.Summary.Currency = "USD" },
		"duplicate month":    func(i *aiPlanInput) { i.Summary.Months[1].Month = i.Summary.Months[0].Month },
		"missing month":      func(i *aiPlanInput) { i.Summary.Months[1].Month = "2026-10" },
		"current month":      func(i *aiPlanInput) { i.Summary.Months[2].Month = "2026-11" },
		"future month":       func(i *aiPlanInput) { i.Summary.Months[2].Month = "2028-11" },
		"negative":           func(i *aiPlanInput) { i.Summary.Months[0].Income = -1 },
		"overflow":           func(i *aiPlanInput) { i.Summary.Months[0].Income = aiMaxMoney + 1 },
		"reconciliation":     func(i *aiPlanInput) { i.Summary.Categories[0].Income++ },
		"duplicate category": func(i *aiPlanInput) { i.Summary.Categories[1].Name = "INCOME" },
		"control":            func(i *aiPlanInput) { i.Summary.Categories[0].Name = "income\nforged" },
		"empty transactions": func(i *aiPlanInput) { i.Summary.TransactionCount = 0 },
		"negative goal":      func(i *aiPlanInput) { i.Profile.Goals[0].Target = -1 },
		"goal deadline":      func(i *aiPlanInput) { i.Profile.Goals[0].Months = 0 },
		"unknown debt value": func(i *aiPlanInput) { i.Profile.Debt = "maybe" },
	} {
		t.Run(name, func(t *testing.T) {
			in := aiFixtureInput()
			change(&in)
			if _, e := buildAIPlan(in, aiFixtureNow()); e == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}

type aiRoundTrip func(*http.Request) (*http.Response, error)

func (f aiRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func aiGoodAdvice() aiAdvice {
	return aiAdvice{Summary: "Your fictional spending leaves room to plan.", Observations: []string{"Income varies."}, Actions: []string{"Review the next month's bills."}, Buckets: []string{"Create a savings category in Actual and review its target."}, Savings: "Keep emergency savings accessible.", Investing: "This is education, not a suitability assessment; investments can fall.", Questions: []string{"Are irregular annual bills included?"}}
}
func aiProviderBody(provider string, advice aiAdvice) string {
	text, _ := json.Marshal(advice)
	var out any
	if provider == "openai" {
		out = map[string]any{"status": "completed", "output": []any{map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": string(text)}}}}}
	} else {
		out = map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": string(text)}}}}
	}
	raw, _ := json.Marshal(out)
	return string(raw)
}
func TestAIProviderRequestsAreFixedPrivateAndToolFree(t *testing.T) {
	for _, provider := range []string{"openai", "deepseek"} {
		t.Run(provider, func(t *testing.T) {
			calls := 0
			a := &App{ai: &aiRuntime{client: &http.Client{Transport: aiRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				want := "api.openai.com"
				path := "/v1/responses"
				if provider == "deepseek" {
					want = "api.deepseek.com"
					path = "/chat/completions"
				}
				if r.URL.Scheme != "https" || r.URL.Host != want || r.URL.Path != path || r.Header.Get("Authorization") != "Bearer fictional-test-key" {
					t.Error("wrong request boundary")
				}
				var body map[string]any
				if json.NewDecoder(r.Body).Decode(&body) != nil {
					t.Error("invalid payload")
				}
				if body["tools"] != nil || body["model"] != aiDefault(provider) {
					t.Error("unexpected tool/model")
				}
				if provider == "openai" && body["store"] != false {
					t.Error("OpenAI response storage enabled")
				}
				encoded, _ := json.Marshal(body)
				if bytes.Contains(encoded, []byte("fictional-test-key")) {
					t.Error("key leaked into prompt")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(aiProviderBody(provider, aiGoodAdvice())))}, nil
			})}}}
			in := aiFixtureInput()
			p, _ := buildAIPlan(in, aiFixtureNow())
			v, e := a.aiCall(context.Background(), provider, aiDefault(provider), "fictional-test-key", aiAnalyseInput{Summary: in.Summary, Profile: in.Profile, Question: "Help me plan"}, p)
			if e != nil || v.Summary == "" || calls != 1 {
				t.Fatal(v, e, calls)
			}
		})
	}
}
func TestAIProviderFailureResponsesAreSanitized(t *testing.T) {
	for name, fixture := range map[string]struct {
		status int
		body   string
		err    error
	}{
		"unauthorized": {401, "secret raw provider body", nil}, "rate limited": {429, "secret raw provider body", nil}, "redirect": {302, "secret raw provider body", nil}, "network": {0, "", errors.New("secret key request URL")},
		"malformed": {200, "not json", nil}, "blank": {200, `{"choices":[{"finish_reason":"stop","message":{"content":"   "}}]}`, nil}, "truncated": {200, `{"choices":[{"finish_reason":"length","message":{"content":"{}"}}]}`, nil}, "oversized": {200, strings.Repeat("x", (128<<10)+1), nil},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			a := &App{ai: &aiRuntime{client: &http.Client{Transport: aiRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if fixture.err != nil {
					return nil, fixture.err
				}
				return &http.Response{StatusCode: fixture.status, Body: io.NopCloser(strings.NewReader(fixture.body))}, nil
			})}}}
			_, e := a.aiCall(context.Background(), "deepseek", "deepseek-flash", "fictional-key", aiAnalyseInput{}, aiPlan{})
			if e == nil || strings.Contains(e.Error(), "secret") || calls != 1 {
				t.Fatal("unsanitized/accepted/retried", e, calls)
			}
		})
	}
	for _, raw := range []string{`{"status":"incomplete","output":[]}`, `{"status":"completed","output":[{"type":"message","content":[{"type":"refusal","text":"No"}]}]}`} {
		if _, e := aiResponseContent("openai", []byte(raw)); e == nil {
			t.Fatal("OpenAI refusal/incomplete accepted")
		}
	}
}
func TestAIRateLimitConcurrentCooldownAndHourlyReset(t *testing.T) {
	a := &App{ai: &aiRuntime{usage: map[string]aiUsage{}}}
	now := aiFixtureNow()
	if !a.aiBegin("fictional", now) || a.aiBegin("fictional", now.Add(time.Minute)) {
		t.Fatal("concurrent request allowed")
	}
	a.aiEnd("fictional")
	if a.aiBegin("fictional", now.Add(time.Second)) {
		t.Fatal("cooldown bypassed")
	}
	for i := 1; i < 10; i++ {
		if !a.aiBegin("fictional", now.Add(time.Duration(i)*time.Minute)) {
			t.Fatal("normal quota rejected", i)
		}
		a.aiEnd("fictional")
	}
	if a.aiBegin("fictional", now.Add(15*time.Minute)) {
		t.Fatal("hourly quota bypassed")
	}
	if !a.aiBegin("other-fictional", now.Add(15*time.Minute)) {
		t.Fatal("cross-user quota shared")
	}
	if !a.aiBegin("fictional", now.Add(time.Hour)) {
		t.Fatal("hourly reset failed")
	}
}
