package finance

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
)

type cryptoEmailRuntime struct {
	mu       sync.Mutex
	client   *http.Client
	inflight map[string]context.CancelFunc
}
type cryptoEmailConfig struct {
	User, Origin, From, To, APIKey string
	Revision                       int64
	Enabled                        bool
}
type cryptoEmailView struct {
	Provider       string `json:"provider"`
	Configured     bool   `json:"configured"`
	Enabled        bool   `json:"enabled"`
	From           string `json:"from"`
	To             string `json:"to"`
	Revision       int64  `json:"revision"`
	LastAcceptedAt string `json:"lastAcceptedAt"`
	LastError      string `json:"lastError"`
}

func (a *App) cryptoEmailRoutes() error {
	if err := a.migrate(10, `CREATE TABLE finance_crypto_email(user_id TEXT PRIMARY KEY,revision INTEGER NOT NULL,secret BLOB,last_test INTEGER NOT NULL DEFAULT 0,last_accepted TEXT NOT NULL DEFAULT '',last_error TEXT NOT NULL DEFAULT '');
CREATE TABLE finance_crypto_email_outbox(id TEXT PRIMARY KEY,user_id TEXT NOT NULL,revision INTEGER NOT NULL,proposal_id TEXT NOT NULL,created INTEGER NOT NULL,next_attempt INTEGER NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,status TEXT NOT NULL DEFAULT 'pending');
CREATE INDEX finance_crypto_email_pending ON finance_crypto_email_outbox(status,next_attempt);`); err != nil {
		return err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	a.email = &cryptoEmailRuntime{client: &http.Client{Timeout: 12 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, inflight: map[string]context.CancelFunc{}}
	a.h.Mux.HandleFunc("GET /api/finance/crypto/email", a.h.Require(a.cryptoEmailGet))
	a.h.Mux.HandleFunc("POST /api/finance/crypto/email", a.h.Require(a.cryptoEmailSettings))
	a.h.Mux.HandleFunc("POST /api/finance/crypto/email/test", a.h.Require(a.cryptoEmailTest))
	return nil
}
func cryptoEmailAddress(s string) bool {
	if len(s) < 3 || len(s) > 254 || strings.ContainsAny(s, "\r\n\t <>,;\"()") {
		return false
	}
	for _, c := range s {
		if c < 33 || c > 126 {
			return false
		}
	}
	v, err := mail.ParseAddress(s)
	return err == nil && v.Name == "" && v.Address == s && strings.Contains(s, "@")
}
func cryptoEmailKey(s string) bool {
	if len(s) < 8 || len(s) > 512 {
		return false
	}
	for _, c := range s {
		if c < 33 || c > 126 {
			return false
		}
	}
	return true
}
func (a *App) cryptoEmailLoad(user string) (cryptoEmailConfig, cryptoEmailView, error) {
	c := cryptoEmailConfig{User: user, Origin: a.h.Config.Origin}
	v := cryptoEmailView{Provider: "resend"}
	var secret []byte
	err := a.h.DB.QueryRow(`SELECT revision,secret,last_accepted,last_error FROM finance_crypto_email WHERE user_id=?`, user).Scan(&v.Revision, &secret, &v.LastAcceptedAt, &v.LastError)
	if errors.Is(err, sql.ErrNoRows) {
		return c, v, nil
	}
	if err != nil || v.Revision < 1 || v.Revision > 1000000000 || len(secret) > 16384 {
		return c, v, errors.New("email settings unavailable")
	}
	c.Revision = v.Revision
	if len(secret) == 0 {
		return c, v, nil
	}
	plain, err := openSession(secret)
	if err != nil {
		return c, v, errors.New("email settings could not be opened")
	}
	defer clear(plain)
	if json.Unmarshal(plain, &c) != nil || c.User != user || c.Origin != a.h.Config.Origin || c.Revision != v.Revision || !cryptoEmailAddress(c.From) || !cryptoEmailAddress(c.To) || !cryptoEmailKey(c.APIKey) {
		return cryptoEmailConfig{}, v, errors.New("email settings binding mismatch")
	}
	v.Configured, v.Enabled, v.From, v.To = true, c.Enabled, c.From, c.To
	return c, v, nil
}
func (a *App) cryptoEmailGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	a.email.mu.Lock()
	_, v, err := a.cryptoEmailLoad(apphost.Current(r).User.ID)
	a.email.mu.Unlock()
	if err != nil {
		apphost.Problem(w, 503, "Email settings could not be opened under this Windows account and Finance address")
		return
	}
	apphost.JSON(w, 200, v)
}
func (a *App) cryptoEmailSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var in struct {
		Revision int64  `json:"revision"`
		Enabled  bool   `json:"enabled"`
		From     string `json:"from"`
		To       string `json:"to"`
		APIKey   string `json:"apiKey"`
		Consent  bool   `json:"consent"`
		Remove   bool   `json:"remove"`
	}
	if !aiDecode(w, r, &in) {
		return
	}
	user := apphost.Current(r).User.ID
	a.email.mu.Lock()
	defer a.email.mu.Unlock()
	c, v, err := a.cryptoEmailLoad(user)
	if err != nil {
		apphost.Problem(w, 503, "Email settings could not be opened")
		return
	}
	if in.Revision != v.Revision || in.Revision >= 1000000000 {
		apphost.Problem(w, 412, "Email settings changed; reload before saving")
		return
	}
	var secret []byte
	if !in.Remove {
		if !cryptoEmailAddress(in.From) || !cryptoEmailAddress(in.To) || in.Enabled && !in.Consent {
			apphost.Problem(w, 400, "Enter one sender and recipient email address and agree to enabled email alerts")
			return
		}
		if in.APIKey != "" {
			c.APIKey = in.APIKey
		}
		if !cryptoEmailKey(c.APIKey) {
			apphost.Problem(w, 400, "Enter your Resend API key")
			return
		}
		c.From, c.To, c.Enabled, c.Revision = in.From, in.To, in.Enabled, v.Revision+1
		plain, _ := json.Marshal(c)
		secret, err = protectSession(plain)
		clear(plain)
		if err != nil {
			apphost.Problem(w, 503, "Email settings could not be protected")
			return
		}
	}
	// A tombstone retains revision/test throttle after removal. Pending jobs from
	// an earlier address/key configuration must never be sent to its replacement.
	_, err = a.h.DB.Exec(`INSERT INTO finance_crypto_email(user_id,revision,secret) VALUES(?,?,?) ON CONFLICT(user_id) DO UPDATE SET revision=excluded.revision,secret=excluded.secret,last_accepted='',last_error=''`, user, v.Revision+1, secret)
	if err != nil {
		apphost.Problem(w, 503, "Email settings could not be saved")
		return
	}
	if cancel := a.email.inflight[user]; cancel != nil {
		cancel()
	}
	_, _ = a.h.DB.Exec(`UPDATE finance_crypto_email_outbox SET status='cancelled' WHERE user_id=? AND status='pending'`, user)
	_, v, err = a.cryptoEmailLoad(user)
	if err != nil {
		apphost.Problem(w, 503, "Email settings could not be reloaded")
		return
	}
	apphost.JSON(w, 200, v)
}
func (a *App) cryptoEmailQueueLocked(user, proposal string, c cryptoEmailConfig) error {
	if !c.Enabled || c.APIKey == "" {
		return nil
	}
	now := time.Now().Unix()
	// Opaque stable key; no email, wallet or proposal text goes to the provider.
	id := cryptoPushHash(user + ":" + proposal + ":" + a.h.Config.Origin)
	_, err := a.h.DB.Exec(`INSERT OR IGNORE INTO finance_crypto_email_outbox(id,user_id,revision,proposal_id,created,next_attempt) VALUES(?,?,?,?,?,?)`, id, user, c.Revision, proposal, now, now)
	return err
}
func (a *App) cryptoEmailQueue(user, proposal string) error {
	a.email.mu.Lock()
	defer a.email.mu.Unlock()
	c, _, err := a.cryptoEmailLoad(user)
	if err != nil {
		return err
	}
	return a.cryptoEmailQueueLocked(user, proposal, c)
}
func (a *App) cryptoEmailTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var in struct{}
	if !aiDecode(w, r, &in) {
		return
	}
	user, now := apphost.Current(r).User.ID, time.Now().Unix()
	a.email.mu.Lock()
	defer a.email.mu.Unlock()
	c, v, err := a.cryptoEmailLoad(user)
	if err != nil || !v.Configured || !v.Enabled {
		apphost.Problem(w, 400, "Save and enable email alerts first")
		return
	}
	result, err := a.h.DB.Exec(`UPDATE finance_crypto_email SET last_test=? WHERE user_id=? AND last_test<=?`, now, user, now-60)
	if err != nil {
		apphost.Problem(w, 503, "Email test unavailable")
		return
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		apphost.Problem(w, 429, "Wait one minute before testing again")
		return
	}
	if a.cryptoEmailQueueLocked(user, "test-"+apphost.ID(), c) != nil {
		apphost.Problem(w, 503, "Email test could not be queued")
		return
	}
	apphost.JSON(w, 202, map[string]any{"queued": true, "message": "Test queued. Provider acceptance and inbox delivery are separate; check your inbox and spam folder."})
}
func (a *App) cryptoEmailLoop(ctx context.Context) {
	if a.email == nil {
		return
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.cryptoEmailDispatch(ctx)
		}
	}
}
func (a *App) cryptoEmailDispatch(ctx context.Context) {
	now := time.Now().Unix()
	_, _ = a.h.DB.Exec(`DELETE FROM finance_crypto_email_outbox WHERE created<?`, now-86400)
	rows, err := a.h.DB.Query(`SELECT id,user_id,revision,proposal_id,attempts FROM finance_crypto_email_outbox WHERE status='pending' AND attempts<3 AND next_attempt<=? AND created>=? ORDER BY created LIMIT 10`, now, now-1800)
	if err != nil {
		return
	}
	type job struct {
		id, user, proposal string
		revision           int64
		attempts           int
	}
	var jobs []job
	for rows.Next() {
		var j job
		if rows.Scan(&j.id, &j.user, &j.revision, &j.proposal, &j.attempts) == nil {
			jobs = append(jobs, j)
		}
	}
	rows.Close()
	for _, j := range jobs {
		if ctx.Err() != nil {
			return
		}
		var role string
		valid := a.h.DB.QueryRow(`SELECT role FROM host_users WHERE id=?`, j.user).Scan(&role) == nil && role != "disabled"
		if valid && !strings.HasPrefix(j.proposal, "test-") {
			if a.crypto == nil {
				valid = false
			} else {
				_, err = a.cryptoReviewIntent(j.user, j.proposal)
				valid = err == nil
			}
		}
		a.email.mu.Lock()
		c, _, loadErr := a.cryptoEmailLoad(j.user)
		if !valid || loadErr != nil || !c.Enabled || c.Revision != j.revision {
			_, _ = a.h.DB.Exec(`UPDATE finance_crypto_email_outbox SET status='cancelled' WHERE id=?`, j.id)
			a.email.mu.Unlock()
			continue
		}
		if a.email.inflight[j.user] != nil {
			a.email.mu.Unlock()
			continue
		}
		result, e := a.h.DB.Exec(`UPDATE finance_crypto_email_outbox SET attempts=attempts+1,next_attempt=? WHERE id=? AND status='pending' AND attempts=?`, now+300, j.id, j.attempts)
		if e != nil {
			a.email.mu.Unlock()
			continue
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			a.email.mu.Unlock()
			continue
		}
		requestCtx, cancel := context.WithCancel(ctx)
		a.email.inflight[j.user] = cancel
		a.email.mu.Unlock()
		body, _ := json.Marshal(map[string]any{"from": c.From, "to": []string{c.To}, "subject": "My Finance crypto review", "text": "Open My Finance to review your crypto monitor. Sign in and check the current proposal before making any decision. This email does not approve a transaction.\n\n" + strings.TrimRight(c.Origin, "/") + "/#crypto\n"})
		req, _ := http.NewRequestWithContext(requestCtx, "POST", "https://api.resend.com/emails", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "finance-crypto-"+j.id)
		resp, e := a.email.client.Do(req)
		status, lastError := "pending", "Email provider could not be reached; bounded retry is pending"
		if e == nil {
			raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 8193))
			resp.Body.Close()
			var accepted struct {
				ID string `json:"id"`
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 && readErr == nil && len(raw) <= 8192 && json.Unmarshal(raw, &accepted) == nil && accepted.ID != "" {
				status, lastError = "accepted", ""
			} else if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
				status, lastError = "rejected", "Resend rejected the email. Check the API key, verified sender and permitted recipient."
			} else {
				lastError = "Email acceptance is unknown; bounded retry uses the same idempotency key"
			}
		}
		cancel()
		a.email.mu.Lock()
		delete(a.email.inflight, j.user)
		// Never write the old request's result into changed notification settings.
		current, _, e := a.cryptoEmailLoad(j.user)
		if e == nil && current.Revision == j.revision {
			if status == "pending" && j.attempts+1 >= 3 {
				status = "exhausted"
				lastError = "Email acceptance is unknown after three attempts; check Resend and your inbox"
			}
			_, _ = a.h.DB.Exec(`UPDATE finance_crypto_email_outbox SET status=? WHERE id=? AND status='pending'`, status, j.id)
			if status == "accepted" {
				_, _ = a.h.DB.Exec(`UPDATE finance_crypto_email SET last_accepted=?,last_error='' WHERE user_id=? AND revision=?`, time.Now().UTC().Format(time.RFC3339), j.user, j.revision)
			} else {
				_, _ = a.h.DB.Exec(`UPDATE finance_crypto_email SET last_error=? WHERE user_id=? AND revision=?`, lastError, j.user, j.revision)
			}
		}
		a.email.mu.Unlock()
	}
}
