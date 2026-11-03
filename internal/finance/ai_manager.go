package finance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"
)

type aiCurrent struct {
	Month      string       `json:"month"`
	AsOf       string       `json:"asOf"`
	Income     int64        `json:"incomeMinor"`
	Outgoing   int64        `json:"outgoingMinor"`
	Categories []aiCategory `json:"categories"`
}
type aiCategoryRule struct {
	Name      string `json:"name"`
	Essential bool   `json:"essential"`
	Target    *int64 `json:"targetMonthlyMinor"`
}

// Missing classification must never silently turn a category into optional
// spending, including when a model omits a field from its proposed rules.
func (r *aiCategoryRule) UnmarshalJSON(data []byte) error {
	var in struct {
		Name      string `json:"name"`
		Essential *bool  `json:"essential"`
		Target    *int64 `json:"targetMonthlyMinor"`
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e := d.Decode(&in); e != nil {
		return e
	}
	r.Name = in.Name
	r.Target = in.Target
	r.Essential = true
	if in.Essential != nil {
		r.Essential = *in.Essential
	}
	return nil
}

type aiManagerRules struct {
	Enabled    bool             `json:"enabled"`
	Savings    int64            `json:"savingsMonthlyMinor"`
	Investment int64            `json:"investmentMonthlyMinor"`
	CashBuffer int64            `json:"cashBufferMinor"`
	Categories []aiCategoryRule `json:"categories"`
}
type aiManagerCategory struct {
	Name       string `json:"name"`
	Essential  bool   `json:"essential"`
	Historical int64  `json:"historicalMonthlyMinor"`
	Wanted     int64  `json:"wantedMinor"`
	Limit      int64  `json:"limitMinor"`
	Spent      int64  `json:"spentMinor"`
	Remaining  int64  `json:"remainingMinor"`
	Overspent  int64  `json:"overspentMinor"`
	Reduction  int64  `json:"reductionMinor"`
}
type aiManagerPlan struct {
	PlanningIncome                int64               `json:"planningIncomeMinor"`
	CategoryAllocated             int64               `json:"categoryAllocatedMinor"`
	EssentialRequired             int64               `json:"essentialRequiredMinor"`
	EssentialAllocated            int64               `json:"essentialAllocatedMinor"`
	EssentialShortfall            int64               `json:"essentialShortfallMinor"`
	UnassignedEssentials          int64               `json:"unassignedEssentialsMinor"`
	UnassignedEssentialsAllocated int64               `json:"unassignedEssentialsAllocatedMinor"`
	DebtReserved                  int64               `json:"debtReservedMinor"`
	EmergencyReserved             int64               `json:"emergencyReservedMinor"`
	EmergencyTarget               int64               `json:"emergencyTargetMinor"`
	EmergencyGap                  int64               `json:"emergencyGapMinor"`
	EssentialMonthly              int64               `json:"essentialMonthlyMinor"`
	SavingsWanted                 int64               `json:"savingsWantedMinor"`
	SavingsReserved               int64               `json:"savingsReservedMinor"`
	GoalReserved                  int64               `json:"goalReservedMinor"`
	InvestmentWanted              int64               `json:"investmentWantedMinor"`
	InvestmentReserved            int64               `json:"investmentReservedMinor"`
	CashBufferWanted              int64               `json:"cashBufferWantedMinor"`
	CashBufferReserved            int64               `json:"cashBufferReservedMinor"`
	Unallocated                   int64               `json:"unallocatedMinor"`
	UnfundedTargets               int64               `json:"unfundedTargetsMinor"`
	Spent                         int64               `json:"spentMinor"`
	SpendingFundingGap            int64               `json:"spendingFundingGapMinor"`
	CurrentMonth                  string              `json:"currentMonth"`
	AsOf                          string              `json:"asOf"`
	CurrentAvailable              bool                `json:"currentAvailable"`
	InvestmentEligible            bool                `json:"investmentEligible"`
	Categories                    []aiManagerCategory `json:"categories"`
	Goals                         []aiGoalPlan        `json:"goals"`
	Warnings                      []string            `json:"warnings"`
}
type aiManagerSuggestion struct {
	Categories []aiCategoryRule `json:"categories"`
	Savings    int64            `json:"savingsMonthlyMinor"`
	Investment int64            `json:"investmentMonthlyMinor"`
	CashBuffer int64            `json:"cashBufferMinor"`
	Reason     string           `json:"reason"`
}

func aiCategoryKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// Settings validation deliberately does not require a live budget: rules for a
// category no longer in the selected period can be retained, but are ignored by
// the allocator until that category is present in the reviewed summary again.
func validateAIProfile(p aiProfile) error {
	if !aiMoney(p.Essentials) || !aiMoney(p.Emergency) || p.Horizon < 0 || p.Horizon > 80 || len(p.Goals) > 8 {
		return errors.New("Check your essential spending, savings and planning horizon")
	}
	if p.Debt != "yes" && p.Debt != "no" && p.Debt != "unknown" {
		return errors.New("Choose whether you have priority arrears or expensive debt")
	}
	if p.Risk != "unknown" && p.Risk != "cautious" && p.Risk != "balanced" && p.Risk != "adventurous" {
		return errors.New("Choose a risk preference or leave it unknown")
	}
	seen := map[string]bool{}
	for _, g := range p.Goals {
		key := aiCategoryKey(g.Name)
		if !aiText(g.Name, 80) || seen[key] || !aiMoney(g.Target) || !aiMoney(g.Saved) || g.Months < 1 || g.Months > 600 {
			return errors.New("Use distinct savings goal names, valid amounts and a one to six-hundred month deadline")
		}
		seen[key] = true
	}
	if p.Manager != nil {
		return validateAIManagerRules(*p.Manager)
	}
	return nil
}
func validateAIManagerRules(r aiManagerRules) error {
	if !aiMoney(r.Savings) || !aiMoney(r.Investment) || !aiMoney(r.CashBuffer) || len(r.Categories) > 200 {
		return errors.New("Check the manager's saving, investment, cash buffer and category limits")
	}
	seen := map[string]bool{}
	for _, c := range r.Categories {
		key := aiCategoryKey(c.Name)
		if !aiText(c.Name, 80) || seen[key] || c.Target != nil && !aiMoney(*c.Target) {
			return errors.New("Use distinct category names and non-negative category targets, or leave a target blank for its historical average")
		}
		seen[key] = true
	}
	return nil
}
func validateAICurrent(c *aiCurrent, now time.Time) error {
	if c == nil {
		return nil
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	asOf, e := time.Parse("2006-01-02", c.AsOf)
	if e != nil || c.Month != today.Format("2006-01") || asOf.Format("2006-01") != c.Month || asOf.After(today) {
		return errors.New("The current-month snapshot must have a valid date in this month, no later than today; refresh the budget")
	}
	if !aiMoney(c.Income) || !aiMoney(c.Outgoing) || len(c.Categories) > 200 {
		return errors.New("The current-month totals are outside the supported range")
	}
	var income, outgoing int64
	seen := map[string]bool{}
	for _, v := range c.Categories {
		key := aiCategoryKey(v.Name)
		if !aiText(v.Name, 80) || seen[key] || !aiMoney(v.Income) || !aiMoney(v.Outgoing) {
			return errors.New("Check current-month category names and amounts")
		}
		seen[key] = true
		income += v.Income
		outgoing += v.Outgoing
	}
	if income != c.Income || outgoing != c.Outgoing {
		return errors.New("Current-month category totals must match current-month income and outgoings")
	}
	return nil
}

// Allocate a bounded total by largest remainder. Names are sorted before this
// helper is called, so ties are stable. big.Int avoids overflowing when multiplying
// otherwise-valid integer pence; no floating arithmetic can invent a penny.
func aiProportion(weights []int64, budget int64) []int64 {
	out := make([]int64, len(weights))
	var total int64
	for _, v := range weights {
		total += v
	}
	budget = min(max(0, budget), total)
	if total == 0 || budget == 0 {
		return out
	}
	type remainder struct {
		index int
		value int64
	}
	order := make([]remainder, len(weights))
	var allocated int64
	for i, v := range weights {
		product := new(big.Int).Mul(big.NewInt(v), big.NewInt(budget))
		q, r := new(big.Int), new(big.Int)
		q.QuoRem(product, big.NewInt(total), r)
		out[i] = q.Int64()
		allocated += out[i]
		order[i] = remainder{i, r.Int64()}
	}
	sort.SliceStable(order, func(i, j int) bool { return order[i].value > order[j].value })
	for i := int64(0); i < budget-allocated; i++ {
		out[order[i].index]++
	}
	return out
}

func buildAIManager(in aiPlanInput, base aiPlan, now time.Time) (aiManagerPlan, error) {
	s, p := in.Summary, in.Profile
	r := *p.Manager
	out := aiManagerPlan{PlanningIncome: base.PlanningIncome, SavingsWanted: r.Savings, InvestmentWanted: r.Investment, CashBufferWanted: r.CashBuffer, CurrentMonth: now.Format("2006-01"), Categories: []aiManagerCategory{}, Goals: []aiGoalPlan{}, Warnings: []string{}}
	known := map[string]aiCategory{}
	history := map[string]int64{}
	current := map[string]int64{}
	for _, c := range s.Categories {
		key := aiCategoryKey(c.Name)
		known[key] = c
		history[key] = c.Outgoing
	}
	fresh := false
	if s.Current != nil {
		out.CurrentAvailable = true
		out.AsOf = s.Current.AsOf
		out.CurrentMonth = s.Current.Month
		out.Spent = s.Current.Outgoing
		asOf, _ := time.Parse("2006-01-02", s.Current.AsOf)
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		fresh = today.Sub(asOf) <= 7*24*time.Hour
		for _, c := range s.Current.Categories {
			key := aiCategoryKey(c.Name)
			if _, ok := known[key]; !ok {
				known[key] = c
			}
			current[key] = c.Outgoing
		}
	}
	if len(known) > 200 {
		return out, errors.New("This review has more than 200 historical and current categories; choose a shorter history")
	}
	keys := make([]string, 0, len(known))
	for key := range known {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rules := map[string]aiCategoryRule{}
	staleRules := 0
	for _, rule := range r.Categories {
		key := aiCategoryKey(rule.Name)
		if _, ok := known[key]; ok {
			rules[key] = rule
		} else {
			staleRules++
		}
	}
	if staleRules > 0 {
		out.Warnings = append(out.Warnings, "Saved rules for categories absent from this summary were retained but ignored. Review them when refreshing your budget.")
	}
	// Preserve the exact rounded historical total across category defaults.
	n := int64(len(s.Months))
	averages := make([]int64, len(keys))
	remainders := make([]int, len(keys))
	var rounded int64
	for i, key := range keys {
		averages[i] = history[key] / n
		rounded += averages[i]
		remainders[i] = i
	}
	sort.SliceStable(remainders, func(i, j int) bool { return history[keys[remainders[i]]]%n > history[keys[remainders[j]]]%n })
	for i := int64(0); i < base.AverageOutgoing-rounded; i++ {
		averages[remainders[i]]++
	}
	var essentialWanted int64
	for i, key := range keys {
		c := aiManagerCategory{Name: known[key].Name, Essential: true, Historical: averages[i], Wanted: averages[i], Spent: current[key]}
		if rule, ok := rules[key]; ok {
			c.Essential = rule.Essential
			if rule.Target != nil {
				c.Wanted = *rule.Target
			}
		}
		if c.Essential {
			essentialWanted += c.Wanted
		}
		out.Categories = append(out.Categories, c)
	}
	out.UnassignedEssentials = max(0, p.Essentials-essentialWanted)
	out.EssentialMonthly = max(base.Essentials, essentialWanted+out.UnassignedEssentials)
	out.EmergencyTarget = 3 * out.EssentialMonthly
	out.EmergencyGap = max(0, out.EmergencyTarget-p.Emergency)
	weights := make([]int64, len(out.Categories)+1)
	for i, c := range out.Categories {
		if c.Essential {
			weights[i] = max(c.Wanted, c.Spent)
		}
	}
	weights[len(out.Categories)] = out.UnassignedEssentials
	for _, v := range weights {
		out.EssentialRequired += v
	}
	allocated := aiProportion(weights, out.PlanningIncome)
	remaining := out.PlanningIncome
	for i := range out.Categories {
		out.Categories[i].Limit = allocated[i]
		remaining -= allocated[i]
		out.EssentialAllocated += allocated[i]
	}
	out.UnassignedEssentialsAllocated = allocated[len(out.Categories)]
	remaining -= out.UnassignedEssentialsAllocated
	out.EssentialAllocated += out.UnassignedEssentialsAllocated
	out.EssentialShortfall = out.EssentialRequired - out.EssentialAllocated
	// Already-spent flexible money is committed. It must reduce future savings
	// before the manager can distribute any additional discretionary allowance.
	weights = make([]int64, len(out.Categories))
	for i, c := range out.Categories {
		if !c.Essential {
			weights[i] = c.Spent
		}
	}
	allocated = aiProportion(weights, remaining)
	for i, n := range allocated {
		out.Categories[i].Limit += n
		remaining -= n
	}
	for _, c := range out.Categories {
		out.SpendingFundingGap += max(0, c.Spent-c.Limit)
	}
	reserve := func(wanted int64) int64 { amount := min(max(0, wanted), remaining); remaining -= amount; return amount }
	if p.Debt != "no" {
		out.DebtReserved = reserve(remaining)
		out.Warnings = append(out.Warnings, "Money after protected costs is held for debt review until priority arrears and expensive borrowing are understood. No repayment is made.")
	} else {
		out.EmergencyReserved = reserve(out.EmergencyGap)
		out.SavingsReserved = reserve(r.Savings)
	}
	var goalWanted, goalUnfunded int64
	for _, g := range p.Goals {
		gp := aiGoalPlan{Name: g.Name, Remaining: max(0, g.Target-g.Saved)}
		gp.Required = (gp.Remaining + g.Months - 1) / g.Months
		goalWanted += gp.Required
		if p.Debt == "no" {
			gp.Allocated = reserve(gp.Required)
		}
		out.GoalReserved += gp.Allocated
		goalUnfunded += gp.Required - gp.Allocated
		if gp.Remaining == 0 {
			zero := int64(0)
			gp.MonthsToGoal = &zero
		} else if gp.Allocated > 0 {
			months := (gp.Remaining + gp.Allocated - 1) / gp.Allocated
			gp.MonthsToGoal = &months
		}
		out.Goals = append(out.Goals, gp)
	}
	last := s.Months[len(s.Months)-1].Month
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	out.InvestmentEligible = s.Source == "actual" && fresh && len(s.Months) >= 3 && last == thisMonth.AddDate(0, -1, 0).Format("2006-01") && p.Debt == "no" && out.EmergencyGap == 0 && out.EssentialMonthly > 0 && p.Horizon >= 5 && p.Risk != "unknown" && out.EssentialShortfall == 0 && out.SpendingFundingGap == 0 && goalUnfunded == 0 && out.SavingsReserved == r.Savings && remaining > r.CashBuffer
	if out.InvestmentEligible {
		out.InvestmentReserved = reserve(min(r.Investment, max(0, remaining-r.CashBuffer)))
	}
	out.CashBufferReserved = reserve(r.CashBuffer)
	weights = make([]int64, len(out.Categories))
	for i, c := range out.Categories {
		if !c.Essential {
			weights[i] = max(0, c.Wanted-c.Limit)
		}
	}
	allocated = aiProportion(weights, remaining)
	for i, n := range allocated {
		out.Categories[i].Limit += n
		remaining -= n
	}
	out.Unallocated = remaining
	var categoryUnfunded int64
	for i := range out.Categories {
		c := &out.Categories[i]
		c.Remaining = max(0, c.Limit-c.Spent)
		c.Overspent = max(0, c.Spent-min(c.Wanted, c.Limit))
		c.Reduction = max(0, c.Wanted-c.Limit)
		categoryUnfunded += c.Reduction
		out.CategoryAllocated += c.Limit
	}
	out.UnfundedTargets = categoryUnfunded + (out.UnassignedEssentials - out.UnassignedEssentialsAllocated) + (r.Savings - out.SavingsReserved) + (goalWanted - out.GoalReserved) + (r.Investment - out.InvestmentReserved) + (r.CashBuffer - out.CashBufferReserved)
	out.Warnings = append(out.Warnings, "This is a monthly income forecast, not your bank balance or permission to spend. Only budget cash you already have and check upcoming bills.", "Savings and investment amounts are planning reserves only. Actual categories, bank accounts, transfers and trades are unchanged.", "Category overspending compares spending with the lower of its requested target and funded allowance; already-spent money reduces what can be reserved next.")
	if !out.CurrentAvailable {
		out.Warnings = append(out.Warnings, "No current-month snapshot is available. Remaining amounts are forecast allowances, not tracked spending or a spendable balance.")
	} else if !fresh {
		out.Warnings = append(out.Warnings, "Current spending is more than seven days old. Refresh your budget; investment reserves are disabled until the current snapshot is fresh.")
	}
	if s.Source == "manual" {
		out.Warnings = append(out.Warnings, "Manual totals are estimates, not verified category spending. Investment reserves are disabled; review the figures before making commitments.")
	}
	if len(s.Months) < 3 || last != thisMonth.AddDate(0, -1, 0).Format("2006-01") {
		out.Warnings = append(out.Warnings, "The historical period is short or not up to date. Refresh at least three complete recent months before considering an investment reserve.")
	}
	if out.EssentialShortfall > 0 {
		out.Warnings = append(out.Warnings, "Conservative income cannot cover the protected essential costs. Review the shortfall before adding savings or investments.")
	}
	if out.SpendingFundingGap > 0 {
		out.Warnings = append(out.Warnings, "Recorded spending already exceeds part of the funded plan. This does not erase that spending or create cash to cover it.")
	}
	if r.Investment > 0 && !out.InvestmentEligible {
		out.Warnings = append(out.Warnings, "The requested investment reserve is held back. Confirm fresh actual spending, funded essentials and savings goals, no expensive debt, an existing emergency buffer and a suitable long-term horizon.")
	}
	if out.UnfundedTargets > 0 {
		out.Warnings = append(out.Warnings, "Some requested category limits or savings, investment and cash-buffer targets are not funded. Review the reductions and priorities.")
	}
	return out, nil
}

func applyAIManagerPlan(out *aiPlan, m aiManagerPlan) {
	out.Essentials = m.EssentialMonthly
	out.EmergencyTarget = m.EmergencyTarget
	out.EmergencyGap = m.EmergencyGap
	out.SpendingReserve = m.CategoryAllocated + m.UnassignedEssentialsAllocated
	out.Surplus = max(0, m.PlanningIncome-out.SpendingReserve)
	var flexibleSpent int64
	for _, c := range m.Categories {
		if !c.Essential {
			flexibleSpent += c.Spent
		}
	}
	out.Deficit = max(0, m.EssentialRequired+flexibleSpent-m.PlanningIncome)
	out.Goals = m.Goals
	out.Warnings = m.Warnings
	out.InvestmentReady = m.InvestmentEligible
	out.InvestmentReason = "Investment reserves stay at zero until the manager has fresh Actual spending, protected costs, funded saving goals, an existing emergency buffer, no expensive debt and a suitable long-term horizon."
	if m.InvestmentEligible {
		out.InvestmentReason = "Your answers pass a basic discussion checklist. Any investment reserve is a user-requested forecast amount, not a suitability assessment or trade. Review UK account types, diversification, fees and loss capacity with a regulated adviser where needed."
	}
	out.Buckets = []aiBucket{{"Managed category allowances", out.SpendingReserve, "Funded category limits and any additional essential-cost reserve; review the manager's category table for spending and reductions."}}
	for _, b := range []aiBucket{{"Debt review reserve", m.DebtReserved, "Held while expensive borrowing or priority arrears are reviewed; no payment is made."}, {"Emergency savings", m.EmergencyReserved, "Accessible cash-buffer planning reserve."}, {"Extra savings", m.SavingsReserved, "General savings in addition to the named goals below."}} {
		if b.Monthly > 0 {
			out.Buckets = append(out.Buckets, b)
		}
	}
	for _, g := range m.Goals {
		if g.Allocated > 0 {
			out.Buckets = append(out.Buckets, aiBucket{g.Name, g.Allocated, "Named savings goal contribution after protected costs and higher-priority reserves."})
		}
	}
	for _, b := range []aiBucket{{"Investment planning reserve", m.InvestmentReserved, "Your requested long-term reserve only; no investment, transfer or security selection occurs."}, {"Cash buffer", m.CashBufferReserved, "Unspent cash cushion requested in your manager rules."}, {"Unallocated", m.Unallocated, "Forecast income left without a commitment; this is not a bank balance."}} {
		if b.Monthly > 0 {
			out.Buckets = append(out.Buckets, b)
		}
	}
}

func validateAIManagerSuggestion(s *aiManagerSuggestion, summary aiSummary) error {
	if s == nil {
		return nil
	}
	if !aiText(s.Reason, 2000) {
		return errors.New("The AI manager suggestion has an invalid explanation")
	}
	if e := validateAIManagerRules(aiManagerRules{Savings: s.Savings, Investment: s.Investment, CashBuffer: s.CashBuffer, Categories: s.Categories}); e != nil {
		return errors.New("The AI manager suggestion contains invalid limits")
	}
	known := map[string]string{}
	for _, c := range summary.Categories {
		known[aiCategoryKey(c.Name)] = c.Name
	}
	if summary.Current != nil {
		for _, c := range summary.Current.Categories {
			known[aiCategoryKey(c.Name)] = c.Name
		}
	}
	for i, c := range s.Categories {
		canonical, ok := known[aiCategoryKey(c.Name)]
		if !ok {
			return fmt.Errorf("The AI manager suggestion referenced a category outside the reviewed summary")
		}
		s.Categories[i].Name = canonical
	}
	return nil
}
