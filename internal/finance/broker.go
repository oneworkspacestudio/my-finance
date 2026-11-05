package finance

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
)

// This adapter is deliberately a display-only connection. Its only upstream
// operations are the two literal GET paths below. No order/payment operation or
// arbitrary URL can be supplied by a browser, stored profile, or model output.
type brokerRuntime struct {
	mu     sync.Mutex
	usage  map[string]brokerUsage
	client *http.Client
}
type brokerUsage struct {
	last time.Time
	busy bool
}
type brokerKey struct {
	Version     int    `json:"version"`
	User        string `json:"user"`
	Environment string `json:"environment"`
	Key         string `json:"key"`
	Secret      string `json:"secret"`
}
type brokerSettingsInput struct {
	Environment     string `json:"environment"`
	Key             string `json:"apiKey"`
	Secret          string `json:"apiSecret"`
	ReadOnlyConsent bool   `json:"readOnlyConsent"`
	Remove          bool   `json:"remove"`
}
type brokerPosition struct {
	Ticker   string `json:"ticker"`
	Name     string `json:"name"`
	Quantity string `json:"quantity"`
}
type brokerSnapshot struct {
	Provider             string           `json:"provider"`
	Environment          string           `json:"environment"`
	ReadOnly             bool             `json:"readOnly"`
	AsOf                 string           `json:"asOf"`
	Currency             string           `json:"currency"`
	CashAvailable        string           `json:"cashAvailable"`
	TotalValue           string           `json:"totalValue"`
	InvestedValue        string           `json:"investedValue"`
	UnrealizedProfitLoss string           `json:"unrealizedProfitLoss"`
	Positions            []brokerPosition `json:"positions"`
}

