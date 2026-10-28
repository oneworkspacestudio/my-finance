package finance

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"io"
	"net/http"
	"strings"
	"time"
)

type connection struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Bank     string `json:"bank"`
	Status   string `json:"status"`
	Expires  string `json:"expires"`
	Accounts int    `json:"availableAccounts"`
	Created  string `json:"created"`
}

func (a *App) connectionRoutes() error {
	if e := a.migrate(1, `CREATE TABLE finance_connections(id TEXT PRIMARY KEY,user_id TEXT NOT NULL,session_hash TEXT NOT NULL,secret BLOB,bank TEXT NOT NULL,expires TEXT NOT NULL,status TEXT NOT NULL,accounts INTEGER NOT NULL,created TEXT NOT NULL,UNIQUE(user_id,session_hash));`); e != nil {
		return e
	}
	a.h.Mux.HandleFunc("GET /api/finance/connections", a.h.Owner(a.connections))
	a.h.Mux.HandleFunc("POST /api/finance/connections/{id}/check", a.h.Owner(a.checkConnection))
	a.h.Mux.HandleFunc("POST /api/finance/connections/{id}/revoke", a.h.Owner(a.revokeConnection))
	return nil
}
func (a *App) captureConnection(body []byte, s apphost.Session) error {
	var result struct {
		Status string
		Data   struct {
			SessionID  string `json:"session_id"`
			ValidUntil string `json:"valid_until"`
			Accounts   []json.RawMessage
			Aspsp      struct{ Name string }
		}
	}
	if json.Unmarshal(body, &result) != nil || result.Status != "ok" || result.Data.SessionID == "" {
		return nil
	}
	if !validSessionReference(result.Data.SessionID) || len(result.Data.Accounts) > 500 {
		return errors.New("Invalid provider session metadata")
	}
	secret, e := protectSession([]byte(result.Data.SessionID))
	if e != nil {
		return e
	}
	bank := result.Data.Aspsp.Name
	if len(bank) > 120 {
		bank = "Enable Banking"
	}
	expires := ""
	if t, e := time.Parse(time.RFC3339, result.Data.ValidUntil); e == nil {
		expires = t.UTC().Format(time.RFC3339)
	}
	_, e = a.h.DB.Exec(`INSERT INTO finance_connections(id,user_id,session_hash,secret,bank,expires,status,accounts,created)VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id,session_hash)DO NOTHING`, apphost.ID(), s.User.ID, apphost.Hash(result.Data.SessionID), secret, bank, expires, "Authorised", len(result.Data.Accounts), time.Now().UTC().Format(time.RFC3339))
	return e
}
func (a *App) connections(w http.ResponseWriter, r *http.Request) {
	rows, e := a.h.DB.Query(`SELECT id,bank,expires,status,accounts,created FROM finance_connections WHERE user_id=? ORDER BY created DESC LIMIT 128`, apphost.Current(r).User.ID)
	if e != nil {
		apphost.Problem(w, 500, "Connection history is unavailable")
		return
	}
	defer rows.Close()
	out := []connection{}
	for rows.Next() {
		v := connection{Provider: "Enable Banking"}
		if e = rows.Scan(&v.ID, &v.Bank, &v.Expires, &v.Status, &v.Accounts, &v.Created); e != nil {
			apphost.Problem(w, 500, "Connection history is unavailable")
			return
		}
		if t, e := time.Parse(time.RFC3339, v.Expires); e == nil && time.Now().After(t) && v.Status == "Authorised" {
			v.Status = "Needs reauthorisation"
		}
		out = append(out, v)
	}
	if rows.Err() != nil {
		apphost.Problem(w, 500, "Connection history is unavailable")
		return
	}
	apphost.JSON(w, 200, out)
}
func (a *App) connectionSecret(w http.ResponseWriter, r *http.Request) (string, bool) {
	var secret []byte
	var status string
	e := a.h.DB.QueryRow(`SELECT secret,status FROM finance_connections WHERE id=? AND user_id=?`, r.PathValue("id"), apphost.Current(r).User.ID).Scan(&secret, &status)
	if e != nil || len(secret) == 0 || status == "Revoked" {
		apphost.Problem(w, 404, "This active bank authorisation was not found")
		return "", false
	}
	cleartext, e := openSession(secret)
	if e != nil {
		apphost.Problem(w, 503, "Reconnect this bank under the current Windows account")
		return "", false
	}
	defer clear(cleartext)
	return string(cleartext), true
}
func (a *App) engineJSON(ctx context.Context, u apphost.User, route string, body any) (map[string]json.RawMessage, error) {
	if u.Role != "owner" {
		return nil, errors.New("Only the owner can manage provider permissions")
	}
	payload, _ := json.Marshal(body)
	req, e := http.NewRequestWithContext(ctx, "POST", a.engine+route, bytes.NewReader(payload))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Finance-Capability", a.cap)
	req.Header.Set("X-Finance-Identity", u.ID)
	req.Header.Set("X-Finance-Owner", "1")
	req.Header.Set("X-Finance-Display-Name", base64.RawURLEncoding.EncodeToString([]byte(u.Name)))
	client := http.Client{Timeout: 35 * time.Second}
	resp, e := client.Do(req)
	if e != nil {
		return nil, errors.New("The bank provider is temporarily unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("The provider could not confirm this permission. Check its dashboard or reconnect.")
	}
	var result struct {
		Status string
		Data   map[string]json.RawMessage
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 16<<10)).Decode(&result) != nil || result.Status != "ok" {
		return nil, errors.New("The provider returned an unexpected permission response")
	}
	return result.Data, nil
}
func (a *App) checkConnection(w http.ResponseWriter, r *http.Request) {
	secret, ok := a.connectionSecret(w, r)
	if !ok {
		return
	}
	data, e := a.engineJSON(r.Context(), apphost.Current(r).User, "/enablebanking/finance-session-status", map[string]string{"sessionId": secret})
	if e != nil {
		apphost.Problem(w, 503, e.Error())
		return
	}
	var status, expiry string
	json.Unmarshal(data["providerStatus"], &status)
	json.Unmarshal(data["validUntil"], &expiry)
	state := "Needs reauthorisation"
	if status == "AUTHORIZED" {
		state = "Authorised"
	}
	if t, e := time.Parse(time.RFC3339, expiry); e == nil {
		expiry = t.UTC().Format(time.RFC3339)
		if time.Now().After(t) {
			state = "Needs reauthorisation"
		}
	} else {
		expiry = ""
	}
	if _, e = a.h.DB.Exec(`UPDATE finance_connections SET status=?,expires=? WHERE id=? AND user_id=?`, state, expiry, r.PathValue("id"), apphost.Current(r).User.ID); e != nil {
		apphost.Problem(w, 500, "Permission was checked but could not be recorded")
		return
	}
	apphost.JSON(w, 200, map[string]string{"status": state, "expires": expiry})
}
func (a *App) revokeConnection(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Confirm bool `json:"confirm"`
	}
	if !apphost.Decode(w, r, &input) {
		return
	}
	if !input.Confirm {
		apphost.Problem(w, 400, "Confirm revocation of this provider authorisation")
		return
	}
	secret, ok := a.connectionSecret(w, r)
	if !ok {
		return
	}
	data, e := a.engineJSON(r.Context(), apphost.Current(r).User, "/enablebanking/finance-revoke", map[string]string{"sessionId": secret})
	if e != nil {
		apphost.Problem(w, 503, e.Error())
		return
	}
	var revoked bool
	json.Unmarshal(data["revoked"], &revoked)
	if !revoked {
		apphost.Problem(w, 503, "The provider has not confirmed revocation")
		return
	}
	if _, e = a.h.DB.Exec(`UPDATE finance_connections SET status='Revoked',secret=NULL WHERE id=? AND user_id=?`, r.PathValue("id"), apphost.Current(r).User.ID); e != nil {
		apphost.Problem(w, 500, "Provider access was revoked; local recording needs a retry/check")
		return
	}
	a.h.Audit(apphost.Current(r).User.ID, "finance.provider-session-revoked", r.PathValue("id"))
	apphost.JSON(w, 200, map[string]string{"status": "Revoked", "message": "Provider session revoked. Existing ledger data is retained. In your budget choose Unlink account to stop refresh attempts; check your bank dashboard for final consent closure."})
}

// Human-readable labels only. Never display or log native provider session IDs.
func validSessionReference(s string) bool {
	return len(s) > 0 && len(s) <= 200 && !strings.ContainsAny(s, "\r\n")
}
