package finance

import (
	"context"
	"crypto/elliptic"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
)

type cryptoPushRuntime struct {
	mu     sync.Mutex
	client *http.Client
}
type cryptoPushKey struct {
	User, Origin, Private, Public string
}
type cryptoPushSubscription struct {
	User, Origin string
	Subscription webpush.Subscription
}

func (a *App) cryptoPushRoutes() error {
	if err := a.migrate(9, `CREATE TABLE finance_crypto_push_keys(user_id TEXT PRIMARY KEY,secret BLOB NOT NULL,last_test INTEGER NOT NULL DEFAULT 0);
CREATE TABLE finance_crypto_push_subscriptions(endpoint_hash TEXT PRIMARY KEY,user_id TEXT NOT NULL,secret BLOB NOT NULL,created INTEGER NOT NULL);
CREATE TABLE finance_crypto_push_outbox(id TEXT PRIMARY KEY,user_id TEXT NOT NULL,endpoint_hash TEXT NOT NULL,proposal_id TEXT NOT NULL,created INTEGER NOT NULL,next_attempt INTEGER NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,status TEXT NOT NULL DEFAULT 'pending');
CREATE INDEX finance_crypto_push_pending ON finance_crypto_push_outbox(status,next_attempt);`); err != nil {
		return err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	a.push = &cryptoPushRuntime{client: &http.Client{Timeout: 12 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	a.h.Mux.HandleFunc("GET /api/finance/crypto/push", a.h.Require(a.cryptoPushSettings))
	a.h.Mux.HandleFunc("POST /api/finance/crypto/push/subscribe", a.h.Require(a.cryptoPushSubscribe))
	a.h.Mux.HandleFunc("POST /api/finance/crypto/push/remove", a.h.Require(a.cryptoPushRemove))
	a.h.Mux.HandleFunc("POST /api/finance/crypto/push/test", a.h.Require(a.cryptoPushTest))
	return nil
}

func cryptoPushEndpoint(endpoint string) bool {
	if len(endpoint) < 20 || len(endpoint) > 4096 {
		return false
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Port() != "" || u.Opaque != "" || u.RawPath != "" || strings.Contains(u.Path, "..") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	// These are browser push-service origins, never user-provided webhooks/RPCs.
	switch host {
	case "fcm.googleapis.com":
		return strings.HasPrefix(u.Path, "/fcm/send/") || strings.HasPrefix(u.Path, "/wp/")
	case "updates.push.services.mozilla.com":
		return strings.HasPrefix(u.Path, "/wpush/")
	case "web.push.apple.com":
		return len(u.Path) > 1
	default:
		return strings.HasSuffix(host, ".notify.windows.com") && len(host) > len(".notify.windows.com") && len(u.Path) > 0
	}
}
func cryptoPushValid(s webpush.Subscription) bool {
	if !cryptoPushEndpoint(s.Endpoint) || len(s.Keys.Auth) > 32 || len(s.Keys.P256dh) > 100 {
		return false
	}
	auth, e1 := base64.RawURLEncoding.DecodeString(strings.TrimRight(s.Keys.Auth, "="))
	pub, e2 := base64.RawURLEncoding.DecodeString(strings.TrimRight(s.Keys.P256dh, "="))
	if e1 != nil || e2 != nil || len(auth) != 16 || len(pub) != 65 {
		return false
	}
	x, y := elliptic.Unmarshal(elliptic.P256(), pub)
	return x != nil && y != nil
}
func cryptoPushHash(s string) string {
	v := sha256.Sum256([]byte(s))
	return hex.EncodeToString(v[:])
}
func (a *App) cryptoPushAvailable() bool {
	u, err := url.Parse(a.h.Config.Origin)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))
}
func (a *App) cryptoPushLoadKey(user string, create bool) (cryptoPushKey, error) {
	var key cryptoPushKey
	var secret []byte
	err := a.h.DB.QueryRow(`SELECT secret FROM finance_crypto_push_keys WHERE user_id=?`, user).Scan(&secret)
	if errors.Is(err, sql.ErrNoRows) && create {
		private, public, e := webpush.GenerateVAPIDKeys()
		if e != nil {
			return key, e
		}
		key = cryptoPushKey{User: user, Origin: a.h.Config.Origin, Private: private, Public: public}
		plain, _ := json.Marshal(key)
		secret, e = protectSession(plain)
		clear(plain)
		if e != nil {
			return cryptoPushKey{}, e
		}
		_, e = a.h.DB.Exec(`INSERT INTO finance_crypto_push_keys(user_id,secret) VALUES(?,?)`, user, secret)
		return key, e
	}
	if err != nil || len(secret) > 8192 {
		return key, errors.New("push key unavailable")
	}
	plain, err := openSession(secret)
	if err != nil {
		return key, err
	}
	defer clear(plain)
	if json.Unmarshal(plain, &key) != nil || key.User != user || key.Origin != a.h.Config.Origin || key.Public == "" || key.Private == "" {
		return cryptoPushKey{}, errors.New("push key binding mismatch")
	}
	return key, nil
}
func (a *App) cryptoPushSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user := apphost.Current(r).User.ID
	a.push.mu.Lock()
	key, err := a.cryptoPushLoadKey(user, true)
	a.push.mu.Unlock()
	var count int
	if err != nil || a.h.DB.QueryRow(`SELECT count(*) FROM finance_crypto_push_subscriptions WHERE user_id=?`, user).Scan(&count) != nil {
		apphost.Problem(w, 503, "Notification settings could not be opened under this Windows account and address")
		return
	}
	apphost.JSON(w, 200, map[string]any{"publicKey": key.Public, "subscriptions": count, "available": a.cryptoPushAvailable()})
}
func (a *App) cryptoPushSubscribe(w http.ResponseWriter, r *http.Request) {
	var sub webpush.Subscription
	if !aiDecode(w, r, &sub) {
		return
	}
	if !a.cryptoPushAvailable() || !cryptoPushValid(sub) {
		apphost.Problem(w, 400, "Use a supported browser's notification permission on the secure Finance address")
		return
	}
	user, hash := apphost.Current(r).User.ID, cryptoPushHash(sub.Endpoint)
	a.push.mu.Lock()
	defer a.push.mu.Unlock()
	if _, err := a.cryptoPushLoadKey(user, true); err != nil {
		apphost.Problem(w, 503, "Notification key unavailable")
		return
	}
	var owner string
	err := a.h.DB.QueryRow(`SELECT user_id FROM finance_crypto_push_subscriptions WHERE endpoint_hash=?`, hash).Scan(&owner)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		apphost.Problem(w, 503, "Notification registration unavailable")
		return
	}
	if owner != "" && owner != user {
		apphost.Problem(w, 422, "This browser subscription belongs to another account. Disable its notifications and enable a new subscription")
		return
	}
	var count int
	if a.h.DB.QueryRow(`SELECT count(*) FROM finance_crypto_push_subscriptions WHERE user_id=?`, user).Scan(&count) != nil || count >= 5 && owner == "" {
		apphost.Problem(w, 400, "Use at most five notification devices for this account")
		return
	}
	plain, _ := json.Marshal(cryptoPushSubscription{User: user, Origin: a.h.Config.Origin, Subscription: sub})
	secret, err := protectManagerProfile(plain)
	clear(plain)
	if err == nil {
		_, err = a.h.DB.Exec(`INSERT INTO finance_crypto_push_subscriptions(endpoint_hash,user_id,secret,created) VALUES(?,?,?,?) ON CONFLICT(endpoint_hash) DO UPDATE SET secret=excluded.secret,created=excluded.created WHERE user_id=excluded.user_id`, hash, user, secret, time.Now().Unix())
	}
	if err != nil {
		apphost.Problem(w, 503, "Notification registration could not be saved")
		return
	}
	apphost.JSON(w, 200, map[string]any{"subscribed": true})
}
func (a *App) cryptoPushRemove(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Endpoint string `json:"endpoint"`
	}
	if !aiDecode(w, r, &in) {
		return
	}
	if len(in.Endpoint) > 4096 {
		apphost.Problem(w, 400, "Invalid browser subscription")
		return
	}
	user, hash := apphost.Current(r).User.ID, cryptoPushHash(in.Endpoint)
	if _, err := a.h.DB.Exec(`DELETE FROM finance_crypto_push_subscriptions WHERE user_id=? AND endpoint_hash=?`, user, hash); err != nil {
		apphost.Problem(w, 503, "Notification registration could not be removed")
		return
	}
	_, _ = a.h.DB.Exec(`DELETE FROM finance_crypto_push_outbox WHERE user_id=? AND endpoint_hash=?`, user, hash)
	apphost.JSON(w, 200, map[string]any{"subscribed": false})
}
func (a *App) cryptoPushQueue(user, proposal string) error {
	now := time.Now().Unix()
	_, err := a.h.DB.Exec(`INSERT OR IGNORE INTO finance_crypto_push_outbox(id,user_id,endpoint_hash,proposal_id,created,next_attempt)
SELECT user_id||':'||endpoint_hash||':'||?,user_id,endpoint_hash,?,?,? FROM finance_crypto_push_subscriptions WHERE user_id=?`, proposal, proposal, now, now, user)
	return err
}
func (a *App) cryptoQueueNotification(user, proposalID string) {
	if a.push != nil {
		_ = a.cryptoPushQueue(user, proposalID)
	}
	if a.email != nil {
		_ = a.cryptoEmailQueue(user, proposalID)
	}
}
func (a *App) cryptoPushTest(w http.ResponseWriter, r *http.Request) {
	var in struct{}
	if !aiDecode(w, r, &in) {
		return
	}
	user, now := apphost.Current(r).User.ID, time.Now().Unix()
	var count int
	if a.h.DB.QueryRow(`SELECT count(*) FROM finance_crypto_push_subscriptions WHERE user_id=?`, user).Scan(&count) != nil || count == 0 {
		apphost.Problem(w, 400, "Enable notifications on this device first")
		return
	}
	result, err := a.h.DB.Exec(`UPDATE finance_crypto_push_keys SET last_test=? WHERE user_id=? AND last_test<=?`, now, user, now-60)
	if err != nil {
		apphost.Problem(w, 503, "Notification test unavailable")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		apphost.Problem(w, 429, "Wait one minute before testing again")
		return
	}
	if err = a.cryptoPushQueue(user, "test-"+apphost.ID()); err != nil {
		apphost.Problem(w, 503, "Notification test could not be queued")
		return
	}
	apphost.JSON(w, 202, map[string]any{"queued": true, "message": "A generic notification is queued. Delivery depends on your browser and notification permission."})
}