func (a *App) brokerRoutes() error {
	if e := a.migrate(6, `CREATE TABLE finance_broker_keys(user_id TEXT PRIMARY KEY,environment TEXT NOT NULL,secret BLOB NOT NULL);`); e != nil {
		return e
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	a.broker = &brokerRuntime{usage: map[string]brokerUsage{}, client: &http.Client{Timeout: 25 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	a.h.Mux.HandleFunc("GET /api/finance/investments/broker/settings", a.h.Require(a.brokerSettings))
	a.h.Mux.HandleFunc("POST /api/finance/investments/broker/settings", a.h.Require(a.saveBrokerSettings))
	a.h.Mux.HandleFunc("POST /api/finance/investments/broker/refresh", a.h.Require(a.refreshBroker))
	return nil
}
func (a *App) brokerSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	environment := "demo"
	e := a.h.DB.QueryRow(`SELECT environment FROM finance_broker_keys WHERE user_id=?`, apphost.Current(r).User.ID).Scan(&environment)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		apphost.Problem(w, 503, "Broker settings are unavailable")
		return
	}
	apphost.JSON(w, 200, map[string]any{"configured": e == nil, "provider": "trading212", "environment": environment, "readOnly": true})
}
func brokerEnvironment(s string) bool { return s == "demo" || s == "live" }
func brokerCredentialText(s string) bool {
	return len(s) >= 8 && len(s) <= 1024 && strings.IndexFunc(s, func(c rune) bool { return c <= 32 || c > 126 }) < 0
}
func (a *App) saveBrokerSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var in brokerSettingsInput
	if !aiDecode(w, r, &in) {
		return
	}
	user := apphost.Current(r).User.ID
	if in.Remove {
		if _, e := a.h.DB.Exec(`DELETE FROM finance_broker_keys WHERE user_id=?`, user); e != nil {
			apphost.Problem(w, 503, "The local broker key could not be removed")
			return
		}
		a.brokerSettings(w, r)
		return
	}
	if !brokerEnvironment(in.Environment) || !in.ReadOnlyConsent || !brokerCredentialText(in.Key) || strings.Contains(in.Key, ":") || !brokerCredentialText(in.Secret) {
		apphost.Problem(w, 400, "Choose a broker environment, enter both key values and confirm read-only access")
		return
	}
	plain, e := json.Marshal(brokerKey{Version: 1, User: user, Environment: in.Environment, Key: in.Key, Secret: in.Secret})
	if e != nil {
		apphost.Problem(w, 400, "Broker settings could not be encoded")
		return
	}
	if len(plain) > 4096 {
		clear(plain)
		apphost.Problem(w, 400, "Broker credentials are too long; enter only the API key and secret")
		return
	}
	secret, e := protectSession(plain)
	clear(plain)
	in.Key = ""
	in.Secret = ""
	if e != nil {
		apphost.Problem(w, 503, "Windows could not protect this broker key; nothing was saved")
		return
	}
	if _, e = a.h.DB.Exec(`INSERT INTO finance_broker_keys(user_id,environment,secret) VALUES(?,?,?) ON CONFLICT(user_id) DO UPDATE SET environment=excluded.environment,secret=excluded.secret`, user, in.Environment, secret); e != nil {
		apphost.Problem(w, 503, "Broker settings could not be saved")
		return
	}
	a.brokerSettings(w, r)
}
func (a *App) loadBrokerKey(user string) (brokerKey, []byte, error) {
	var key brokerKey
	var environment string
	var secret []byte
	if e := a.h.DB.QueryRow(`SELECT environment,secret FROM finance_broker_keys WHERE user_id=?`, user).Scan(&environment, &secret); e != nil {
		return key, nil, errors.New("Add your own read-only Trading 212 API key first")
	}
	if len(secret) > 8192 {
		return key, nil, errors.New("Remove the unreadable broker key and enter it again")
	}
	plain, e := openSession(secret)
	if e != nil {
		return key, nil, errors.New("Re-enter your broker key under this Windows account")
	}
	defer clear(plain)
	if len(plain) > 4096 || json.Unmarshal(plain, &key) != nil || key.Version != 1 || key.User != user || key.Environment != environment || !brokerEnvironment(environment) || !brokerCredentialText(key.Key) || strings.Contains(key.Key, ":") || !brokerCredentialText(key.Secret) {
		return brokerKey{}, nil, errors.New("Remove the unreadable broker key and enter it again")
	}
	return key, secret, nil
}
func (b *brokerRuntime) begin(user string, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for k, v := range b.usage {
		if !v.busy && now.Sub(v.last) >= time.Minute {
			delete(b.usage, k)
		}
	}
	v, ok := b.usage[user]
	if v.busy || now.Sub(v.last) < 30*time.Second || !ok && len(b.usage) >= 2048 {
		return false
	}
	b.usage[user] = brokerUsage{last: now, busy: true}
	return true
}
func (b *brokerRuntime) end(user string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	v := b.usage[user]
	v.busy = false
	b.usage[user] = v
}
func (a *App) refreshBroker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var in struct {
		Consent bool `json:"consent"`
	}
	if !aiDecode(w, r, &in) {
		return
	}
	if !in.Consent {
		apphost.Problem(w, 400, "Confirm a read-only refresh from Trading 212")
		return
	}
	user := apphost.Current(r).User.ID
	key, original, e := a.loadBrokerKey(user)
	if e != nil {
		apphost.Problem(w, 422, e.Error())
		return
	}
	if !a.broker.begin(user, time.Now()) {
		apphost.Problem(w, 429, "Wait 30 seconds between broker refreshes")
		return
	}
	defer a.broker.end(user)
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	snapshot, e := a.broker.snapshot(ctx, key)
	key.Key = ""
	key.Secret = ""
	if e != nil {
		apphost.Problem(w, 502, e.Error())
		return
	}
	// Removing/replacing the credentials invalidates an in-flight refresh too.
	var current []byte
	var environment string
	if e = a.h.DB.QueryRow(`SELECT environment,secret FROM finance_broker_keys WHERE user_id=?`, user).Scan(&environment, &current); e != nil || environment != key.Environment || !bytes.Equal(current, original) {
		apphost.Problem(w, 422, "Broker access changed during refresh; load your current settings")
		return
	}
	apphost.JSON(w, 200, snapshot)
}

