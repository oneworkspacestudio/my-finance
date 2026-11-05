package finance

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
)

type cryptoRuntime struct {
	mu       sync.Mutex
	client   *http.Client
	inflight map[string]cryptoFlight
}
type cryptoFlight struct {
	revision int64
	cancel   context.CancelFunc
}
type cryptoAttempt struct {
	At             string `json:"at"`
	ReservedMicros int64  `json:"reservedMicros"`
	Outcome        string `json:"outcome"`
}
type cryptoEnvelope struct {
	Version     int             `json:"version"`
	User        string          `json:"user"`
	KeyID       string          `json:"keyId"`
	ConsentedAt string          `json:"consentedAt"`
	State       cryptoState     `json:"state"`
	Attempts    []cryptoAttempt `json:"attempts"`
}

func (a *App) cryptoRoutes() error {
	if e := a.migrate(7, `CREATE TABLE finance_crypto_monitors(user_id TEXT PRIMARY KEY REFERENCES host_users(id) ON DELETE CASCADE,revision INTEGER NOT NULL,enabled INTEGER NOT NULL,next_run TEXT NOT NULL,secret BLOB NOT NULL);`); e != nil {
		return e
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	a.crypto = &cryptoRuntime{client: &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, inflight: map[string]cryptoFlight{}}
	// An interrupted request stays counted. Schedule a future check, never replay
	// or catch up an unconfirmed paid request after restart.
	rows, e := a.h.DB.Query(`SELECT user_id FROM finance_crypto_monitors WHERE enabled=1`)
	if e != nil {
		return e
	}
	var users []string
	for rows.Next() {
		var user string
		if e = rows.Scan(&user); e != nil {
			rows.Close()
			return e
		}
		users = append(users, user)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, user := range users {
		v, e := a.cryptoLoad(user)
		if e != nil {
			continue
		}
		old := v.State.Revision
		v.State.Revision++
		cryptoExpire(&v.State, time.Now(), true)
		v.State.Status = "scheduled"
		v.State.NextRun = time.Now().UTC().Add(time.Duration(v.State.Config.IntervalMinutes) * time.Minute).Format(time.RFC3339Nano)
		if oldStatus := v.Attempts; len(oldStatus) > 0 && oldStatus[len(oldStatus)-1].Outcome == "reserved" {
			v.Attempts[len(v.Attempts)-1].Outcome = "interrupted"
			v.State.LastError = "A previous interrupted check remains counted; it was not retried"
		}
		if e = a.cryptoSave(v, old); e != nil {
			return e
		}
	}
	a.h.Mux.HandleFunc("GET /api/finance/crypto", a.h.Require(a.cryptoGet))
	a.h.Mux.HandleFunc("POST /api/finance/crypto/settings", a.h.Require(a.cryptoSettings))
	a.h.Mux.HandleFunc("POST /api/finance/crypto/pause", a.h.Require(a.cryptoPause))
	a.h.Mux.HandleFunc("POST /api/finance/crypto/check", a.h.Require(a.cryptoCheck))
	a.h.Mux.HandleFunc("POST /api/finance/crypto/dismiss", a.h.Require(a.cryptoDismiss))
	return nil
}

func (a *App) cryptoLoad(user string) (cryptoEnvelope, error) {
	var v cryptoEnvelope
	var secret []byte
	var revision int64
	if e := a.h.DB.QueryRow(`SELECT revision,secret FROM finance_crypto_monitors WHERE user_id=?`, user).Scan(&revision, &secret); e != nil {
		return v, e
	}
	if len(secret) > 256<<10 {
		return v, errors.New("Invalid protected monitor record")
	}
	plain, e := openSession(secret)
	if e != nil {
		return v, errors.New("Windows cannot open this monitor; restore compatible recovery")
	}
	defer clear(plain)
	if len(plain) > 128<<10 || json.Unmarshal(plain, &v) != nil || v.Version != 1 || v.User != user || v.State.Revision != revision || revision < 1 || revision > 1000000000 || len(v.State.Proposals) > 30 || len(v.State.Evidence) > 7 || len(v.Attempts) > 32 {
		return v, errors.New("Invalid protected monitor record")
	}
	if e = cryptoConfigValid(v.State.Config); e != nil {
		return v, e
	}
	if !cryptoUsageValid(v) {
		return v, errors.New("Invalid protected monitor usage record")
	}
	return v, nil
}

func cryptoUsageValid(v cryptoEnvelope) bool {
	u := v.State.Usage
	day, e := time.Parse("2006-01-02", u.Day)
	if e != nil || day.Format("2006-01-02") != u.Day || u.Analyses < 0 || u.Analyses > 96 {
		return false
	}
	reserved, e := cryptoBoundedDecimal(u.ReservedSpendUSD, 6, "0", "10")
	if e != nil || reserved.Cmp(big.NewRat(int64(u.Analyses)*cryptoReservationMicros, 1000000)) != 0 {
		return false
	}
	for _, attempt := range v.Attempts {
		if _, e := time.Parse(time.RFC3339Nano, attempt.At); e != nil || attempt.ReservedMicros != cryptoReservationMicros {
			return false
		}
		switch attempt.Outcome {
		case "reserved", "interrupted", "failed", "completed":
		default:
			return false
		}
	}
	return true
}

func (a *App) cryptoSave(v cryptoEnvelope, previous int64) error {
	if v.Version != 1 || v.User == "" || v.State.Revision < 1 || v.State.Revision > 1000000000 || len(v.State.Proposals) > 30 || len(v.State.Evidence) > 7 || len(v.Attempts) > 32 || !cryptoUsageValid(v) {
		return errors.New("Invalid monitor storage bounds")
	}
	plain, e := json.Marshal(v)
	if e != nil || len(plain) > 128<<10 {
		return errors.New("Monitor history reached its storage limit")
	}
	secret, e := protectManagerProfile(plain)
	clear(plain)
	if e != nil {
		return errors.New("Windows could not protect the monitor")
	}
	result, e := a.h.DB.Exec(`INSERT INTO finance_crypto_monitors(user_id,revision,enabled,next_run,secret)VALUES(?,?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET revision=excluded.revision,enabled=excluded.enabled,next_run=excluded.next_run,secret=excluded.secret WHERE finance_crypto_monitors.revision=?`, v.User, v.State.Revision, v.State.Config.Enabled, v.State.NextRun, secret, previous)
	if e != nil {
		return errors.New("The monitor could not be saved")
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return errors.New("The monitor changed; refresh before trying again")
	}
	return nil
}

func (a *App) cryptoKeyID(user string) (string, error) {
	var model string
	var secret []byte
	if e := a.h.DB.QueryRow(`SELECT model,secret FROM finance_ai_keys WHERE user_id=? AND provider='deepseek'`, user).Scan(&model, &secret); e != nil {
		return "", errors.New("Add a DeepSeek key in AI settings")
	}
	if model != "deepseek-flash" {
		return "", errors.New("Choose deepseek-flash in AI settings for the verified monitor cost estimate")
	}
	h := sha256.New()
	h.Write([]byte(model))
	h.Write(secret)
	return hex.EncodeToString(h.Sum(nil)), nil
}
func (a *App) cryptoActive(user string) bool {
	var role string
	return a.h.DB.QueryRow(`SELECT role FROM host_users WHERE id=?`, user).Scan(&role) == nil && role != "disabled"
}
func cryptoUsageRefresh(s *cryptoState, now time.Time) {
	day := now.UTC().Format("2006-01-02")
	if s.Usage.Day == "" || day > s.Usage.Day {
		s.Usage.Day = day
		s.Usage.Analyses = 0
		s.Usage.ReservedSpendUSD = "0.000000"
	}
	s.Usage.MaxSpendUSD = s.Config.MaxSpendUSD
	s.Usage.RemainingAnalyses = max(0, s.Config.MaxAnalysesPerDay-s.Usage.Analyses)
}
func cryptoExpire(s *cryptoState, now time.Time, all bool) {
	for i := range s.Proposals {
		p := &s.Proposals[i]
		at, e := time.Parse(time.RFC3339Nano, p.ExpiresAt)
		if p.Status == "pending" && (all || e != nil || !now.Before(at)) {
			p.Status = "expired"
		}
	}
}
func cryptoPending(s cryptoState, now time.Time) bool {
	for _, p := range s.Proposals {
		at, e := time.Parse(time.RFC3339Nano, p.ExpiresAt)
		if p.Status == "pending" && e == nil && now.Before(at) {
			return true
		}
	}
	return false
}

const cryptoReceiptWaitMessage = "Check your submitted or unknown swap receipt before another paid analysis. Monitoring waits for that receipt even if you change wallets."

func cryptoAwaitingReceipt(s cryptoState) bool {
	for _, p := range s.Proposals {
		if p.Status == "submitted" || p.Status == "unknown" {
			return true
		}
	}
	return false
}
func (a *App) cryptoCancelLocked(user string) {
	if f, ok := a.crypto.inflight[user]; ok {
		f.cancel()
	}
}
func cryptoReply(w http.ResponseWriter, v *cryptoEnvelope, key bool) {
	w.Header().Set("Cache-Control", "no-store")
	var state *cryptoState
	if v != nil {
		s := v.State
		cryptoExpire(&s, time.Now(), false)
		cryptoUsageRefresh(&s, time.Now())
		state = &s
	}
	apphost.JSON(w, 200, map[string]any{"configured": v != nil, "state": state, "deepseekConfigured": key})
}
func (a *App) cryptoGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user := apphost.Current(r).User.ID
	a.crypto.mu.Lock()
	defer a.crypto.mu.Unlock()
	v, e := a.cryptoLoad(user)
	_, ke := a.cryptoKeyID(user)
	if errors.Is(e, sql.ErrNoRows) {
		cryptoReply(w, nil, ke == nil)
		return
	}
	if e != nil {
		apphost.Problem(w, 503, "The protected monitor could not be opened")
		return
	}
	cryptoReply(w, &v, ke == nil)
}

func (a *App) cryptoSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var in struct {
		Revision int64        `json:"revision"`
		Consent  bool         `json:"consent"`
		Config   cryptoConfig `json:"config"`
	}
	if !aiDecode(w, r, &in) {
		return
	}
	in.Config.WalletAddress = strings.ToLower(in.Config.WalletAddress)
	in.Config.Objective = strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(in.Config.Objective))
	if e := cryptoConfigValid(in.Config); e != nil {
		apphost.Problem(w, 400, e.Error())
		return
	}
	user := apphost.Current(r).User.ID
	a.crypto.mu.Lock()
	defer a.crypto.mu.Unlock()
	v, e := a.cryptoLoad(user)
	if errors.Is(e, sql.ErrNoRows) {
		v = cryptoEnvelope{Version: 1, User: user, State: cryptoState{Evidence: []cryptoEvidence{}, Proposals: []cryptoProposal{}, LastRisks: []string{}}, Attempts: []cryptoAttempt{}}
	} else if e != nil {
		apphost.Problem(w, 503, "The protected monitor could not be opened")
		return
	}
	if in.Revision != v.State.Revision {
		apphost.Problem(w, 412, "The monitor changed; refresh before saving")
		return
	}
	keyID, ke := a.cryptoKeyID(user)
	if in.Config.Enabled {
		if !in.Consent {
			apphost.Problem(w, 400, "Allow background monitoring, public evidence sharing and bounded provider charges")
			return
		}
		if ke != nil {
			apphost.Problem(w, 400, ke.Error())
			return
		}
		if _, _, e = a.aiCredential(user, "deepseek"); e != nil {
			apphost.Problem(w, 400, e.Error())
			return
		}
		v.KeyID = keyID
		v.ConsentedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	old := v.State.Revision
	v.State.Revision++
	v.State.Config = in.Config
	cryptoExpire(&v.State, time.Now(), true)
	v.State.LastError = ""
	v.State.Status = "paused"
	v.State.NextRun = ""
	if in.Config.Enabled {
		v.State.Status = "scheduled"
		v.State.NextRun = time.Now().UTC().Format(time.RFC3339Nano)
	}
	cryptoUsageRefresh(&v.State, time.Now())
	if e = a.cryptoSave(v, old); e != nil {
		apphost.Problem(w, 503, e.Error())
		return
	}
	a.cryptoCancelLocked(user)
	cryptoReply(w, &v, ke == nil)
}

