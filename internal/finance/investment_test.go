package finance

import (
	"context"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func paperFixtureConfig() paperConfig {
	return paperConfig{Initial: 100000, Target: 150000, MaxLoss: 20000, MaxOrder: 20000, MaxPosition: 50000, MaxDecisions: 5}
}
func paperFixtureState() paperState {
	return newPaperState("fictional", 1, paperFixtureConfig(), []paperPrice{{"SIM-EQUITY", 123}, {"SIM-BOND", 100}, {"SIM-CASH", 100}}, aiFixtureNow())
}
func TestPaperWholeShareConservationAndProratedBasis(t *testing.T) {
	s := paperFixtureState()
	s.Config.MaxOrder = 50000
	p, e := applyPaperDecision(s, paperDecision{[]paperAction{{"buy", "SIM-EQUITY", 3}}, "Fictional purchase."}, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	if p.Cash != 99631 || p.Equity != 100000 || p.Realized != 0 {
		t.Fatal("purchase conservation", p)
	}
	for i := range p.Prices {
		if p.Prices[i].Instrument == "SIM-EQUITY" {
			p.Prices[i].Price = 130
		}
	}
	p, e = applyPaperDecision(p, paperDecision{[]paperAction{{"buy", "SIM-EQUITY", 2}}, "Second fictional purchase at a changed scenario price."}, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	p, e = applyPaperDecision(p, paperDecision{[]paperAction{{"sell", "SIM-EQUITY", 2}}, "Fictional partial sale."}, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	var holding paperHolding
	for _, h := range p.Holdings {
		if h.Instrument == "SIM-EQUITY" {
			holding = h
		}
	}
	if holding.Basis != 378 || holding.Units != 3 || p.Realized != 9 || p.Cash != 99631 || p.Cash+holding.Basis != p.Config.Initial+p.Realized {
		t.Fatal("prorated penny basis lost", holding, p)
	}
	p, e = applyPaperDecision(p, paperDecision{[]paperAction{{"sell", "SIM-EQUITY", 3}}, "Fictional closing sale."}, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	if p.Cash != 100021 || p.Realized != 21 || p.Equity != 100021 {
		t.Fatal("closing basis conservation", p)
	}
}
func TestPaperBatchRejectionIsAtomic(t *testing.T) {
	s := paperFixtureState()
	before, _ := json.Marshal(s)
	for name, actions := range map[string][]paperAction{
		"unknown instrument": {{"buy", "REAL-STOCK", 1}}, "negative": {{"buy", "SIM-EQUITY", -1}}, "short sale": {{"sell", "SIM-EQUITY", 1}}, "order cap": {{"buy", "SIM-EQUITY", 1000}}, "bad side": {{"transfer", "SIM-EQUITY", 1}}, "duplicate": {{"buy", "SIM-EQUITY", 1}, {"sell", "SIM-EQUITY", 1}}, "late failure": {{"buy", "SIM-BOND", 1}, {"sell", "SIM-EQUITY", 1}}, "too many": {{"buy", "SIM-BOND", 1}, {"buy", "SIM-CASH", 1}, {"buy", "SIM-EQUITY", 1}, {"buy", "SIM-BOND", 1}},
	} {
		t.Run(name, func(t *testing.T) {
			out, e := applyPaperDecision(s, paperDecision{Actions: actions, Reason: "Fictional invalid test."}, aiFixtureNow())
			if e == nil {
				t.Fatal("invalid order accepted")
			}
			after, _ := json.Marshal(out)
			original, _ := json.Marshal(s)
			if string(before) != string(after) || string(before) != string(original) {
				t.Fatal("rejected batch changed portfolio")
			}
		})
	}
	s.Config.MaxOrder = 100000
	s.Config.MaxPosition = 1000
	if _, e := applyPaperDecision(s, paperDecision{[]paperAction{{"buy", "SIM-EQUITY", 9}}, "Over position cap."}, aiFixtureNow()); e == nil {
		t.Fatal("position cap ignored")
	}
	s.Config.MaxPosition = 100000
	s.Cash = 100
	s.Realized = s.Cash - s.Config.Initial
	if _, e := applyPaperDecision(s, paperDecision{[]paperAction{{"buy", "SIM-EQUITY", 1}}, "Over cash."}, aiFixtureNow()); e == nil {
		t.Fatal("margin allowed")
	}
}
func TestPaperScenarioOverCapCanHoldOrGraduallyReduce(t *testing.T) {
	s := paperFixtureState()
	s.Config.MaxOrder = 50000
	s, e := applyPaperDecision(s, paperDecision{[]paperAction{{"buy", "SIM-BOND", 500}}, "Initial fictional holding."}, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	for i := range s.Prices {
		if s.Prices[i].Instrument == "SIM-BOND" {
			s.Prices[i].Price = 200
		}
	}
	s.Config.MaxOrder = 20000
	if _, e = applyPaperDecision(s, paperDecision{[]paperAction{}, "Hold existing scenario exposure."}, aiFixtureNow()); e != nil {
		t.Fatal("safe hold blocked", e)
	}
	p, e := applyPaperDecision(s, paperDecision{[]paperAction{{"sell", "SIM-BOND", 100}}, "Reduce exposure within order limit."}, aiFixtureNow())
	if e != nil {
		t.Fatal("bounded reduction blocked", e)
	}
	if _, e = applyPaperDecision(p, paperDecision{[]paperAction{{"buy", "SIM-BOND", 1}}, "Increase over-cap exposure."}, aiFixtureNow()); e == nil {
		t.Fatal("over-cap exposure increased")
	}
}
func TestPaperStopsAndBoundedAudit(t *testing.T) {
	s := paperFixtureState()
	s.Equity = s.Config.Target
	s.PnL = s.Equity - s.Config.Initial
	s.Run = &paperRun{ID: "fixture"}
	paperStop(&s)
	if s.Status != "goal_reached" || s.Run != nil {
		t.Fatal("goal not stopped")
	}
	s = paperFixtureState()
	s.PnL = -s.Config.MaxLoss
	s.Equity = s.Config.Initial + s.PnL
	paperStop(&s)
	if s.Status != "loss_limit" {
		t.Fatal("loss not stopped")
	}
	s = paperFixtureState()
	s.Attempts = s.Config.MaxDecisions
	paperStop(&s)
	if s.Status != "decision_limit" {
		t.Fatal("decision cap not stopped")
	}
	for i := 0; i < 150; i++ {
		paperRecord(&s, "hold", strings.Repeat("£", 800), nil, aiFixtureNow())
	}
	if len(s.Audit) != 50 || len(s.Audit[0].Detail) > 600 || s.Audit[0].Actions == nil {
		t.Fatal("audit unbounded or invalid")
	}
}
func TestPaperConservationScenarioSweep(t *testing.T) {
	random := rand.New(rand.NewSource(20261105))
	s := paperFixtureState()
	s.Config.MaxOrder = 50000
	for i := 0; i < 700; i++ {
		for n := range s.Prices {
			s.Prices[n].Price = 1 + random.Int63n(1000)
		}
		if e := paperMark(&s); e != nil {
			t.Fatal(e)
		}
		h := s.Holdings[random.Intn(len(s.Holdings))]
		side := "buy"
		if random.Intn(2) == 0 {
			side = "sell"
		}
		d := paperDecision{[]paperAction{{side, h.Instrument, 1 + random.Int63n(100)}}, "Fictional random conservation scenario."}
		before, _ := json.Marshal(s)
		next, e := applyPaperDecision(s, d, aiFixtureNow())
		if e != nil {
			after, _ := json.Marshal(next)
			if string(before) != string(after) {
				t.Fatal("failed decision mutated state")
			}
		} else {
			s = next
		}
		var basis, value int64
		for _, h := range s.Holdings {
			if h.Units < 0 || h.Basis < 0 {
				t.Fatal("negative holding")
			}
			basis += h.Basis
			value += h.Value
		}
		if s.Cash+basis != s.Config.Initial+s.Realized || s.Cash+value != s.Equity || s.Cash < 0 {
			t.Fatal("cash invented", s)
		}
	}
}
func paperProviderFixture(provider, content string) string {
	var result any
	if provider == "openai" {
		result = map[string]any{"status": "completed", "output": []any{map[string]any{"type": "message", "content": []any{map[string]string{"type": "output_text", "text": content}}}}}
	} else {
		result = map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": content}}}}
	}
	b, _ := json.Marshal(result)
	return string(b)
}
func TestPaperProviderFixedEndpointsStrictJSONAndNoSensitiveData(t *testing.T) {
	for _, provider := range []string{"openai", "deepseek"} {
		t.Run(provider, func(t *testing.T) {
			calls := 0
			a := &App{ai: &aiRuntime{client: &http.Client{Transport: aiRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				expected := "api.openai.com"
				if provider == "deepseek" {
					expected = "api.deepseek.com"
				}
				if r.URL.Scheme != "https" || r.URL.Host != expected || r.Header.Get("Authorization") != "Bearer fictional-key" {
					t.Error("request boundary")
				}
				body, _ := io.ReadAll(r.Body)
				for _, text := range []string{"fictional-key", "sessionHash", "user_id", "broker", "https://live.trading212.com"} {
					if strings.Contains(string(body), text) && text != "broker" {
						t.Error("private state escaped", text)
					}
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(paperProviderFixture(provider, `{"actions":[],"reason":"Hold this fictional scenario."}`)))}, nil
			})}}}
			d, e := a.paperCall(context.Background(), provider, aiDefault(provider), "fictional-key", paperFixtureState())
			if e != nil || d.Actions == nil || len(d.Actions) != 0 || calls != 1 {
				t.Fatal("valid hold failed", d, e, calls)
			}
		})
	}
	for _, content := range []string{`{"actions":[{"side":"buy","instrument":"SIM-BOND","units":1.5}],"reason":"Fractional"}`, `{"actions":[],"reason":"Hold","endpoint":"https://evil.invalid"}`, `{"actions":null,"reason":"Hold"}`, `{"reason":"Missing actions"}`, `{"actions":[],"reason":"Hold"} {}`} {
		a := &App{ai: &aiRuntime{client: &http.Client{Transport: aiRoundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(paperProviderFixture("openai", content)))}, nil
		})}}}
		if _, e := a.paperCall(context.Background(), "openai", aiDefault("openai"), "fictional-key", paperFixtureState()); e == nil {
			t.Fatal("invalid JSON actions accepted", content)
		}
	}
	if !reflect.DeepEqual(paperPreview(paperFixtureState())["config"], paperFixtureConfig()) {
		t.Fatal("preview differs from provider data")
	}
}
