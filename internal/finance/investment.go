package finance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
)

type investmentRuntime struct {
	mu       sync.Mutex
	inflight map[string]paperInFlight
}
type paperInFlight struct {
	revision int64
	cancel   context.CancelFunc
}
type paperSetupInput struct {
	Revision int64        `json:"revision"`
	Confirm  bool         `json:"confirm"`
	Config   paperConfig  `json:"config"`
	Prices   []paperPrice `json:"prices"`
}
type paperPricesInput struct {
	Revision int64        `json:"revision"`
	Prices   []paperPrice `json:"prices"`
}
type paperRunInput struct {
	Revision  int64  `json:"revision"`
	Provider  string `json:"provider"`
	Decisions int    `json:"decisions"`
	Consent   bool   `json:"consent"`
}
type paperStepInput struct {
	Revision int64  `json:"revision"`
	RunID    string `json:"runId"`
}

func (a *App) investmentRoutes() error {
	if e := a.migrate(5, `CREATE TABLE finance_paper_investments(user_id TEXT PRIMARY KEY,revision INTEGER NOT NULL,secret BLOB NOT NULL);`); e != nil {
		return e
	}
	a.paper = &investmentRuntime{inflight: map[string]paperInFlight{}}
	// Runs never resume after a process restart, even if the previous process
	// stopped between charging an attempt and receiving the provider response.
	rows, e := a.h.DB.Query(`SELECT user_id FROM finance_paper_investments`)
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
		v, e := a.paperLoad(user)
		if e != nil {
			continue
		}
		if v.State.Run != nil || v.State.Status == "thinking" || v.State.Status == "running" {
			v.State.Status = "paused"
			v.State.Run = nil
			v.SessionHash = ""
			v.State.Revision++
			paperStop(&v.State)
			paperRecord(&v.State, "restart", "The server restarted. The previous run is paused; an interrupted provider attempt remains counted.", nil, time.Now())
			if e = a.paperSave(v); e != nil {
				return e
			}
		}
	}
	a.h.Mux.HandleFunc("GET /api/finance/investments/paper", a.h.Require(a.paperGet))
	a.h.Mux.HandleFunc("POST /api/finance/investments/paper/setup", a.h.Require(a.paperSetup))
	a.h.Mux.HandleFunc("POST /api/finance/investments/paper/prices", a.h.Require(a.paperPrices))
	a.h.Mux.HandleFunc("POST /api/finance/investments/paper/run", a.h.Require(a.paperRunStart))
	a.h.Mux.HandleFunc("POST /api/finance/investments/paper/step", a.h.Require(a.paperStep))
	a.h.Mux.HandleFunc("POST /api/finance/investments/paper/pause", a.h.Require(a.paperPause))
	return nil
}
func (a *App) paperLoad(user string) (paperEnvelope, error) {
	var v paperEnvelope
	var secret []byte
	var revision int64
	e := a.h.DB.QueryRow(`SELECT revision,secret FROM finance_paper_investments WHERE user_id=?`, user).Scan(&revision, &secret)
	if e != nil {
		return v, e
	}
	if len(secret) > 256<<10 {
		return v, errors.New("invalid protected paper record")
	}
	plain, e := openSession(secret)
	if e != nil {
		return v, e
	}
	defer clear(plain)
	if len(plain) > 128<<10 || json.Unmarshal(plain, &v) != nil || v.Version != 1 || v.User != user || v.State.Revision != revision || revision < 1 || revision > 1_000_000_000 || len(v.State.Audit) > 50 {
		return v, errors.New("invalid protected paper record")
	}
	if e = paperMark(&v.State); e != nil {
		return v, e
	}
	return v, nil
}
func (a *App) paperSave(v paperEnvelope) error {
	if v.Version != 1 || v.User == "" || v.State.Revision < 1 || v.State.Revision > 1_000_000_000 || len(v.State.Audit) > 50 {
		return errors.New("Invalid paper portfolio record")
	}
	if e := paperMark(&v.State); e != nil {
		return e
	}
	plain, e := json.Marshal(v)
	if e != nil {
		return e
	}
	secret, e := protectManagerProfile(plain)
	clear(plain)
	if e != nil {
		return e
	}
	_, e = a.h.DB.Exec(`INSERT INTO finance_paper_investments(user_id,revision,secret)VALUES(?,?,?) ON CONFLICT(user_id) DO UPDATE SET revision=excluded.revision,secret=excluded.secret`, v.User, v.State.Revision, secret)
	return e
}
func (a *App) paperView(w http.ResponseWriter, v *paperEnvelope) {
	view := map[string]any{"configured": v != nil, "instruments": paperInstruments, "simulation": nil, "aiPreview": nil}
	if v != nil {
		view["simulation"] = v.State
		view["aiPreview"] = paperPreview(v.State)
	}
	apphost.JSON(w, 200, view)
}
func paperLoadProblem(w http.ResponseWriter, e error) {
	if errors.Is(e, sql.ErrNoRows) {
		apphost.Problem(w, 404, "Create a paper portfolio first")
	} else {
		apphost.Problem(w, 422, "This protected paper portfolio could not be opened. Preserve your recovery copy and check the server's Windows account")
	}
}
func paperRevision(w http.ResponseWriter, expected int64, v paperEnvelope) bool {
	if expected != v.State.Revision {
		apphost.Problem(w, 412, "The paper portfolio changed. Reload its latest state before continuing")
		return false
	}
	return true
}
func (a *App) paperCancel(user string) {
	if flight, ok := a.paper.inflight[user]; ok {
		flight.cancel()
		delete(a.paper.inflight, user)
	}
}
func (a *App) paperInvalidateRun(v *paperEnvelope, session apphost.Session) bool {
	if v.State.Run == nil {
		return false
	}
	expires, e := time.Parse(time.RFC3339Nano, v.State.Run.Expires)
	if e == nil && time.Now().Before(expires) && v.SessionHash == apphost.Hash(session.CSRF) {
		return false
	}
	a.paperCancel(v.User)
	v.State.Run = nil
	v.SessionHash = ""
	v.State.Status = "paused"
	v.State.Revision++
	paperStop(&v.State)
	paperRecord(&v.State, "paused", "The approved run expired or its sign-in session changed. Start a new run to continue.", nil, time.Now())
	return true
}
func (a *App) paperGet(w http.ResponseWriter, r *http.Request) {
	a.paper.mu.Lock()
	defer a.paper.mu.Unlock()
	session := apphost.Current(r)
	v, e := a.paperLoad(session.User.ID)
	if errors.Is(e, sql.ErrNoRows) {
		a.paperView(w, nil)
		return
	}
	if e != nil {
		paperLoadProblem(w, e)
		return
	}
	if a.paperInvalidateRun(&v, session) {
		if e = a.paperSave(v); e != nil {
			apphost.Problem(w, 503, "The paper run could not be paused safely")
			return
		}
	}
	a.paperView(w, &v)
}
func (a *App) paperSetup(w http.ResponseWriter, r *http.Request) {
	var in paperSetupInput
	if !aiDecode(w, r, &in) {
		return
	}
	if !in.Confirm || !validPaperConfig(in.Config) || !validPaperPrices(in.Prices, nil) {
		apphost.Problem(w, 400, "Confirm a valid virtual GBP budget, goal, loss/order/position limits, decision cap and fictional scenario prices")
		return
	}
	a.paper.mu.Lock()
	defer a.paper.mu.Unlock()
	user := apphost.Current(r).User.ID
	old, e := a.paperLoad(user)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		paperLoadProblem(w, e)
		return
	}
	revision := int64(1)
	if e == nil {
		if !paperRevision(w, in.Revision, old) {
			return
		}
		revision = old.State.Revision + 1
	} else if in.Revision != 0 {
		apphost.Problem(w, 412, "Reload before creating a paper portfolio")
		return
	}
	v := paperEnvelope{Version: 1, User: user, State: newPaperState(apphost.ID(), revision, in.Config, in.Prices, time.Now())}
	if e = a.paperSave(v); e != nil {
		apphost.Problem(w, 503, "The paper portfolio could not be saved")
		return
	}
	a.paperCancel(user)
	a.paperView(w, &v)
}
func (a *App) paperPrices(w http.ResponseWriter, r *http.Request) {
	var in paperPricesInput
	if !aiDecode(w, r, &in) {
		return
	}
	a.paper.mu.Lock()
	defer a.paper.mu.Unlock()
	user := apphost.Current(r).User.ID
	v, e := a.paperLoad(user)
	if e != nil {
		paperLoadProblem(w, e)
		return
	}
	if !paperRevision(w, in.Revision, v) {
		return
	}
	if !validPaperPrices(in.Prices, v.State.Prices) {
		apphost.Problem(w, 400, "Enter positive GBP scenario prices for exactly this portfolio's fictional instruments")
		return
	}
	wasTerminal := paperTerminal(v.State.Status)
	v.State.Prices = append([]paperPrice{}, in.Prices...)
	v.State.Run = nil
	v.SessionHash = ""
	if !wasTerminal {
		v.State.Status = "paused"
	}
	v.State.Revision++
	if e = paperMark(&v.State); e != nil {
		apphost.Problem(w, 400, "The scenario prices could not be applied")
		return
	}
	if !wasTerminal {
		paperStop(&v.State)
	}
	paperRecord(&v.State, "prices", "Scenario prices changed manually. These are fictional GBP prices, not market quotes; the approved run is paused.", nil, time.Now())
	if e = a.paperSave(v); e != nil {
		apphost.Problem(w, 503, "Scenario prices could not be saved")
		return
	}
	a.paperCancel(user)
	a.paperView(w, &v)
}
func (a *App) paperRunStart(w http.ResponseWriter, r *http.Request) {
	var in paperRunInput
	if !aiDecode(w, r, &in) {
		return
	}
	if !in.Consent || aiDefault(in.Provider) == "" {
		apphost.Problem(w, 400, "Review the paper inputs and consent to this finite run with OpenAI or DeepSeek")
		return
	}
	a.paper.mu.Lock()
	defer a.paper.mu.Unlock()
	session := apphost.Current(r)
	v, e := a.paperLoad(session.User.ID)
	if e != nil {
		paperLoadProblem(w, e)
		return
	}
	if !paperRevision(w, in.Revision, v) {
		return
	}
	if v.State.Run != nil || paperTerminal(v.State.Status) || in.Decisions < 1 || in.Decisions > v.State.Config.MaxDecisions-v.State.Attempts {
		apphost.Problem(w, 400, "Pause the current run or choose a decision count within this portfolio's remaining approved limit")
		return
	}
	model, key, e := a.aiCredential(session.User.ID, in.Provider)
	key = ""
	_ = key
	if e != nil {
		apphost.Problem(w, 400, e.Error())
		return
	}
	v.State.Run = &paperRun{ID: apphost.ID(), Provider: in.Provider, Model: model, Remaining: in.Decisions, Expires: time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339Nano)}
	v.SessionHash = apphost.Hash(session.CSRF)
	v.State.Status = "running"
	v.State.Revision++
	paperRecord(&v.State, "approved", "Approved a finite run using the reviewed fictional portfolio and limits. Each decision can incur an AI API charge.", nil, time.Now())
	if e = a.paperSave(v); e != nil {
		apphost.Problem(w, 503, "The paper run could not be approved")
		return
	}
	a.paperView(w, &v)
}
func (a *App) paperPause(w http.ResponseWriter, r *http.Request) {
	var in paperStepInput
	if !aiDecode(w, r, &in) {
		return
	}
	a.paper.mu.Lock()
	defer a.paper.mu.Unlock()
	user := apphost.Current(r).User.ID
	v, e := a.paperLoad(user)
	if e != nil {
		paperLoadProblem(w, e)
		return
	}
	if !(in.RunID != "" && v.State.Run != nil && in.RunID == v.State.Run.ID) && !paperRevision(w, in.Revision, v) {
		return
	}
	v.State.Run = nil
	v.SessionHash = ""
	if !paperTerminal(v.State.Status) {
		v.State.Status = "paused"
	}
	v.State.Revision++
	paperRecord(&v.State, "paused", "The user paused this paper run. Any outstanding model result is discarded; a started API attempt may still be charged.", nil, time.Now())
	if e = a.paperSave(v); e != nil {
		apphost.Problem(w, 503, "The paper run could not be paused")
		return
	}
	a.paperCancel(user)
	a.paperView(w, &v)
}

