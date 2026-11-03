package finance

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
)

// A scope is a browser-derived digest of the synced budget ID, or the explicit
// manual-estimates workspace. It never grants access to an Actual budget.
var managerScope = regexp.MustCompile(`^(manual|[a-f0-9]{64})$`)

type managerSettingsInput struct {
	Scope   string     `json:"scope"`
	Profile *aiProfile `json:"profile,omitempty"`
	Remove  bool       `json:"remove"`
}

type managerSettingsEnvelope struct {
	Version int       `json:"version"`
	User    string    `json:"user"`
	Scope   string    `json:"scope"`
	Profile aiProfile `json:"profile"`
}

func (a *App) managerSettingsRoutes() error {
	if e := a.migrate(4, `CREATE TABLE finance_manager_settings(user_id TEXT NOT NULL,scope TEXT NOT NULL,secret BLOB NOT NULL,PRIMARY KEY(user_id,scope));`); e != nil {
		return e
	}
	a.h.Mux.HandleFunc("GET /api/finance/ai/manager-settings", a.h.Require(a.managerSettings))
	a.h.Mux.HandleFunc("POST /api/finance/ai/manager-settings", a.h.Require(a.saveManagerSettings))
	return nil
}

func (a *App) managerSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	scope := r.URL.Query().Get("scope")
	if !managerScope.MatchString(scope) {
		apphost.Problem(w, 400, "Choose a budget or manual estimates before loading manager rules")
		return
	}
	user := apphost.Current(r).User.ID
	var secret []byte
	e := a.h.DB.QueryRow(`SELECT secret FROM finance_manager_settings WHERE user_id=? AND scope=?`, user, scope).Scan(&secret)
	if errors.Is(e, sql.ErrNoRows) {
		apphost.JSON(w, 200, map[string]any{"configured": false})
		return
	}
	if e != nil {
		apphost.Problem(w, 503, "Manager rules are unavailable")
		return
	}
	if len(secret) > 256<<10 {
		apphost.Problem(w, 422, "Saved manager rules exceed the supported size; remove them and enter your rules again")
		return
	}
	plain, e := openSession(secret)
	if e != nil {
		apphost.Problem(w, 422, "Saved manager rules cannot be opened under this Windows account; remove them and enter your rules again")
		return
	}
	defer clear(plain)
	var envelope managerSettingsEnvelope
	if len(plain) > 128<<10 || json.Unmarshal(plain, &envelope) != nil || envelope.Version != 1 || envelope.User != user || envelope.Scope != scope || validateAIProfile(envelope.Profile) != nil {
		apphost.Problem(w, 422, "Saved manager rules could not be verified; remove them and enter your rules again")
		return
	}
	apphost.JSON(w, 200, map[string]any{"configured": true, "profile": envelope.Profile})
}

func (a *App) saveManagerSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var in managerSettingsInput
	if !aiDecode(w, r, &in) {
		return
	}
	if !managerScope.MatchString(in.Scope) {
		apphost.Problem(w, 400, "Choose a budget or manual estimates before saving manager rules")
		return
	}
	user := apphost.Current(r).User.ID
	if in.Remove {
		if _, e := a.h.DB.Exec(`DELETE FROM finance_manager_settings WHERE user_id=? AND scope=?`, user, in.Scope); e != nil {
			apphost.Problem(w, 503, "Manager rules could not be removed")
			return
		}
		apphost.JSON(w, 200, map[string]bool{"saved": false, "removed": true})
		return
	}
	if in.Profile == nil {
		apphost.Problem(w, 400, "Enter manager rules before saving")
		return
	}
	if e := validateAIProfile(*in.Profile); e != nil {
		apphost.Problem(w, 400, e.Error())
		return
	}
	plain, e := json.Marshal(managerSettingsEnvelope{Version: 1, User: user, Scope: in.Scope, Profile: *in.Profile})
	if e != nil {
		apphost.Problem(w, 400, "Manager rules could not be encoded")
		return
	}
	secret, e := protectManagerProfile(plain)
	clear(plain)
	if e != nil {
		apphost.Problem(w, 503, "Windows could not protect these rules; nothing was saved")
		return
	}
	// Bound storage atomically while allowing an existing scope to be updated.
	result, e := a.h.DB.Exec(`INSERT INTO finance_manager_settings(user_id,scope,secret)
		SELECT ?,?,? WHERE (SELECT count(*) FROM finance_manager_settings WHERE user_id=?) < 50
		OR EXISTS(SELECT 1 FROM finance_manager_settings WHERE user_id=? AND scope=?)
		ON CONFLICT(user_id,scope) DO UPDATE SET secret=excluded.secret`, user, in.Scope, secret, user, user, in.Scope)
	if e != nil {
		apphost.Problem(w, 503, "Manager rules could not be saved")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		apphost.Problem(w, 422, "Remove saved rules for an unused budget before adding another; each account supports fifty budgets")
		return
	}
	apphost.JSON(w, 200, map[string]bool{"saved": true})
}
