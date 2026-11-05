package finance

import (
	"context"
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

type cryptoWalletRuntime struct {
	mu     sync.Mutex
	client *http.Client
	last   map[string]time.Time
}
type cryptoWalletTx struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Data    string `json:"data"`
	Value   string `json:"value"`
	ChainID string `json:"chainId"`
	Gas     string `json:"gas,omitempty"`
}
type cryptoWalletQuote struct {
	ID               string               `json:"id"`
	ProposalID       string               `json:"proposalId"`
	WalletAddress    string               `json:"walletAddress"`
	ChainID          int                  `json:"chainId"`
	SellToken        string               `json:"sellToken"`
	BuyToken         string               `json:"buyToken"`
	SellAmount       string               `json:"sellAmount"`
	BuyAmount        string               `json:"buyAmount"`
	MinBuyAmount     string               `json:"minBuyAmount"`
	Allowance        string               `json:"allowance"`
	SlippageBPS      int                  `json:"slippageBps"`
	PoolFee          int                  `json:"poolFee"`
	BlockNumber      string               `json:"blockNumber"`
	ExpiresAt        string               `json:"expiresAt"`
	Deadline         int64                `json:"deadline"`
	ApprovalRequired bool                 `json:"approvalRequired"`
	ResetRequired    bool                 `json:"resetRequired"`
	Snapshot         cryptoWalletSnapshot `json:"snapshot"`
	Market           cryptoMarket         `json:"market"`
}
type cryptoWalletRecord struct {
	User     string                    `json:"user"`
	Session  string                    `json:"session"`
	Intent   cryptoTradeIntent         `json:"intent"`
	Quote    cryptoWalletQuote         `json:"quote"`
	Prepared map[string]cryptoWalletTx `json:"prepared"`
	Hashes   map[string]string         `json:"hashes"`
	Status   map[string]string         `json:"status"`
}

func (a *App) cryptoWalletRoutes() error {
	if e := a.migrate(8, `CREATE TABLE finance_crypto_wallet(user_id TEXT NOT NULL,quote_id TEXT NOT NULL,created_at TEXT NOT NULL,secret BLOB NOT NULL,PRIMARY KEY(user_id,quote_id));`); e != nil {
		return e
	}
	a.wallet = &cryptoWalletRuntime{client: cryptoRPCClient(), last: map[string]time.Time{}}
	a.h.Mux.HandleFunc("POST /api/finance/crypto/wallet/quote", a.h.Require(a.cryptoWalletMakeQuote))
	a.h.Mux.HandleFunc("POST /api/finance/crypto/wallet/prepare", a.h.Require(a.cryptoWalletPrepare))
	a.h.Mux.HandleFunc("POST /api/finance/crypto/wallet/receipt", a.h.Require(a.cryptoWalletReceipt))
	a.h.Mux.HandleFunc("GET /api/finance/crypto/wallet/reviews", a.h.Require(a.cryptoWalletReviews))
	return nil
}
func (a *App) cryptoWalletSave(v cryptoWalletRecord) error {
	b, e := json.Marshal(v)
	if e != nil || len(b) > 48<<10 {
		return errors.New("Wallet record too large")
	}
	secret, e := protectManagerProfile(b)
	clear(b)
	if e != nil {
		return errors.New("Windows could not protect the wallet review")
	}
	_, e = a.h.DB.Exec(`INSERT INTO finance_crypto_wallet(user_id,quote_id,created_at,secret)VALUES(?,?,?,?) ON CONFLICT(user_id,quote_id) DO UPDATE SET secret=excluded.secret`, v.User, v.Quote.ID, time.Now().UTC().Format(time.RFC3339Nano), secret)
	return e
}