func (a *App) cryptoPause(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var in struct {
		Revision int64 `json:"revision"`
	}
	if !aiDecode(w, r, &in) {
		return
	}
	user := apphost.Current(r).User.ID
	a.crypto.mu.Lock()
	defer a.crypto.mu.Unlock()
	v, e := a.cryptoLoad(user)
	if e != nil {
		apphost.Problem(w, 400, "No configured monitor is available")
		return
	}
	old := v.State.Revision
	v.State.Revision++
	v.State.Config.Enabled = false
	v.State.Status = "paused"
	v.State.NextRun = ""
	cryptoExpire(&v.State, time.Now(), true)
	if e = a.cryptoSave(v, old); e != nil {
		apphost.Problem(w, 503, e.Error())
		return
	}
	a.cryptoCancelLocked(user)
	_, ke := a.cryptoKeyID(user)
	cryptoReply(w, &v, ke == nil)
}

func (a *App) cryptoCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var in struct {
		Revision int64 `json:"revision"`
	}
	if !aiDecode(w, r, &in) {
		return
	}
	user := apphost.Current(r).User.ID
	a.crypto.mu.Lock()
	defer a.crypto.mu.Unlock()
	v, e := a.cryptoLoad(user)
	if e != nil {
		apphost.Problem(w, 400, "Configure and enable a monitor first")
		return
	}
	if in.Revision != v.State.Revision {
		apphost.Problem(w, 412, "The monitor changed; refresh before checking")
		return
	}
	if !v.State.Config.Enabled || v.ConsentedAt == "" {
		apphost.Problem(w, 400, "Enable monitoring with background consent first")
		return
	}
	if cryptoAwaitingReceipt(v.State) {
		apphost.Problem(w, 400, cryptoReceiptWaitMessage)
		return
	}
	if _, busy := a.crypto.inflight[user]; busy || cryptoPending(v.State, time.Now()) {
		apphost.Problem(w, 400, "Wait for the current check or review the pending proposal first")
		return
	}
	key, ke := a.cryptoKeyID(user)
	if ke != nil || key != v.KeyID {
		apphost.Problem(w, 400, "Review and save monitor consent after changing the DeepSeek key or model")
		return
	}
	old := v.State.Revision
	v.State.Revision++
	v.State.Status = "scheduled"
	v.State.NextRun = time.Now().UTC().Format(time.RFC3339Nano)
	if e = a.cryptoSave(v, old); e != nil {
		apphost.Problem(w, 503, e.Error())
		return
	}
	cryptoReply(w, &v, true)
}

