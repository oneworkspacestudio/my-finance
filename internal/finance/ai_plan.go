package finance

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Money crosses the assistant boundary only as integer GBP pence. Actual
// remains the ledger: these are user-reviewed aggregates, never instructions.
const aiMaxMoney int64 = 100_000_000_000

type aiMonth struct {
	Month    string `json:"month"`
	Income   int64  `json:"incomeMinor"`
	Outgoing int64  `json:"outgoingMinor"`
}
type aiCategory struct {
	Name     string `json:"name"`
	Income   int64  `json:"incomeMinor"`
	Outgoing int64  `json:"outgoingMinor"`
}
type aiSummary struct {
	Source            string       `json:"source"`
	Currency          string       `json:"currency"`
	Months            []aiMonth    `json:"months"`
	Categories        []aiCategory `json:"categories"`
	TransactionCount  int          `json:"transactionCount"`
	ExcludedTransfers int          `json:"excludedTransfers"`
	Current           *aiCurrent   `json:"current,omitempty"`
}
type aiGoal struct {
	Name   string `json:"name"`
	Target int64  `json:"targetMinor"`
	Saved  int64  `json:"savedMinor"`
	Months int64  `json:"months"`
}
type aiProfile struct {
	Essentials int64           `json:"essentialMonthlyMinor"`
	Emergency  int64           `json:"emergencySavingsMinor"`
	Debt       string          `json:"highCostDebt"`
	Horizon    int             `json:"horizonYears"`
	Risk       string          `json:"risk"`
	Goals      []aiGoal        `json:"goals"`
	Manager    *aiManagerRules `json:"manager,omitempty"`
}
type aiPlanInput struct {
	Summary aiSummary `json:"summary"`
	Profile aiProfile `json:"profile"`
}
type aiBucket struct {
	Name    string `json:"name"`
	Monthly int64  `json:"monthlyMinor"`
	Purpose string `json:"purpose"`
}
type aiGoalPlan struct {
	Name         string `json:"name"`
	Remaining    int64  `json:"remainingMinor"`
	Required     int64  `json:"requiredMonthlyMinor"`
	Allocated    int64  `json:"allocatedMonthlyMinor"`
	MonthsToGoal *int64 `json:"monthsToGoal"`
}
type aiPlan struct {
	Currency         string         `json:"currency"`
	AverageIncome    int64          `json:"averageIncomeMinor"`
	AverageOutgoing  int64          `json:"averageOutgoingMinor"`
	SpendingReserve  int64          `json:"spendingReserveMinor"`
	PlanningIncome   int64          `json:"planningIncomeMinor"`
	Surplus          int64          `json:"surplusMinor"`
	Deficit          int64          `json:"deficitMinor"`
	Essentials       int64          `json:"essentialMonthlyMinor"`
	EmergencyTarget  int64          `json:"emergencyTargetMinor"`
	EmergencyGap     int64          `json:"emergencyGapMinor"`
	Buckets          []aiBucket     `json:"buckets"`
	Goals            []aiGoalPlan   `json:"goals"`
	InvestmentReady  bool           `json:"investmentReady"`
	InvestmentReason string         `json:"investmentReason"`
	Warnings         []string       `json:"warnings"`
	Manager          *aiManagerPlan `json:"manager,omitempty"`
}