func (b *brokerRuntime) get(ctx context.Context, key brokerKey, path string, value any) error {
	if !brokerEnvironment(key.Environment) || path != "/equity/account/summary" && path != "/equity/positions" {
		return errors.New("Unsupported read-only broker operation")
	}
	host := "https://demo.trading212.com/api/v0"
	if key.Environment == "live" {
		host = "https://live.trading212.com/api/v0"
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, host+path, nil)
	if e != nil {
		return errors.New("Broker request could not be prepared")
	}
	req.SetBasicAuth(key.Key, key.Secret)
	req.Header.Set("Accept", "application/json")
	resp, e := b.client.Do(req)
	if e != nil {
		return errors.New("Trading 212 did not respond; your accounts were not changed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		switch resp.StatusCode {
		case 401, 403:
			return errors.New("Trading 212 rejected this key or its read permissions; check the selected environment and Account/Portfolio permissions")
		case 429:
			return errors.New("Trading 212 rate limit reached; wait before refreshing again")
		default:
			return errors.New("Trading 212 could not provide a portfolio snapshot; try again later")
		}
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if e != nil || len(raw) > 1<<20 || !utf8.Valid(raw) || json.Unmarshal(raw, value) != nil {
		return errors.New("Trading 212 returned an incomplete or unsupported snapshot")
	}
	return nil
}

var brokerCurrency = regexp.MustCompile(`^[A-Z]{3}$`)
var brokerDecimal = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]{1,3})?$`)

// Keep decimal values exact, but require JSON numbers. json.Number also accepts
// quoted numbers, which would hide an upstream schema change.
type brokerJSONNumber string

func (n *brokerJSONNumber) UnmarshalJSON(raw []byte) error {
	if len(raw) > 40 || !brokerDecimal.Match(raw) {
		return errors.New("Unsupported broker number")
	}
	*n = brokerJSONNumber(raw)
	return nil
}
func validBrokerNumber(n brokerJSONNumber, negative bool) bool {
	s := string(n)
	if len(s) > 40 || !brokerDecimal.MatchString(s) {
		return false
	}
	v, ok := new(big.Rat).SetString(s)
	return ok && (negative || v.Sign() >= 0) && new(big.Rat).Abs(v).Cmp(new(big.Rat).SetInt64(1_000_000_000_000)) <= 0
}
func (b *brokerRuntime) snapshot(ctx context.Context, key brokerKey) (brokerSnapshot, error) {
	var summary struct {
		Currency string `json:"currency"`
		Cash     struct {
			Available brokerJSONNumber `json:"availableToTrade"`
		} `json:"cash"`
		Investments struct {
			Current    brokerJSONNumber `json:"currentValue"`
			Unrealized brokerJSONNumber `json:"unrealizedProfitLoss"`
		} `json:"investments"`
		Total brokerJSONNumber `json:"totalValue"`
	}
	var rows []struct {
		Instrument struct {
			Ticker string `json:"ticker"`
			Name   string `json:"name"`
		} `json:"instrument"`
		Quantity brokerJSONNumber `json:"quantity"`
	}
	bad := errors.New("Trading 212 returned incomplete or unsupported portfolio figures")
	if e := b.get(ctx, key, "/equity/account/summary", &summary); e != nil {
		return brokerSnapshot{}, e
	}
	if !brokerCurrency.MatchString(summary.Currency) || !validBrokerNumber(summary.Cash.Available, false) || !validBrokerNumber(summary.Total, false) || !validBrokerNumber(summary.Investments.Current, false) || !validBrokerNumber(summary.Investments.Unrealized, true) {
		return brokerSnapshot{}, bad
	}
	if e := b.get(ctx, key, "/equity/positions", &rows); e != nil {
		return brokerSnapshot{}, e
	}
	if rows == nil || len(rows) > 2000 {
		return brokerSnapshot{}, bad
	}
	result := brokerSnapshot{Provider: "trading212", Environment: key.Environment, ReadOnly: true, AsOf: time.Now().UTC().Format(time.RFC3339), Currency: summary.Currency, CashAvailable: string(summary.Cash.Available), TotalValue: string(summary.Total), InvestedValue: string(summary.Investments.Current), UnrealizedProfitLoss: string(summary.Investments.Unrealized), Positions: []brokerPosition{}}
	seen := map[string]bool{}
	for _, row := range rows {
		if !aiText(row.Instrument.Ticker, 80) || !aiText(row.Instrument.Name, 200) || !validBrokerNumber(row.Quantity, false) || seen[row.Instrument.Ticker] {
			return brokerSnapshot{}, bad
		}
		seen[row.Instrument.Ticker] = true
		result.Positions = append(result.Positions, brokerPosition{Ticker: row.Instrument.Ticker, Name: row.Instrument.Name, Quantity: string(row.Quantity)})
	}
	return result, nil
}