func (a *App) cryptoDismiss(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var in struct {
		Revision   int64  `json:"revision"`
		ProposalID string `json:"proposalId"`
	}
	if !aiDecode(w, r, &in) {
		return
	}
	user := apphost.Current(r).User.ID
	a.crypto.mu.Lock()
	defer a.crypto.mu.Unlock()
	v, e := a.cryptoLoad(user)
	if e != nil {
		apphost.Problem(w, 400, "Monitor unavailable")
		return
	}
	if in.Revision != v.State.Revision {
		apphost.Problem(w, 412, "The monitor changed; refresh before dismissing")
		return
	}
	found := false
	for i := range v.State.Proposals {
		if v.State.Proposals[i].ID == in.ProposalID && v.State.Proposals[i].Status == "pending" {
			v.State.Proposals[i].Status = "dismissed"
			found = true
		}
	}
	if !found {
		apphost.Problem(w, 400, "No pending proposal matches this request")
		return
	}
	old := v.State.Revision
	v.State.Revision++
	if v.State.Config.Enabled {
		v.State.Status = "scheduled"
	}
	if e = a.cryptoSave(v, old); e != nil {
		apphost.Problem(w, 503, e.Error())
		return
	}
	a.cryptoCancelLocked(user)
	_, ke := a.cryptoKeyID(user)
	cryptoReply(w, &v, ke == nil)
}