func aiText(s string, max int) bool {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max {
		return false
	}
	for _, c := range s {
		if unicode.IsControl(c) {
			return false
		}
	}
	return strings.TrimSpace(s) != ""
}
func aiMoney(n int64) bool { return n >= 0 && n <= aiMaxMoney }
func buildAIPlan(in aiPlanInput, now time.Time) (aiPlan, error) {
	s, p := in.Summary, in.Profile
	out := aiPlan{Currency: "GBP", Buckets: []aiBucket{}, Goals: []aiGoalPlan{}, Warnings: []string{}}
	bad := func(s string) (aiPlan, error) { return aiPlan{}, errors.New(s) }
	if s.Currency != "GBP" || len(s.Months) < 1 || len(s.Months) > 12 {
		return bad("Choose one to twelve complete months from a GBP budget")
	}
	if s.Source != "actual" && s.Source != "manual" {
		return bad("Choose an Actual budget summary or clearly labelled manual estimates")
	}
	if len(s.Categories) > 200 || s.TransactionCount < 0 || s.Source == "actual" && s.TransactionCount == 0 || s.TransactionCount > 1_000_000 || s.ExcludedTransfers < 0 || s.ExcludedTransfers > 1_000_000 {
		return bad("Choose a period with at least one transaction in the complete-month history, within the supported size limits, or use manual estimates")
	}
	var totalIn, totalOut int64
	lowest := aiMaxMoney
	var previous time.Time
	currentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for _, m := range s.Months {
		d, e := time.Parse("2006-01", m.Month)
		if e != nil || !d.Before(currentMonth) || d.Year() < 1900 || (!previous.IsZero() && !d.Equal(previous.AddDate(0, 1, 0))) {
			return bad("Use consecutive complete months, oldest first; current and future months are excluded")
		}
		if !aiMoney(m.Income) || !aiMoney(m.Outgoing) {
			return bad("Monthly totals must be non-negative GBP pence within the supported range")
		}
		previous = d
		totalIn += m.Income
		totalOut += m.Outgoing
		lowest = min(lowest, m.Income)
	}
	var categoryIn, categoryOut int64
	seen := map[string]bool{}
	for _, c := range s.Categories {
		name := strings.ToLower(strings.TrimSpace(c.Name))
		if !aiText(c.Name, 80) || seen[name] || !aiMoney(c.Income) || !aiMoney(c.Outgoing) {
			return bad("Check category names and totals; combine duplicate category labels")
		}
		seen[name] = true
		categoryIn += c.Income
		categoryOut += c.Outgoing
	}
	if categoryIn != totalIn || categoryOut != totalOut {
		return bad("Category totals must match the monthly income and outgoing totals")
	}
	if e := validateAIProfile(p); e != nil {
		return bad(e.Error())
	}
	if e := validateAICurrent(s.Current, now); e != nil {
		return bad(e.Error())
	}
	n := int64(len(s.Months))
	out.AverageIncome = totalIn / n
	// Round spending upwards and income downwards so rounding never overallocates.
	out.AverageOutgoing = (totalOut + n - 1) / n
	out.PlanningIncome = min(out.AverageIncome, lowest)
	out.Essentials = p.Essentials
	if out.Essentials == 0 {
		out.Essentials = out.AverageOutgoing
		out.Warnings = append(out.Warnings, "Essential spending was not supplied; all observed outgoings are used for the emergency-fund estimate.")
	}
	out.EmergencyTarget = 3 * out.Essentials
	out.EmergencyGap = max(0, out.EmergencyTarget-p.Emergency)
	out.SpendingReserve = max(out.AverageOutgoing, out.Essentials)
	out.Surplus = max(0, out.PlanningIncome-out.SpendingReserve)
	out.Deficit = max(0, out.SpendingReserve-out.PlanningIncome)
	if out.Essentials > out.AverageOutgoing {
		out.Warnings = append(out.Warnings, "Your essential spending estimate exceeds the historical average. The plan reserves the higher estimate before calculating surplus; review whether other variable costs need adding too.")
	}
	out.Warnings = append(out.Warnings, "This is a draft based on complete-month cash flows. Check refunds, irregular income, missing accounts and upcoming bills before using it.", "The planning income uses the lowest month in this period. Allocations are estimates, not money available to spend today.", "Buckets are suggestions for Actual categories. No money moves and no budget values are changed.")
	if s.Source == "manual" {
		out.Warnings = append(out.Warnings, "These are manual estimates, not verified transactions. Category-level spending analysis and investment readiness cannot be established from this input.")
	}
	if len(s.Months) < 3 {
		out.Warnings = append(out.Warnings, "Fewer than three complete months are available; this may not represent your usual finances.")
	}
	if previous.Before(currentMonth.AddDate(0, -1, 0)) {
		out.Warnings = append(out.Warnings, "The selected period does not include the most recent complete month. Refresh the summary before relying on it.")
	}
	if out.Deficit > 0 {
		out.Warnings = append(out.Warnings, "The spending reserve exceeds the conservative planning income. Review essential bills and debt support before adding new savings commitments.")
	}
	// Spending is a factual reserve, even in a deficit: never disguise overspending
	// by presenting an artificially balanced or affordable bucket allocation.
	out.Buckets = append(out.Buckets, aiBucket{"Spending reserve", out.SpendingReserve, "The higher of observed average outgoings and your essential-cost estimate, including current debt payments; review categories and upcoming bills."})
	remaining := out.Surplus
	if p.Debt == "yes" {
		if remaining > 0 {
			out.Buckets = append(out.Buckets, aiBucket{"Debt review reserve", remaining, "Review priority arrears and expensive borrowing before committing this reserve; confirm repayment terms and keep essential bills covered."})
			remaining = 0
		}
		out.Warnings = append(out.Warnings, "Debt balances, rates and minimum payments are not known. A repayment order cannot be calculated from transactions alone.")
	} else if p.Debt == "unknown" {
		out.Warnings = append(out.Warnings, "Confirm priority arrears and expensive debt before allocating the surplus to savings or long-term goals.")
	} else if out.EmergencyGap > 0 {
		allocation := min(remaining, out.EmergencyGap)
		if allocation > 0 {
			out.Buckets = append(out.Buckets, aiBucket{"Emergency savings", allocation, "Build an accessible cash buffer; the initial target is three months of essential costs, not an investment."})
			remaining -= allocation
		}
	}
	for _, g := range p.Goals {
		gp := aiGoalPlan{Name: g.Name, Remaining: max(0, g.Target-g.Saved)}
		gp.Required = (gp.Remaining + g.Months - 1) / g.Months
		if p.Debt == "no" {
			gp.Allocated = min(remaining, gp.Required)
			remaining -= gp.Allocated
		}
		if gp.Remaining == 0 {
			zero := int64(0)
			gp.MonthsToGoal = &zero
		} else if gp.Allocated > 0 {
			months := (gp.Remaining + gp.Allocated - 1) / gp.Allocated
			gp.MonthsToGoal = &months
		}
		if gp.Allocated > 0 {
			out.Buckets = append(out.Buckets, aiBucket{g.Name, gp.Allocated, "Savings goal allocation in your chosen priority order, after the emergency reserve."})
		}
		if gp.Allocated < gp.Required {
			out.Warnings = append(out.Warnings, fmt.Sprintf("The goal %q needs more per month than this draft allocates. Extend its deadline, reduce its target, or review spending.", g.Name))
		}
		out.Goals = append(out.Goals, gp)
	}
	out.InvestmentReady = s.Source == "actual" && p.Debt == "no" && out.EmergencyGap == 0 && out.Essentials > 0 && p.Horizon >= 5 && p.Risk != "unknown" && out.Deficit == 0 && remaining > 0 && len(s.Months) >= 3 && previous.Equal(currentMonth.AddDate(0, -1, 0))
	out.InvestmentReason = "Keep the focus on cash-flow stability, debt, accessible emergency savings and near-term goals. Confirm a five-year or longer horizon and your ability to bear losses before considering investments."
	if out.InvestmentReady {
		out.InvestmentReason = "Your answers meet this basic discussion checklist. This does not establish investment suitability: review diversification, costs, loss capacity and tax circumstances with a regulated adviser before choosing products."
	}
	if remaining > 0 {
		out.Buckets = append(out.Buckets, aiBucket{"Uncommitted buffer", remaining, "Keep available for irregular bills and decisions you have not yet made. It is not automatically allocated to investments."})
	}
	if p.Manager != nil && p.Manager.Enabled {
		manager, e := buildAIManager(in, out, now)
		if e != nil {
			return bad(e.Error())
		}
		out.Manager = &manager
		applyAIManagerPlan(&out, manager)
	}
	return out, nil
}