func (a *App) paperStep(w http.ResponseWriter, r *http.Request) {
	var in paperStepInput
	if !aiDecode(w, r, &in) {
		return
	}
	session := apphost.Current(r)
	user := session.User.ID
	a.paper.mu.Lock()
	v, e := a.paperLoad(user)
	if e != nil {
		a.paper.mu.Unlock()
		paperLoadProblem(w, e)
		return
	}
	if a.paperInvalidateRun(&v, session) {
		e = a.paperSave(v)
		a.paper.mu.Unlock()
		if e != nil {
			apphost.Problem(w, 503, "Run state could not be saved")
		} else {
			apphost.Problem(w, 412, "This paper run expired or its session changed; reload and approve a new run")
		}
		return
	}
	if !paperRevision(w, in.Revision, v) {
		a.paper.mu.Unlock()
		return
	}
	if v.State.Status != "running" || v.State.Run == nil || v.State.Run.ID != in.RunID || v.State.Run.Remaining < 1 || v.State.Attempts >= v.State.Config.MaxDecisions {
		a.paper.mu.Unlock()
		apphost.Problem(w, 400, "This paper run is not ready for another decision")
		return
	}
	model, key, e := a.aiCredential(user, v.State.Run.Provider)
	if e != nil {
		a.paper.mu.Unlock()
		apphost.Problem(w, 400, e.Error())
		return
	}
	if model != v.State.Run.Model {
		a.paper.mu.Unlock()
		apphost.Problem(w, 412, "The provider model changed. Pause this run and approve the updated provider settings")
		return
	}
	if !a.aiBegin(user, time.Now()) {
		a.paper.mu.Unlock()
		apphost.Problem(w, 429, "Wait at least 30 seconds between AI decisions; your account can make ten AI requests per hour")
		return
	}
	defer a.aiEnd(user)
	v.State.Attempts++
	v.State.Run.Remaining--
	v.State.Status = "thinking"
	v.State.Revision++
	paperRecord(&v.State, "attempt", "A provider decision was started and counted before the outgoing request. It will not be retried automatically.", nil, time.Now())
	if e = a.paperSave(v); e != nil {
		a.paper.mu.Unlock()
		apphost.Problem(w, 503, "The attempt could not be recorded; no provider request was sent")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	expectedRevision := v.State.Revision
	a.paper.inflight[user] = paperInFlight{expectedRevision, cancel}
	provider := v.State.Run.Provider
	a.paper.mu.Unlock()
	decision, callError := a.paperCall(ctx, provider, model, key, v.State)
	key = ""
	cancel()
	a.paper.mu.Lock()
	defer a.paper.mu.Unlock()
	if flight, ok := a.paper.inflight[user]; ok && flight.revision == expectedRevision {
		delete(a.paper.inflight, user)
	}
	current, e := a.paperLoad(user)
	if e != nil {
		paperLoadProblem(w, e)
		return
	}
	if current.State.Revision != expectedRevision || current.State.Run == nil || current.State.Run.ID != in.RunID || current.State.Status != "thinking" {
		a.paperView(w, &current)
		return
	}
	// Recheck the originating session's persisted expiry/role before committing a
	// returned decision. The browser's identity may have been signed out meanwhile.
	var expiresText string
	e = a.h.DB.QueryRow(`SELECT s.expires FROM host_sessions s JOIN host_users u ON u.id=s.user_id WHERE s.user_id=? AND s.csrf=? AND u.role!='disabled' LIMIT 1`, user, session.CSRF).Scan(&expiresText)
	expires, parseError := time.Parse(time.RFC3339Nano, expiresText)
	if e != nil || parseError != nil || !time.Now().Before(expires) || !time.Now().Before(session.Expires) || a.paperInvalidateRun(&current, session) {
		callError = errors.New("The sign-in or approved paper run expired; the model result was discarded")
	}
	if callError == nil {
		next, applyError := applyPaperDecision(current.State, decision, time.Now())
		if applyError != nil {
			callError = applyError
		} else {
			current.State = next
		}
	}
	current.State.Revision++
	if callError != nil {
		current.State.Status = "paused"
		current.State.Run = nil
		current.SessionHash = ""
		paperRecord(&current.State, "rejected", callError.Error(), nil, time.Now())
	} else if current.State.Run.Remaining == 0 {
		current.State.Status = "paused"
		current.State.Run = nil
		current.SessionHash = ""
		paperRecord(&current.State, "completed", "The approved finite run completed. Approve a new run to use any remaining portfolio decision allowance.", nil, time.Now())
	} else {
		current.State.Status = "running"
	}
	paperStop(&current.State)
	if current.State.Run == nil {
		current.SessionHash = ""
	}
	if e = a.paperSave(current); e != nil {
		apphost.Problem(w, 503, "The provider attempt remains recorded but its result could not be saved. Reload; no automatic retry will occur")
		return
	}
	if callError != nil {
		apphost.Problem(w, 502, callError.Error())
		return
	}
	a.paperView(w, &current)
}