func (a *App) cryptoReviewIntent(user, proposalID string) (cryptoTradeIntent, error) {
	a.crypto.mu.Lock()
	defer a.crypto.mu.Unlock()
	return a.cryptoIntentLocked(user, proposalID)
}
func (a *App) cryptoIntentLocked(user, proposalID string) (cryptoTradeIntent, error) {
	v, e := a.cryptoLoad(user)
	if e != nil || !a.cryptoActive(user) || !v.State.Config.Enabled {
		return cryptoTradeIntent{}, errors.New("The monitor is unavailable or paused")
	}
	key, e := a.cryptoKeyID(user)
	if e != nil || key != v.KeyID {
		return cryptoTradeIntent{}, errors.New("The DeepSeek settings changed; review monitor consent again")
	}
	for _, p := range v.State.Proposals {
		expires, e := time.Parse(time.RFC3339Nano, p.ExpiresAt)
		if p.ID == proposalID && p.Status == "pending" && e == nil && time.Now().Before(expires) && p.Revision == v.State.Revision {
			c := v.State.Config
			return cryptoTradeIntent{ProposalID: p.ID, Revision: p.Revision, WalletAddress: c.WalletAddress, ChainID: c.ChainID, SellToken: p.SellToken, BuyToken: p.BuyToken, SellAmount: p.SellAmount, BudgetUSD: c.BudgetUSD, MaxTradeUSD: c.MaxTradeUSD, ReserveETH: c.ReserveETH, SlippageBPS: c.SlippageBPS, ExpiresAt: expires}, nil
		}
	}
	return cryptoTradeIntent{}, errors.New("This proposal expired, changed or is no longer pending")
}
func (a *App) cryptoIntentCurrent(user string, intent cryptoTradeIntent) bool {
	a.crypto.mu.Lock()
	defer a.crypto.mu.Unlock()
	current, e := a.cryptoIntentLocked(user, intent.ProposalID)
	return e == nil && current == intent
}
func (a *App) cryptoRecordTransaction(user, proposalID, hash, status string) error {
	if !cryptoHash.MatchString(hash) || (status != "submitted" && status != "confirmed" && status != "reverted" && status != "unknown") {
		return errors.New("Invalid verified transaction receipt")
	}
	a.crypto.mu.Lock()
	defer a.crypto.mu.Unlock()
	v, e := a.cryptoLoad(user)
	if e != nil {
		return e
	}
	hash = strings.ToLower(hash)
	for i := range v.State.Proposals {
		p := &v.State.Proposals[i]
		if p.ID != proposalID {
			continue
		}
		if p.TransactionHash != "" && p.TransactionHash != hash {
			return errors.New("A different transaction is already recorded")
		}
		if p.Status == "confirmed" || p.Status == "reverted" {
			if p.Status != status {
				return errors.New("A final receipt cannot be downgraded")
			}
			return nil
		}
		p.TransactionHash = hash
		p.Status = status
		old := v.State.Revision
		wasWaiting := v.State.Status == "receipt"
		v.State.Revision++
		cryptoExpire(&v.State, time.Now(), true)
		if v.State.Config.Enabled {
			v.State.Status = "scheduled"
			if cryptoAwaitingReceipt(v.State) {
				v.State.Status = "receipt"
				v.State.LastError = cryptoReceiptWaitMessage
			} else if wasWaiting {
				v.State.LastError = ""
				v.State.NextRun = time.Now().UTC().Format(time.RFC3339Nano)
			}
		}
		if e = a.cryptoSave(v, old); e != nil {
			return e
		}
		a.cryptoCancelLocked(user)
		return nil
	}
	return errors.New("The proposal is no longer in monitor history")
}
