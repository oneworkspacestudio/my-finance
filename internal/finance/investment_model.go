package finance

import (
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const paperMaxCash int64 = 1_000_000_000
const paperMaxPrice int64 = 100_000_000
const paperMaxUnits int64 = 1_000_000

type paperInstrument struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var paperInstruments = []paperInstrument{{"SIM-EQUITY", "Fictional equity basket"}, {"SIM-BOND", "Fictional bond basket"}, {"SIM-CASH", "Fictional cash basket"}}

type paperConfig struct {
	Initial      int64 `json:"initialCashMinor"`
	Target       int64 `json:"targetValueMinor"`
	MaxLoss      int64 `json:"maxLossMinor"`
	MaxOrder     int64 `json:"maxOrderMinor"`
	MaxPosition  int64 `json:"maxPositionMinor"`
	MaxDecisions int   `json:"maxDecisions"`
}
type paperPrice struct {
	Instrument string `json:"instrument"`
	Price      int64  `json:"priceMinor"`
}
type paperHolding struct {
	Instrument string `json:"instrument"`
	Units      int64  `json:"units"`
	Basis      int64  `json:"costBasisMinor"`
	Value      int64  `json:"valueMinor"`
}
type paperAction struct {
	Side       string `json:"side"`
	Instrument string `json:"instrument"`
	Units      int64  `json:"units"`
}
type paperDecision struct {
	Actions []paperAction `json:"actions"`
	Reason  string        `json:"reason"`
}
type paperRun struct {
	ID        string `json:"id"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Remaining int    `json:"remaining"`
	Expires   string `json:"expires"`
}
type paperAudit struct {
	At      string        `json:"at"`
	Kind    string        `json:"kind"`
	Detail  string        `json:"detail"`
	Cash    int64         `json:"cashMinor"`
	Equity  int64         `json:"equityMinor"`
	PnL     int64         `json:"pnlMinor"`
	Actions []paperAction `json:"actions"`
}
type paperState struct {
	ID       string         `json:"id"`
	Revision int64          `json:"revision"`
	Status   string         `json:"status"`
	Config   paperConfig    `json:"config"`
	Prices   []paperPrice   `json:"prices"`
	Cash     int64          `json:"cashMinor"`
	Equity   int64          `json:"equityMinor"`
	PnL      int64          `json:"pnlMinor"`
	Realized int64          `json:"realizedMinor"`
	Holdings []paperHolding `json:"holdings"`
	Attempts int            `json:"attempts"`
	Run      *paperRun      `json:"run"`
	Audit    []paperAudit   `json:"audit"`
}
type paperEnvelope struct {
	Version     int        `json:"version"`
	User        string     `json:"user"`
	SessionHash string     `json:"sessionHash"`
	State       paperState `json:"state"`
}

func validPaperConfig(c paperConfig) bool {
	return c.Initial >= 1 && c.Initial <= paperMaxCash && c.Target > c.Initial && c.Target <= 1_000_000_000_000 && c.MaxLoss >= 1 && c.MaxLoss <= c.Initial && c.MaxOrder >= 1 && c.MaxOrder <= c.Initial && c.MaxPosition >= 1 && c.MaxPosition <= c.Initial && c.MaxDecisions >= 1 && c.MaxDecisions <= 20
}
func validPaperPrices(prices []paperPrice, existing []paperPrice) bool {
	if len(prices) < 1 || len(prices) > len(paperInstruments) {
		return false
	}
	seen := map[string]bool{}
	for _, p := range prices {
		allowed := false
		for _, i := range paperInstruments {
			if p.Instrument == i.ID {
				allowed = true
			}
		}
		if !allowed || seen[p.Instrument] || p.Price < 1 || p.Price > paperMaxPrice {
			return false
		}
		seen[p.Instrument] = true
	}
	if existing != nil {
		if len(existing) != len(prices) {
			return false
		}
		for _, p := range existing {
			if !seen[p.Instrument] {
				return false
			}
		}
	}
	return true
}
func paperMark(s *paperState) error {
	if !validPaperConfig(s.Config) || !validPaperPrices(s.Prices, nil) || s.Cash < 0 || s.Cash > 3*paperMaxPrice*paperMaxUnits || len(s.Holdings) != len(s.Prices) || s.Attempts < 0 || s.Attempts > s.Config.MaxDecisions {
		return errors.New("Invalid virtual portfolio")
	}
	prices := map[string]int64{}
	for _, p := range s.Prices {
		prices[p.Instrument] = p.Price
	}
	var value, basis int64
	seen := map[string]bool{}
	for i := range s.Holdings {
		h := &s.Holdings[i]
		price, ok := prices[h.Instrument]
		if !ok || seen[h.Instrument] || h.Units < 0 || h.Units > paperMaxUnits || h.Basis < 0 || h.Basis > 3*paperMaxPrice*paperMaxUnits || h.Units == 0 && h.Basis != 0 {
			return errors.New("Invalid virtual holding")
		}
		seen[h.Instrument] = true
		h.Value = h.Units * price
		value += h.Value
		basis += h.Basis
	}
	// At-cost conservation is exact despite changing scenario prices. Realized
	// proceeds/basis carry the only permitted difference from initial virtual cash.
	if s.Cash+basis != s.Config.Initial+s.Realized {
		return errors.New("Virtual cash conservation failed")
	}
	s.Equity = s.Cash + value
	s.PnL = s.Equity - s.Config.Initial
	return nil
}
func paperStop(s *paperState) {
	if s.Equity >= s.Config.Target {
		s.Status = "goal_reached"
		s.Run = nil
	} else if s.PnL <= -s.Config.MaxLoss {
		s.Status = "loss_limit"
		s.Run = nil
	} else if s.Attempts >= s.Config.MaxDecisions {
		s.Status = "decision_limit"
		s.Run = nil
	}
}
func paperTerminal(status string) bool {
	return status == "goal_reached" || status == "loss_limit" || status == "decision_limit"
}
func paperRecord(s *paperState, kind, detail string, actions []paperAction, now time.Time) {
	// Audit is a bounded encrypted record, not an unbounded model transcript.
	if len(detail) > 600 {
		detail = detail[:600]
		for !utf8.ValidString(detail) {
			detail = detail[:len(detail)-1]
		}
	}
	if actions == nil {
		actions = []paperAction{}
	}
	s.Audit = append(s.Audit, paperAudit{now.UTC().Format(time.RFC3339Nano), kind, detail, s.Cash, s.Equity, s.PnL, append([]paperAction{}, actions...)})
	if len(s.Audit) > 50 {
		s.Audit = append([]paperAudit{}, s.Audit[len(s.Audit)-50:]...)
	}
}
func newPaperState(id string, revision int64, c paperConfig, prices []paperPrice, now time.Time) paperState {
	prices = append([]paperPrice{}, prices...)
	sort.Slice(prices, func(i, j int) bool { return prices[i].Instrument < prices[j].Instrument })
	s := paperState{ID: id, Revision: revision, Status: "ready", Config: c, Prices: prices, Cash: c.Initial, Equity: c.Initial, Holdings: []paperHolding{}, Audit: []paperAudit{}}
	for _, p := range prices {
		s.Holdings = append(s.Holdings, paperHolding{Instrument: p.Instrument})
	}
	paperRecord(&s, "created", "Created a fictional portfolio with virtual GBP cash. No real asset or money is involved.", nil, now)
	return s
}
func applyPaperDecision(original paperState, d paperDecision, now time.Time) (paperState, error) {
	if len(d.Actions) > 3 || !aiText(d.Reason, 800) {
		return original, errors.New("The model returned an invalid paper decision")
	}
	s := original
	s.Holdings = append([]paperHolding{}, original.Holdings...)
	s.Audit = append([]paperAudit{}, original.Audit...)
	if e := paperMark(&s); e != nil {
		return original, e
	}
	prices := map[string]int64{}
	for _, p := range s.Prices {
		prices[p.Instrument] = p.Price
	}
	used := map[string]bool{}
	for _, order := range d.Actions {
		price, allowed := prices[order.Instrument]
		if !allowed || used[order.Instrument] || (order.Side != "buy" && order.Side != "sell") || order.Units < 1 || order.Units > paperMaxUnits {
			return original, errors.New("The model proposed an unsupported or duplicate paper order")
		}
		used[order.Instrument] = true
		value := order.Units * price
		if value > s.Config.MaxOrder {
			return original, errors.New("A proposed paper order exceeds your order limit")
		}
		var holding *paperHolding
		for i := range s.Holdings {
			if s.Holdings[i].Instrument == order.Instrument {
				holding = &s.Holdings[i]
				break
			}
		}
		if holding == nil {
			return original, errors.New("The model referenced an unconfigured fictional instrument")
		}
		if order.Side == "buy" {
			if value > s.Cash || holding.Units+order.Units > paperMaxUnits || (holding.Units+order.Units)*price > s.Config.MaxPosition {
				return original, errors.New("A paper purchase exceeds virtual cash or your position limit")
			}
			s.Cash -= value
			holding.Units += order.Units
			holding.Basis += value
		} else {
			if order.Units > holding.Units {
				return original, errors.New("A paper sale exceeds units held; short selling is unavailable")
			}
			// Values are bounded: basis<=3e14, quantity<=1e6. Split quotient
			// before multiplying so prorated cost basis cannot overflow int64.
			removed := (holding.Basis/holding.Units)*order.Units + (holding.Basis%holding.Units)*order.Units/holding.Units
			holding.Basis -= removed
			holding.Units -= order.Units
			s.Cash += value
			s.Realized += value - removed
		}
	}
	if e := paperMark(&s); e != nil {
		return original, e
	}
	// A manual scenario-price move can lift an existing position over its cap.
	// Every buy is already checked above; holding or reducing existing exposure
	// remains possible even when the order cap requires several smaller sales.
	paperRecord(&s, "decision", strings.TrimSpace(d.Reason), d.Actions, now)
	return s, nil
}

func paperPreview(s paperState) map[string]any {
	return map[string]any{"simulation": true, "currency": "GBP", "config": s.Config, "prices": s.Prices, "holdings": s.Holdings, "cashMinor": s.Cash, "equityMinor": s.Equity, "pnlMinor": s.PnL, "remainingDecisions": s.Config.MaxDecisions - s.Attempts}
}