// Linearize issuing a new wallet request with monitor pause/settings changes.
// Already issued wallet requests cannot be recalled; the swap's on-chain deadline
// remains effective even if the browser closes while its confirmation is open.
func (a *App) cryptoWalletSaveCurrent(v cryptoWalletRecord, session apphost.Session) error {
	a.crypto.mu.Lock()
	defer a.crypto.mu.Unlock()
	current, e := a.cryptoIntentLocked(v.User, v.Intent.ProposalID)
	if e != nil || current != v.Intent {
		return errors.New("Proposal changed before the wallet request was issued")
	}
	var active int
	if e = a.h.DB.QueryRow(`SELECT count(*) FROM host_sessions WHERE user_id=? AND csrf=? AND expires>?`, v.User, session.CSRF, time.Now().UTC().Format(time.RFC3339Nano)).Scan(&active); e != nil || active != 1 {
		return errors.New("Sign-in expired; sign in and review again")
	}
	expires, e := time.Parse(time.RFC3339, v.Quote.ExpiresAt)
	if e != nil || !time.Now().Before(expires) {
		return errors.New("Quote expired; request a fresh quote")
	}
	return a.cryptoWalletSave(v)
}
func (a *App) cryptoWalletLoad(user, id string) (cryptoWalletRecord, error) {
	var v cryptoWalletRecord
	var secret []byte
	e := a.h.DB.QueryRow(`SELECT secret FROM finance_crypto_wallet WHERE user_id=? AND quote_id=?`, user, id).Scan(&secret)
	if e != nil {
		return v, errors.New("Wallet review not found; request a new quote")
	}
	if len(secret) > 96<<10 {
		return v, errors.New("Wallet review is oversized")
	}
	b, e := openSession(secret)
	if e != nil {
		return v, errors.New("Wallet review could not be opened")
	}
	defer clear(b)
	if len(b) > 48<<10 || json.Unmarshal(b, &v) != nil || v.User != user || v.Quote.ID != id || v.Prepared == nil || v.Hashes == nil || v.Status == nil {
		return v, errors.New("Invalid wallet review")
	}
	return v, nil
}
func (a *App) cryptoWalletRecords(user string) ([]cryptoWalletRecord, error) {
	rows, e := a.h.DB.Query(`SELECT quote_id FROM finance_crypto_wallet WHERE user_id=? ORDER BY created_at DESC LIMIT 101`, user)
	if e != nil {
		return nil, e
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) != nil {
			rows.Close()
			return nil, errors.New("Wallet history unavailable")
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil || len(ids) > 100 {
		return nil, errors.New("Wallet history unavailable or over its storage limit")
	}
	var records []cryptoWalletRecord
	for _, id := range ids {
		v, e := a.cryptoWalletLoad(user, id)
		if e != nil {
			return nil, e
		}
		records = append(records, v)
	}
	return records, nil
}
func cryptoWalletSwapUnresolved(v cryptoWalletRecord) bool {
	if v.Status["swap"] == "confirmed" || v.Status["swap"] == "reverted" {
		return false
	}
	if v.Hashes["swap"] != "" {
		return true
	}
	_, prepared := v.Prepared["swap"]
	return prepared && v.Quote.Deadline > time.Now().Unix()
}

// Read-only, without wallet.mu: monitor callers may already hold crypto.mu.
func (a *App) cryptoWalletUnresolved(user, wallet string) bool {
	records, e := a.cryptoWalletRecords(user)
	if e != nil {
		return true
	}
	for _, v := range records {
		if strings.EqualFold(v.Quote.WalletAddress, wallet) && cryptoWalletSwapUnresolved(v) {
			return true
		}
	}
	return false
}
func (a *App) cryptoWalletPending(user, proposal, wallet string) bool {
	records, e := a.cryptoWalletRecords(user)
	if e != nil {
		return true
	}
	for _, v := range records {
		if strings.EqualFold(v.Quote.WalletAddress, wallet) && cryptoWalletSwapUnresolved(v) {
			return true
		}
		if v.Quote.ProposalID == proposal && v.Hashes["swap"] != "" && v.Status["swap"] != "reverted" {
			return true
		}
	}
	return false
}
func cryptoWalletKeepRecord(v cryptoWalletRecord) bool {
	if cryptoWalletSwapUnresolved(v) {
		return true
	}
	for kind, hash := range v.Hashes {
		if hash != "" && v.Status[kind] != "confirmed" && v.Status[kind] != "reverted" {
			return true
		}
	}
	return false
}

// Called under wallet.mu before inserting. Never evict unresolved outcomes,
// including when settings temporarily switch to a different monitored wallet.
func (a *App) cryptoWalletMakeRoom(user string) error {
	records, e := a.cryptoWalletRecords(user)
	if e != nil {
		return e
	}
	if len(records) < 100 {
		return nil
	}
	for i := len(records) - 1; i >= 0; i-- {
		if !cryptoWalletKeepRecord(records[i]) {
			_, e = a.h.DB.Exec(`DELETE FROM finance_crypto_wallet WHERE user_id=? AND quote_id=?`, user, records[i].Quote.ID)
			return e
		}
	}
	return errors.New("Resolve pending wallet receipts before creating more reviews")
}
func walletProblem(w http.ResponseWriter, e error) { apphost.Problem(w, 422, e.Error()) }
func (a *App) cryptoWalletMakeQuote(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var in struct {
		ProposalID string `json:"proposalId"`
	}
	if !aiDecode(w, r, &in) {
		return
	}
	s := apphost.Current(r)
	a.wallet.mu.Lock()
	defer a.wallet.mu.Unlock()
	if time.Since(a.wallet.last[s.User.ID]) < 3*time.Second {
		apphost.Problem(w, 429, "Wait a moment before refreshing the quote")
		return
	}
	a.wallet.last[s.User.ID] = time.Now()
	intent, e := a.cryptoReviewIntent(s.User.ID, in.ProposalID)
	if e != nil {
		walletProblem(w, e)
		return
	}
	if a.cryptoWalletPending(s.User.ID, in.ProposalID, intent.WalletAddress) {
		walletProblem(w, errors.New("A swap was already prepared or submitted; check its receipt or wait for its on-chain deadline"))
		return
	}
	if e = a.cryptoWalletMakeRoom(s.User.ID); e != nil {
		walletProblem(w, e)
		return
	}
	q, e := a.cryptoBuildQuote(r.Context(), intent)
	if e != nil {
		walletProblem(w, e)
		return
	}
	if !a.cryptoIntentCurrent(s.User.ID, intent) {
		walletProblem(w, errors.New("Proposal changed; review the current proposal"))
		return
	}
	v := cryptoWalletRecord{User: s.User.ID, Session: apphost.Hash(s.CSRF), Intent: intent, Quote: q, Prepared: map[string]cryptoWalletTx{}, Hashes: map[string]string{}, Status: map[string]string{}}
	if e = a.cryptoWalletSaveCurrent(v, s); e != nil {
		walletProblem(w, e)
		return
	}
	apphost.JSON(w, 200, q)
}
func (a *App) cryptoBuildQuote(ctx context.Context, intent cryptoTradeIntent) (cryptoWalletQuote, error) {
	var q cryptoWalletQuote
	if intent.ChainID != 8453 || !cryptoWalletAddress(intent.WalletAddress) || intent.SlippageBPS < 10 || intent.SlippageBPS > 100 || time.Until(intent.ExpiresAt) < 30*time.Second {
		return q, errors.New("Proposal is invalid or about to expire")
	}
	data, e := cryptoQuoteData(intent)
	if e != nil {
		return q, e
	}
	block, blockTime, e := a.cryptoChainBlock(ctx)
	if e != nil {
		return q, e
	}
	snapshot, e := a.cryptoSnapshotAt(ctx, intent.WalletAddress, block)
	if e != nil {
		return q, e
	}
	var result string
	if e = a.cryptoRPC(ctx, "eth_call", []any{map[string]string{"to": cryptoQuoter, "data": data}, block}, &result); e != nil {
		return q, errors.New("The direct ETH/USDC pool cannot quote this amount right now")
	}
	if len(result) != 258 || !strings.HasPrefix(result, "0x") {
		return q, errors.New("Invalid Uniswap quote response")
	}
	if _, e = hex.DecodeString(result[2:]); e != nil {
		return q, errors.New("Invalid Uniswap quote encoding")
	}
	out, e := cryptoHexUint(result[:66])
	if e != nil || out.Sign() == 0 {
		return q, errors.New("No executable quote available")
	}
	min := new(big.Int).Div(new(big.Int).Mul(out, big.NewInt(int64(10000-intent.SlippageBPS))), big.NewInt(10000))
	if min.Sign() == 0 {
		return q, errors.New("Swap amount is too small")
	}
	allowance := big.NewInt(0)
	if intent.SellToken == "USDC" {
		allowance, e = a.cryptoContractUint(ctx, cryptoUSDC, cryptoCallData("allowance(address,address)", cryptoAddressWord(intent.WalletAddress), cryptoAddressWord(cryptoRouter)), block)
		if e != nil {
			return q, e
		}
	}
	q = cryptoWalletQuote{ID: apphost.ID(), ProposalID: intent.ProposalID, WalletAddress: strings.ToLower(intent.WalletAddress), ChainID: 8453, SellToken: intent.SellToken, BuyToken: intent.BuyToken, SellAmount: intent.SellAmount, BuyAmount: out.String(), MinBuyAmount: min.String(), Allowance: allowance.String(), SlippageBPS: intent.SlippageBPS, PoolFee: cryptoPoolFee, BlockNumber: block, ExpiresAt: time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339), Deadline: blockTime + 120, Snapshot: snapshot}
	q.Market, e = a.cryptoMarketRead(ctx)
	if e != nil {
		return cryptoWalletQuote{}, e
	}
	if intent.ExpiresAt.Unix() < q.Deadline {
		q.Deadline = intent.ExpiresAt.Unix()
	}
	amount, _ := cryptoUint(intent.SellAmount)
	q.ApprovalRequired = intent.SellToken == "USDC" && allowance.Cmp(amount) != 0
	q.ResetRequired = q.ApprovalRequired && allowance.Sign() > 0
	if e = cryptoWalletLimits(intent, q, big.NewInt(0)); e != nil {
		return cryptoWalletQuote{}, e
	}
	return q, nil
}
func cryptoWalletLimits(intent cryptoTradeIntent, q cryptoWalletQuote, gasCost *big.Int) error {
	amount, e := cryptoUint(intent.SellAmount)
	if e != nil {
		return e
	}
	eth, e := cryptoUint(q.Snapshot.ETH)
	if e != nil {
		return e
	}
	usdc, e := cryptoUint(q.Snapshot.USDC)
	if e != nil {
		return e
	}
	reserve, e := cryptoDecimalUnits(intent.ReserveETH, 18)
	if e != nil {
		return e
	}
	if reserve.Cmp(big.NewInt(100000000000000)) < 0 {
		reserve = big.NewInt(100000000000000)
	}
	needed := new(big.Int).Add(reserve, gasCost)
	if e = cryptoWalletDollarLimits(intent, q); e != nil {
		return e
	}
	if intent.SellToken == "ETH" {
		needed.Add(needed, amount)
	} else {
		if usdc.Cmp(amount) < 0 {
			return errors.New("Insufficient USDC balance")
		}
	}
	if eth.Cmp(needed) < 0 {
		return errors.New("Keep enough ETH for the configured reserve and network fees")
	}
	return nil
}
func cryptoWalletDollarLimits(intent cryptoTradeIntent, q cryptoWalletQuote) error {
	at, e := time.Parse(time.RFC3339Nano, q.Market.FetchedAt)
	if e != nil || time.Since(at) > time.Minute || time.Until(at) > 5*time.Second {
		return errors.New("Market evidence is stale")
	}
	ep, e := cryptoBoundedDecimal(q.Market.ETHUSD, 18, "1", "1000000")
	if e != nil {
		return e
	}
	up, e := cryptoBoundedDecimal(q.Market.USDCUSD, 18, "0.98", "1.02")
	if e != nil {
		return errors.New("USDC price is outside the supported stability band")
	}
	max, e := cryptoBoundedDecimal(intent.MaxTradeUSD, 6, "0.01", "1000000000")
	if e != nil {
		return e
	}
	budget, e := cryptoBoundedDecimal(intent.BudgetUSD, 6, "0.01", "1000000000")
	if e != nil {
		return e
	}
	amount, e := cryptoUint(intent.SellAmount)
	if e != nil {
		return e
	}
	out, e := cryptoUint(q.BuyAmount)
	if e != nil || out.Sign() == 0 {
		return errors.New("Invalid buy amount")
	}
	eth, e := cryptoUint(q.Snapshot.ETH)
	if e != nil {
		return e
	}
	sellPrice, buyPrice := ep, up
	sellScale, buyScale := int64(1000000000000000000), int64(1000000)
	if intent.SellToken == "USDC" {
		sellPrice, buyPrice = up, ep
		sellScale, buyScale = buyScale, sellScale
	}
	sellUSD := new(big.Rat).Mul(new(big.Rat).SetFrac(amount, big.NewInt(sellScale)), sellPrice)
	buyUSD := new(big.Rat).Mul(new(big.Rat).SetFrac(out, big.NewInt(buyScale)), buyPrice)
	if sellUSD.Cmp(max) > 0 {
		return errors.New("This swap exceeds the trade limit at the current independent USD price")
	}
	low := new(big.Rat).Mul(sellUSD, big.NewRat(98, 100))
	high := new(big.Rat).Mul(sellUSD, big.NewRat(102, 100))
	if buyUSD.Cmp(low) < 0 || buyUSD.Cmp(high) > 0 {
		return errors.New("Pool quote differs too much from the independent market price; no transaction is prepared")
	}
	if intent.SellToken == "USDC" {
		projected := new(big.Rat).Mul(new(big.Rat).SetFrac(new(big.Int).Add(eth, out), big.NewInt(1000000000000000000)), ep)
		if projected.Cmp(budget) > 0 {
			return errors.New("This swap exceeds the ETH exposure budget at the current independent USD price")
		}
	}
	return nil
}
func cryptoWalletTransaction(v cryptoWalletRecord, kind string) (cryptoWalletTx, error) {
	t := cryptoWalletTx{From: v.Quote.WalletAddress, To: cryptoRouter, ChainID: "0x2105", Value: "0x0"}
	var e error
	switch kind {
	case "swap":
		t.Data, e = cryptoSwapData(v.Intent, v.Quote.MinBuyAmount, v.Quote.Deadline)
		if v.Intent.SellToken == "ETH" {
			n, _ := cryptoUint(v.Intent.SellAmount)
			t.Value = "0x" + n.Text(16)
		}
	case "approve", "reset":
		if v.Intent.SellToken != "USDC" {
			return t, errors.New("ETH does not require token approval")
		}
		amount := v.Intent.SellAmount
		if kind == "reset" {
			amount = "0"
		}
		t.To = cryptoUSDC
		t.Data, e = cryptoApproveData(amount)
	default:
		e = errors.New("Unsupported wallet operation")
	}
	return t, e
}
func (a *App) cryptoWalletPrepare(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var in struct {
		QuoteID string `json:"quoteId"`
		Kind    string `json:"kind"`
	}
	if !aiDecode(w, r, &in) {
		return
	}
	s := apphost.Current(r)
	a.wallet.mu.Lock()
	defer a.wallet.mu.Unlock()
	v, e := a.cryptoWalletLoad(s.User.ID, in.QuoteID)
	if e != nil {
		walletProblem(w, e)
		return
	}
	expires, _ := time.Parse(time.RFC3339, v.Quote.ExpiresAt)
	if v.Session != apphost.Hash(s.CSRF) || !time.Now().Before(expires) || !a.cryptoIntentCurrent(s.User.ID, v.Intent) {
		walletProblem(w, errors.New("Wallet review expired or changed; request a fresh quote"))
		return
	}
	if len(v.Prepared) > 0 {
		walletProblem(w, errors.New("This wallet request was already prepared; check its receipt or request a new quote"))
		return
	}
	block, _, e := a.cryptoChainBlock(r.Context())
	if e != nil {
		walletProblem(w, e)
		return
	}
	v.Quote.Snapshot, e = a.cryptoSnapshotAt(r.Context(), v.Quote.WalletAddress, block)
	if e != nil {
		walletProblem(w, e)
		return
	}
	v.Quote.Market, e = a.cryptoMarketRead(r.Context())
	if e != nil {
		walletProblem(w, e)
		return
	}
	if v.Intent.SellToken == "USDC" {
		allowance, e := a.cryptoContractUint(r.Context(), cryptoUSDC, cryptoCallData("allowance(address,address)", cryptoAddressWord(v.Quote.WalletAddress), cryptoAddressWord(cryptoRouter)), block)
		if e != nil {
			walletProblem(w, e)
			return
		}
		amount, _ := cryptoUint(v.Intent.SellAmount)
		valid := in.Kind == "swap" && allowance.Cmp(amount) == 0 || in.Kind == "approve" && allowance.Sign() == 0 || in.Kind == "reset" && allowance.Sign() > 0 && allowance.Cmp(amount) != 0
		if !valid {
			walletProblem(w, errors.New("The token allowance changed; refresh and review it again"))
			return
		}
	}
	tx, e := cryptoWalletTransaction(v, in.Kind)
	if e != nil {
		walletProblem(w, e)
		return
	}
	var gasHex, priceHex string
	if e = a.cryptoRPC(r.Context(), "eth_estimateGas", []any{tx}, &gasHex); e != nil {
		walletProblem(w, errors.New("Base could not simulate this exact transaction; no wallet request was created"))
		return
	}
	if e = a.cryptoRPC(r.Context(), "eth_gasPrice", []any{}, &priceHex); e != nil {
		walletProblem(w, e)
		return
	}
	gas, e := cryptoHexUint(gasHex)
	if e != nil || gas.Sign() == 0 || gas.Cmp(big.NewInt(1000000)) > 0 {
		walletProblem(w, errors.New("Unexpected gas estimate"))
		return
	}
	price, e := cryptoHexUint(priceHex)
	if e != nil || price.Sign() == 0 || price.Cmp(big.NewInt(1000000000000)) > 0 {
		walletProblem(w, errors.New("Unexpected gas price"))
		return
	}
	gas.Mul(gas, big.NewInt(120))
	gas.Div(gas, big.NewInt(100))
	tx.Gas = "0x" + gas.Text(16)
	cost := new(big.Int).Mul(gas, price)
	cost.Mul(cost, big.NewInt(2))
	if e = cryptoWalletLimits(v.Intent, v.Quote, cost); e != nil {
		walletProblem(w, e)
		return
	}
	if !a.cryptoIntentCurrent(s.User.ID, v.Intent) || !time.Now().Before(expires) {
		walletProblem(w, errors.New("Proposal changed or quote expired during validation"))
		return
	}
	v.Prepared[in.Kind] = tx
	if e = a.cryptoWalletSaveCurrent(v, s); e != nil {
		walletProblem(w, e)
		return
	}
	apphost.JSON(w, 200, map[string]any{"transaction": tx, "quoteId": v.Quote.ID, "kind": in.Kind, "deadline": v.Quote.Deadline, "expiresAt": v.Quote.ExpiresAt, "estimatedGasCostWei": cost.String()})
}

