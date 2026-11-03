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
	"time"
)

func aiPence(n int64) *int64 { return &n }
func aiManagerFixture() aiPlanInput {
	in := aiFixtureInput()
	in.Summary.Categories = []aiCategory{{"Income", 900000, 0}, {"Essentials", 0, 450000}, {"Flexible", 0, 150000}}
	in.Summary.Current = &aiCurrent{Month: "2026-11", AsOf: "2026-11-03", Income: 200000, Outgoing: 40000, Categories: []aiCategory{{"Income", 200000, 0}, {"Essentials", 0, 30000}, {"Flexible", 0, 10000}}}
	in.Profile.Manager = &aiManagerRules{Enabled: true, Savings: 40000, Investment: 10000, CashBuffer: 10000, Categories: []aiCategoryRule{{Name: "Flexible", Essential: false}}}
	return in
}
func aiFindCategory(t *testing.T, m *aiManagerPlan, name string) aiManagerCategory {
	t.Helper()
	for _, c := range m.Categories {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("category missing: %s", name)
	return aiManagerCategory{}
}
func aiAssertManagerConservation(t *testing.T, p aiPlan) {
	t.Helper()
	m := p.Manager
	if m == nil {
		t.Fatal("missing manager")
	}
	var categories int64
	for _, c := range m.Categories {
		if c.Limit < 0 || c.Remaining < 0 || c.Overspent < 0 || c.Reduction < 0 || c.Spent < 0 {
			t.Fatal("negative allocation", c)
		}
		if c.Remaining != max(0, c.Limit-c.Spent) || c.Overspent != max(0, c.Spent-min(c.Wanted, c.Limit)) || c.Reduction != max(0, c.Wanted-c.Limit) {
			t.Fatal("wrong category tracking", c)
		}
		categories += c.Limit
	}
	if categories != m.CategoryAllocated {
		t.Fatal("category total mismatch")
	}
	total := categories + m.UnassignedEssentialsAllocated + m.DebtReserved + m.EmergencyReserved + m.SavingsReserved + m.GoalReserved + m.InvestmentReserved + m.CashBufferReserved + m.Unallocated
	if total != m.PlanningIncome {
		t.Fatalf("income not conserved: allocated=%d income=%d manager=%+v", total, m.PlanningIncome, m)
	}
	var buckets int64
	for _, b := range p.Buckets {
		buckets += b.Monthly
	}
	if buckets != m.PlanningIncome {
		t.Fatalf("top-level buckets contradict manager: %d vs %d", buckets, m.PlanningIncome)
	}
	if m.InvestmentReserved > 0 && !m.InvestmentEligible {
		t.Fatal("ineligible investment funded")
	}
	if m.SavingsReserved > m.SavingsWanted || m.InvestmentReserved > m.InvestmentWanted || m.CashBufferReserved > m.CashBufferWanted {
		t.Fatal("reserve exceeded explicit request")
	}
}
func TestAIManagerAllocatesWithoutDoubleCountingAndHonoursPriority(t *testing.T) {
	in := aiManagerFixture()
	p, e := buildAIPlan(in, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	m := p.Manager
	aiAssertManagerConservation(t, p)
	if m.EssentialRequired != 150000 || m.EssentialAllocated != 150000 || m.CategoryAllocated != 200000 || m.SavingsReserved != 40000 || m.GoalReserved != 10000 || m.InvestmentReserved != 10000 || m.CashBufferReserved != 10000 || m.Unallocated != 10000 || !m.InvestmentEligible {
		t.Fatal("unexpected allocation", m)
	}
	if c := aiFindCategory(t, m, "Flexible"); c.Wanted != 50000 || c.Limit != 50000 || c.Spent != 10000 || c.Remaining != 40000 || c.Overspent != 0 {
		t.Fatal(c)
	}
	if !reflect.DeepEqual(p.Goals, m.Goals) || p.EmergencyTarget != m.EmergencyTarget {
		t.Fatal("classic plan not aligned")
	}
	in.Profile.Manager.Savings = 100000
	p, e = buildAIPlan(in, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	m = p.Manager
	aiAssertManagerConservation(t, p)
	if m.SavingsReserved != 100000 || m.GoalReserved != 10000 || m.CashBufferReserved != 10000 || m.InvestmentReserved != 0 {
		t.Fatal("explicit cushion or priorities ignored", m)
	}
	if c := aiFindCategory(t, m, "Flexible"); c.Limit != 10000 || c.Reduction != 40000 {
		t.Fatal("flexible future spending did not adjust", c)
	}
}
func TestAIManagerProtectsRecordedSpendingAndReportsOverspending(t *testing.T) {
	in := aiManagerFixture()
	in.Summary.Current.Outgoing = 90000
	in.Summary.Current.Categories[2].Outgoing = 60000
	p, e := buildAIPlan(in, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	aiAssertManagerConservation(t, p)
	c := aiFindCategory(t, p.Manager, "Flexible")
	if c.Limit != 60000 || c.Overspent != 10000 || c.Remaining != 0 || p.Manager.Unallocated != 0 {
		t.Fatal("over-target spending disappeared", c, p.Manager)
	}
	in.Summary.Current.Outgoing = 170000
	in.Summary.Current.Categories[2].Outgoing = 140000
	p, e = buildAIPlan(in, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	aiAssertManagerConservation(t, p)
	if p.Manager.SpendingFundingGap != 10000 || p.Deficit != 10000 || p.Manager.SavingsReserved != 0 || p.Manager.InvestmentReserved != 0 {
		t.Fatal("spent money reused or funding gap hidden", p.Manager)
	}
}
func TestAIManagerEssentialShortfallsAndEmergencyBasis(t *testing.T) {
	in := aiManagerFixture()
	in.Profile.Manager.Categories = append(in.Profile.Manager.Categories, aiCategoryRule{Name: "Essentials", Essential: true, Target: aiPence(250000)})
	p, e := buildAIPlan(in, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	aiAssertManagerConservation(t, p)
	if p.Essentials != 250000 || p.EmergencyTarget != 750000 || p.EmergencyGap != 250000 || p.Manager.EmergencyReserved != 20000 || p.InvestmentReady {
		t.Fatal("new essential costs understated emergency buffer", p.Manager)
	}
	in.Profile.Manager.Categories[1].Target = aiPence(400000)
	p, e = buildAIPlan(in, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	aiAssertManagerConservation(t, p)
	if p.Manager.EssentialShortfall != 120000 || p.Manager.SpendingFundingGap != 10000 || p.Deficit != 130000 || p.Manager.SavingsReserved != 0 {
		t.Fatal("essential funding shortage hidden", p.Manager)
	}
	in = aiManagerFixture()
	in.Profile.Essentials = 200000
	p, e = buildAIPlan(in, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	aiAssertManagerConservation(t, p)
	if p.Manager.UnassignedEssentials != 50000 || p.Manager.UnassignedEssentialsAllocated != 50000 || p.Manager.EssentialRequired != 200000 {
		t.Fatal("unassigned essentials lost", p.Manager)
	}
}
func TestAIManagerExplicitZeroDefaultsStaleRulesAndNewCategories(t *testing.T) {
	in := aiManagerFixture()
	in.Profile.Goals = nil
	in.Profile.Manager.Savings = 0
	in.Profile.Manager.Investment = 0
	in.Profile.Manager.CashBuffer = 0
	in.Profile.Manager.Categories[0].Target = aiPence(0)
	in.Profile.Manager.Categories = append(in.Profile.Manager.Categories, aiCategoryRule{Name: "Absent old category", Target: aiPence(99999)})
	in.Summary.Current.Categories = append(in.Summary.Current.Categories, aiCategory{Name: "New essential", Outgoing: 123})
	in.Summary.Current.Outgoing += 123
	p, e := buildAIPlan(in, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	aiAssertManagerConservation(t, p)
	if c := aiFindCategory(t, p.Manager, "Flexible"); c.Wanted != 0 || c.Limit != 10000 || c.Overspent != 10000 {
		t.Fatal("explicit zero replaced by historical average", c)
	}
	if c := aiFindCategory(t, p.Manager, "New essential"); !c.Essential || c.Limit != 123 {
		t.Fatal("unknown current category treated as optional", c)
	}
	if !strings.Contains(strings.Join(p.Manager.Warnings, " "), "retained but ignored") {
		t.Fatal("stale saved rules not disclosed")
	}
	in.Profile.Manager.Categories[0].Target = nil
	p, e = buildAIPlan(in, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	if aiFindCategory(t, p.Manager, "Flexible").Wanted != 50000 {
		t.Fatal("null target did not use history")
	}
}
func TestAIManagerInvestmentGatesAndCurrentFreshness(t *testing.T) {
	for name, change := range map[string]func(*aiPlanInput){
		"unknown debt": func(i *aiPlanInput) { i.Profile.Debt = "unknown" }, "debt": func(i *aiPlanInput) { i.Profile.Debt = "yes" },
		"emergency gap": func(i *aiPlanInput) { i.Profile.Emergency = 0 }, "short horizon": func(i *aiPlanInput) { i.Profile.Horizon = 2 },
		"unknown risk": func(i *aiPlanInput) { i.Profile.Risk = "unknown" }, "manual": func(i *aiPlanInput) { i.Summary.Source = "manual" },
		"missing current": func(i *aiPlanInput) { i.Summary.Current = nil }, "underfunded savings": func(i *aiPlanInput) { i.Profile.Manager.Savings = 300000 },
		"underfunded goal": func(i *aiPlanInput) { i.Profile.Goals[0].Target = aiMaxMoney; i.Profile.Goals[0].Months = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			in := aiManagerFixture()
			change(&in)
			p, e := buildAIPlan(in, aiFixtureNow())
			if e != nil {
				t.Fatal(e)
			}
			aiAssertManagerConservation(t, p)
			if p.Manager.InvestmentEligible || p.Manager.InvestmentReserved != 0 {
				t.Fatal("investment gate bypassed", p.Manager)
			}
		})
	}
	in := aiManagerFixture()
	p, e := buildAIPlan(in, aiFixtureNow().AddDate(0, 0, 8))
	if e != nil {
		t.Fatal(e)
	}
	if p.Manager.InvestmentEligible || !strings.Contains(strings.Join(p.Manager.Warnings, " "), "seven days") {
		t.Fatal("stale current snapshot accepted as fresh")
	}
	for name, change := range map[string]func(*aiCurrent){"future": func(c *aiCurrent) { c.AsOf = "2026-11-04" }, "previous month": func(c *aiCurrent) { c.Month = "2026-10"; c.AsOf = "2026-10-31" }, "invalid day": func(c *aiCurrent) { c.AsOf = "2026-11-99" }, "unreconciled": func(c *aiCurrent) { c.Outgoing++ }, "negative": func(c *aiCurrent) { c.Categories[0].Income = -1 }, "duplicate": func(c *aiCurrent) { c.Categories = append(c.Categories, c.Categories[0]) }} {
		t.Run(name, func(t *testing.T) {
			i := aiManagerFixture()
			change(i.Summary.Current)
			if _, e := buildAIPlan(i, aiFixtureNow()); e == nil {
				t.Fatal("invalid current summary accepted")
			}
		})
	}
}
func TestAIManagerLargestRemainderStableAndSafeAtMoneyBounds(t *testing.T) {
	if got := aiProportion([]int64{1, 1, 1}, 2); !reflect.DeepEqual(got, []int64{1, 1, 0}) {
		t.Fatal("unstable tie", got)
	}
	if got := aiProportion([]int64{aiMaxMoney, aiMaxMoney, aiMaxMoney}, aiMaxMoney); !reflect.DeepEqual(got, []int64{33333333334, 33333333333, 33333333333}) {
		t.Fatal("overflow or rounding", got)
	}
	weights := make([]int64, 200)
	for i := range weights {
		weights[i] = aiMaxMoney - int64(i)
	}
	got := aiProportion(weights, aiMaxMoney)
	var total int64
	for i, n := range got {
		if n < 0 || n > weights[i] {
			t.Fatal("allocation outside bounds")
		}
		total += n
	}
	if total != aiMaxMoney {
		t.Fatal("lost pence", total)
	}
	// The rounded category defaults must reconcile with the historical average,
	// even when a small category owns the only fractional penny.
	in := aiManagerFixture()
	for i := range in.Summary.Months {
		in.Summary.Months[i].Outgoing = 0
	}
	in.Summary.Months[0].Outgoing = 7
	in.Summary.Categories = []aiCategory{{"Income", 900000, 0}, {"A", 0, 1}, {"B", 0, 6}}
	in.Summary.Current = nil
	in.Profile.Essentials = 0
	in.Profile.Manager.Categories = nil
	p, e := buildAIPlan(in, aiFixtureNow())
	if e != nil {
		t.Fatal(e)
	}
	if aiFindCategory(t, p.Manager, "A").Historical != 1 || aiFindCategory(t, p.Manager, "B").Historical != 2 {
		t.Fatal("historical rounding did not preserve category fractions", p.Manager)
	}
}
func TestAIManagerAllocationConservationSweep(t *testing.T) {
	random := rand.New(rand.NewSource(44721))
	for iteration := 0; iteration < 600; iteration++ {
		in := aiManagerFixture()
		income := random.Int63n(1000000)
		var historical, current int64
		in.Summary.Categories = []aiCategory{{Name: "Income", Income: income * 3}}
		in.Summary.Current.Categories = []aiCategory{{Name: "Income", Income: income / 2}}
		in.Summary.Current.Income = income / 2
		in.Profile.Manager.Categories = nil
		for _, name := range []string{"A", "B", "C", "D", "E"} {
			amount := random.Int63n(300000)
			spent := random.Int63n(150000)
			historical += amount
			current += spent
			in.Summary.Categories = append(in.Summary.Categories, aiCategory{Name: name, Outgoing: amount * 3})
			in.Summary.Current.Categories = append(in.Summary.Current.Categories, aiCategory{Name: name, Outgoing: spent})
			var target *int64
			if random.Intn(3) != 0 {
				target = aiPence(random.Int63n(300000))
			}
			in.Profile.Manager.Categories = append(in.Profile.Manager.Categories, aiCategoryRule{Name: name, Essential: random.Intn(2) == 0, Target: target})
		}
		for i := range in.Summary.Months {
			in.Summary.Months[i].Income = income
			in.Summary.Months[i].Outgoing = historical
		}
		in.Summary.Current.Outgoing = current
		in.Profile.Essentials = random.Int63n(400000)
		in.Profile.Emergency = random.Int63n(4000000)
		in.Profile.Debt = []string{"yes", "no", "unknown"}[random.Intn(3)]
		in.Profile.Manager.Savings = random.Int63n(300000)
		in.Profile.Manager.Investment = random.Int63n(300000)
		in.Profile.Manager.CashBuffer = random.Int63n(300000)
		p, e := buildAIPlan(in, aiFixtureNow())
		if e != nil {
			t.Fatalf("iteration %d: %v", iteration, e)
		}
		aiAssertManagerConservation(t, p)
	}
}
func TestAIManagerSuggestionsValidateKnownCategoriesAndDoNotChangePlan(t *testing.T) {
	in := aiManagerFixture()
	makeSuggestion := func() *aiManagerSuggestion {
		return &aiManagerSuggestion{Categories: []aiCategoryRule{{Name: " flexible ", Essential: false, Target: aiPence(40000)}}, Savings: 40000, Investment: 0, CashBuffer: 10000, Reason: "A draft to review before applying."}
	}
	s := makeSuggestion()
	if e := validateAIManagerSuggestion(s, in.Summary); e != nil || s.Categories[0].Name != "Flexible" {
		t.Fatal("valid suggestion rejected", e)
	}
	for name, mutate := range map[string]func(*aiManagerSuggestion){"unknown": func(s *aiManagerSuggestion) { s.Categories[0].Name = "Invented" }, "negative": func(s *aiManagerSuggestion) { s.Savings = -1 }, "overflow": func(s *aiManagerSuggestion) { s.Investment = aiMaxMoney + 1 }, "duplicate": func(s *aiManagerSuggestion) { s.Categories = append(s.Categories, s.Categories[0]) }, "control": func(s *aiManagerSuggestion) { s.Reason = "reason\ncontrol" }, "negative category": func(s *aiManagerSuggestion) { s.Categories[0].Target = aiPence(-1) }} {
		t.Run(name, func(t *testing.T) {
			s := makeSuggestion()
			mutate(s)
			if validateAIManagerSuggestion(s, in.Summary) == nil {
				t.Fatal("invalid suggestion accepted")
			}
		})
	}
	p, _ := buildAIPlan(in, aiFixtureNow())
	before, _ := json.Marshal(in)
	answer := aiGoodAdvice()
	answer.ManagerSuggestion = makeSuggestion()
	calls := 0
	a := &App{ai: &aiRuntime{client: &http.Client{Transport: aiRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(aiProviderBody("openai", answer)))}, nil
	})}}}
	result, e := a.aiCall(context.Background(), "openai", aiDefault("openai"), "fictional-key", aiAnalyseInput{Summary: in.Summary, Profile: in.Profile}, p)
	if e != nil || result.ManagerSuggestion == nil || calls != 1 {
		t.Fatal("suggestion adapter failed", e)
	}
	after, _ := json.Marshal(in)
	if string(before) != string(after) {
		t.Fatal("model proposal mutated active inputs")
	}
	answer.ManagerSuggestion.Categories[0].Name = "Invented category"
	if _, e = a.aiCall(context.Background(), "openai", aiDefault("openai"), "fictional-key", aiAnalyseInput{Summary: in.Summary, Profile: in.Profile}, p); e == nil {
		t.Fatal("unknown model category escaped validation")
	}
}
func TestAISavedProfileStructuralManagerValidation(t *testing.T) {
	var rule aiCategoryRule
	if e := json.Unmarshal([]byte(`{"name":"Unclassified","targetMonthlyMinor":null}`), &rule); e != nil || !rule.Essential {
		t.Fatal("omitted classification became optional", e)
	}
	if e := json.Unmarshal([]byte(`{"name":"Unclassified","essential":false,"unexpected":"ignored"}`), &rule); e == nil {
		t.Fatal("unknown rule field ignored")
	}
	in := aiManagerFixture()
	in.Profile.Manager.Categories = append(in.Profile.Manager.Categories, aiCategoryRule{Name: "No longer active", Essential: true})
	if e := validateAIProfile(in.Profile); e != nil {
		t.Fatal("stale named rule cannot be saved", e)
	}
	in.Profile.Manager.Categories = append(in.Profile.Manager.Categories, in.Profile.Manager.Categories[0])
	if e := validateAIProfile(in.Profile); e == nil {
		t.Fatal("duplicate saved rule accepted")
	}
	if e := validateAICurrent(nil, time.Now()); e != nil {
		t.Fatal("optional current data rejected")
	}
}