func (a *App) cryptoPushLoop(ctx context.Context) {
	if a.push == nil {
		return
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.cryptoPushDispatch(ctx)
		}
	}
}
func (a *App) cryptoPushDispatch(ctx context.Context) {
	// A job is reserved before sending. A crash cannot create a rapid resend loop.
	now := time.Now().Unix()
	_, _ = a.h.DB.Exec(`DELETE FROM finance_crypto_push_outbox WHERE created<?`, now-86400)
	rows, err := a.h.DB.Query(`SELECT id,user_id,endpoint_hash,proposal_id,attempts FROM finance_crypto_push_outbox WHERE status='pending' AND attempts<3 AND next_attempt<=? AND created>? LIMIT 20`, now, now-1800)
	if err != nil {
		return
	}
	type job struct {
		id, user, endpoint, proposal string
		attempts                     int
	}
	var jobs []job
	for rows.Next() {
		var j job
		if rows.Scan(&j.id, &j.user, &j.endpoint, &j.proposal, &j.attempts) == nil {
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
			_, e := a.cryptoReviewIntent(j.user, j.proposal)
			valid = e == nil
		}
		if !valid {
			_, _ = a.h.DB.Exec(`UPDATE finance_crypto_push_outbox SET status='cancelled' WHERE id=?`, j.id)
			continue
		}
		result, e := a.h.DB.Exec(`UPDATE finance_crypto_push_outbox SET attempts=attempts+1,next_attempt=? WHERE id=? AND status='pending' AND attempts=?`, now+300, j.id, j.attempts)
		if e != nil {
			continue
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			continue
		}
		var secret []byte
		if a.h.DB.QueryRow(`SELECT secret FROM finance_crypto_push_subscriptions WHERE user_id=? AND endpoint_hash=?`, j.user, j.endpoint).Scan(&secret) != nil || len(secret) > 16<<10 {
			continue
		}
		plain, e := openSession(secret)
		var envelope cryptoPushSubscription
		if e == nil {
			e = json.Unmarshal(plain, &envelope)
		}
		clear(plain)
		if e != nil || envelope.User != j.user || envelope.Origin != a.h.Config.Origin || cryptoPushHash(envelope.Subscription.Endpoint) != j.endpoint || !cryptoPushValid(envelope.Subscription) {
			_, _ = a.h.DB.Exec(`UPDATE finance_crypto_push_outbox SET status='invalid' WHERE id=?`, j.id)
			continue
		}
		a.push.mu.Lock()
		key, e := a.cryptoPushLoadKey(j.user, false)
		a.push.mu.Unlock()
		if e != nil {
			continue
		}
		subject := a.h.Config.Origin
		if !strings.HasPrefix(subject, "https://") {
			subject = "https://github.com/oneworkspacestudio/my-finance"
		}
		resp, e := webpush.SendNotificationWithContext(ctx, []byte(`{"kind":"crypto-review"}`), &envelope.Subscription, &webpush.Options{Subscriber: subject, VAPIDPublicKey: key.Public, VAPIDPrivateKey: key.Private, HTTPClient: a.push.client, AuthScheme: webpush.Vapid, TTL: 300, Topic: "finance-crypto-review", Urgency: webpush.UrgencyNormal})
		if e != nil {
			continue
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if resp.StatusCode == 404 || resp.StatusCode == 410 {
			_, _ = a.h.DB.Exec(`DELETE FROM finance_crypto_push_subscriptions WHERE user_id=? AND endpoint_hash=?`, j.user, j.endpoint)
			_, _ = a.h.DB.Exec(`UPDATE finance_crypto_push_outbox SET status='expired' WHERE id=?`, j.id)
		} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			_, _ = a.h.DB.Exec(`UPDATE finance_crypto_push_outbox SET status='sent' WHERE id=?`, j.id)
		} else if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			_, _ = a.h.DB.Exec(`UPDATE finance_crypto_push_outbox SET status='rejected' WHERE id=?`, j.id)
		}
	}
}