// Receipts deliberately remain checkable after pause and quote expiry. They can
// only describe an exact previously prepared transaction, never authorize another.
func (a *App) cryptoWalletReceipt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var in struct {
		QuoteID string `json:"quoteId"`
		Kind    string `json:"kind"`
		Hash    string `json:"hash"`
	}
	if !aiDecode(w, r, &in) {
		return
	}
	if !cryptoWalletHashRE.MatchString(in.Hash) {
		walletProblem(w, errors.New("Enter a valid transaction hash"))
		return
	}
	s := apphost.Current(r)
	a.wallet.mu.Lock()
	defer a.wallet.mu.Unlock()
	v, e := a.cryptoWalletLoad(s.User.ID, in.QuoteID)
	if e != nil {
		walletProblem(w, e)
		return
	}
	expected, ok := v.Prepared[in.Kind]
	if !ok {
		walletProblem(w, errors.New("This operation was not prepared"))
		return
	}
	hash := strings.ToLower(in.Hash)
	if prior := v.Hashes[in.Kind]; prior != "" && prior != hash {
		walletProblem(w, errors.New("A different transaction is already recorded"))
		return
	}
	status, e := a.cryptoVerifyReceipt(r.Context(), expected, hash)
	if e != nil {
		walletProblem(w, e)
		return
	}
	if prior := v.Status[in.Kind]; (prior == "confirmed" || prior == "reverted") && prior != status {
		walletProblem(w, errors.New("A previously safe receipt cannot be downgraded; verify it in the block explorer"))
		return
	}
	v.Hashes[in.Kind] = hash
	v.Status[in.Kind] = status
	if e = a.cryptoWalletSave(v); e != nil {
		walletProblem(w, e)
		return
	}
	if in.Kind == "swap" {
		if e = a.cryptoRecordTransaction(s.User.ID, v.Quote.ProposalID, hash, status); e != nil {
			walletProblem(w, e)
			return
		}
	}
	apphost.JSON(w, 200, map[string]any{"status": status, "hash": hash, "kind": in.Kind, "explorerUrl": "https://basescan.org/tx/" + hash})
}
func (a *App) cryptoVerifyReceipt(ctx context.Context, expected cryptoWalletTx, hash string) (string, error) {
	var chain string
	if e := a.cryptoRPC(ctx, "eth_chainId", []any{}, &chain); e != nil {
		return "", e
	}
	if chain != "0x2105" {
		return "", errors.New("Unexpected receipt chain")
	}
	var tx *struct {
		Hash      string `json:"hash"`
		From      string `json:"from"`
		To        string `json:"to"`
		Input     string `json:"input"`
		Value     string `json:"value"`
		ChainID   string `json:"chainId"`
		BlockHash string `json:"blockHash"`
	}
	if e := a.cryptoRPC(ctx, "eth_getTransactionByHash", []any{hash}, &tx); e != nil {
		return "", e
	}
	if tx == nil {
		return "unknown", nil
	}
	value, e := cryptoHexUint(tx.Value)
	want, _ := cryptoHexUint(expected.Value)
	if e != nil || value.Cmp(want) != 0 || !strings.EqualFold(tx.Hash, hash) || !strings.EqualFold(tx.From, expected.From) || !strings.EqualFold(tx.To, expected.To) || !strings.EqualFold(tx.Input, expected.Data) || tx.ChainID != "0x2105" {
		return "", errors.New("Transaction does not match the reviewed wallet operation")
	}
	var receipt *struct {
		TransactionHash string `json:"transactionHash"`
		BlockHash       string `json:"blockHash"`
		BlockNumber     string `json:"blockNumber"`
		Status          string `json:"status"`
		From            string `json:"from"`
		To              string `json:"to"`
	}
	if e = a.cryptoRPC(ctx, "eth_getTransactionReceipt", []any{hash}, &receipt); e != nil {
		return "", e
	}
	if receipt == nil {
		return "submitted", nil
	}
	if !strings.EqualFold(receipt.TransactionHash, hash) || !strings.EqualFold(receipt.From, expected.From) || !strings.EqualFold(receipt.To, expected.To) || receipt.BlockHash != tx.BlockHash || !cryptoWalletHashRE.MatchString(receipt.BlockHash) {
		return "", errors.New("Receipt does not match the transaction")
	}
	var safe struct {
		Number string `json:"number"`
	}
	if e = a.cryptoRPC(ctx, "eth_getBlockByNumber", []any{"safe", false}, &safe); e != nil {
		return "", e
	}
	safeN, e := cryptoHexUint(safe.Number)
	n, e2 := cryptoHexUint(receipt.BlockNumber)
	if e != nil || e2 != nil {
		return "", errors.New("Invalid confirmation block")
	}
	var canonical struct {
		Hash string `json:"hash"`
	}
	if e = a.cryptoRPC(ctx, "eth_getBlockByNumber", []any{receipt.BlockNumber, false}, &canonical); e != nil {
		return "", e
	}
	if canonical.Hash != receipt.BlockHash || n.Cmp(safeN) > 0 {
		return "submitted", nil
	}
	switch receipt.Status {
	case "0x1":
		return "confirmed", nil
	case "0x0":
		return "reverted", nil
	default:
		return "", errors.New("Invalid transaction status")
	}
}

func (a *App) cryptoWalletReviews(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user := apphost.Current(r).User.ID
	records, e := a.cryptoWalletRecords(user)
	if e != nil {
		walletProblem(w, errors.New("Wallet history unavailable"))
		return
	}
	items := []map[string]any{}
	for _, v := range records {
		id := v.Quote.ID
		for kind := range v.Prepared {
			items = append(items, map[string]any{"quoteId": id, "proposalId": v.Quote.ProposalID, "kind": kind, "hash": v.Hashes[kind], "status": v.Status[kind], "deadline": v.Quote.Deadline})
		}
	}
	apphost.JSON(w, 200, map[string]any{"reviews": items})
}
