package finance

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type walletTestTransport func(*http.Request) (*http.Response, error)

func (f walletTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func walletTestIntent() cryptoTradeIntent {
	return cryptoTradeIntent{ProposalID: "fictional-proposal", WalletAddress: "0x1111111111111111111111111111111111111111", ChainID: 8453, SellToken: "ETH", BuyToken: "USDC", SellAmount: "100000000000000000", MaxTradeUSD: "201", BudgetUSD: "5000", ReserveETH: "0.001", SlippageBPS: 50, Revision: 1, ExpiresAt: time.Now().Add(10 * time.Minute)}
}
func walletTestQuote() cryptoWalletQuote {
	return cryptoWalletQuote{ID: "testquote", ProposalID: "fictional-proposal", WalletAddress: walletTestIntent().WalletAddress, ChainID: 8453, SellToken: "ETH", BuyToken: "USDC", SellAmount: "100000000000000000", BuyAmount: "200000000", MinBuyAmount: "199000000", Deadline: 1800000000, Snapshot: cryptoWalletSnapshot{ETH: "1000000000000000000", USDC: "1000000000"}, Market: cryptoMarket{ETHUSD: "2000", USDCUSD: "1", FetchedAt: time.Now().UTC().Format(time.RFC3339Nano)}}
}
func TestCryptoWalletSelectorsAndEncoding(t *testing.T) {
	for sig, want := range map[string]string{"exactInputSingle((address,address,uint24,address,uint256,uint256,uint160))": "04e45aaf", "multicall(uint256,bytes[])": "5ae401dc", "unwrapWETH9(uint256,address)": "49404b7c", "refundETH()": "12210e8a", "approve(address,uint256)": "095ea7b3", "quoteExactInputSingle((address,address,uint256,uint24,uint160))": "c6a5026a"} {
		if got := cryptoSelector(sig); got != want {
			t.Fatalf("selector %s = %s, want%s", sig, got, want)
		}
	}
	intent := walletTestIntent()
	data, e := cryptoSwapData(intent, "199000000", 1800000000)
	if e != nil {
		t.Fatal(e)
	}
	w := func(offset int) string { return data[10+offset*64 : 10+(offset+1)*64] }
	if w(0) != cryptoIntWord(1800000000) || w(1) != cryptoIntWord(64) || w(2) != cryptoIntWord(2) || w(3) != cryptoIntWord(64) || w(4) != cryptoIntWord(352) {
		t.Fatal("multicall ABI offset/deadline mismatch")
	}
	if !strings.Contains(data, cryptoAddressWord(intent.WalletAddress)) || !strings.Contains(data, "12210e8a") {
		t.Fatal("ETH swap recipient/refund missing")
	}
	intent.SellToken, intent.BuyToken, intent.SellAmount = "USDC", "ETH", "200000000"
	data, e = cryptoSwapData(intent, "99500000000000000", 1800000000)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(data, "49404b7c"+cryptoWord(big.NewInt(99500000000000000))+cryptoAddressWord(intent.WalletAddress)) {
		t.Fatal("USDC swap must atomically unwrap minimum to wallet")
	}
	if strings.Contains(data, "12210e8a") {
		t.Fatal("unexpected refund")
	}
	intent.SellAmount = "0"
	if _, e = cryptoSwapData(intent, "1", 1); e == nil {
		t.Fatal("zero amount enables contract balance mode")
	}
	intent.SellAmount = "1"
	intent.BuyToken = "WETH"
	if _, e = cryptoSwapData(intent, "1", 1); e == nil {
		t.Fatal("arbitrary pair accepted")
	}
}
func TestCryptoWalletExactApprovalsAndNumericBounds(t *testing.T) {
	for _, s := range []string{"0x+1", "0x-1", "0x1_0", "0x", "0x" + strings.Repeat("f", 65)} {
		if _, e := cryptoHexUint(s); e == nil {
			t.Fatal("invalid chain amount", s)
		}
	}
	data, e := cryptoApproveData("200000000")
	if e != nil || data != "0x095ea7b3"+cryptoAddressWord(cryptoRouter)+cryptoIntWord(200000000) {
		t.Fatal("approval amount/spender")
	}
	for _, s := range []string{"-1", "1e18", "01", "+1", strings.Repeat("9", 79)} {
		if _, e := cryptoUint(s); e == nil {
			t.Fatal("invalid amount", s)
		}
	}
	n, e := cryptoDecimalUnits("0.000000000000000001", 18)
	if e != nil || n.String() != "1" {
		t.Fatal("precision")
	}
	n, e = cryptoDecimalUnits("0", 18)
	if e != nil || n.Sign() != 0 {
		t.Fatal("zero")
	}
	if _, e = cryptoDecimalUnits("0.0000001", 6); e == nil {
		t.Fatal("silently rounded")
	}
}
func TestCryptoWalletLimits(t *testing.T) {
	intent, q := walletTestIntent(), walletTestQuote()
	if e := cryptoWalletLimits(intent, q, big.NewInt(0)); e != nil {
		t.Fatal(e)
	}
	intent.MaxTradeUSD = "199.999999"
	if e := cryptoWalletLimits(intent, q, big.NewInt(0)); e == nil {
		t.Fatal("trade cap ignored")
	}
	intent = walletTestIntent()
	q.Market.USDCUSD = "0.5"
	if e := cryptoWalletLimits(intent, q, big.NewInt(0)); e == nil {
		t.Fatal("depeg ignored")
	}
	q = walletTestQuote()
	q.BuyAmount = "100000000"
	if e := cryptoWalletLimits(intent, q, big.NewInt(0)); e == nil {
		t.Fatal("price deviation ignored")
	}
	q = walletTestQuote()
	q.Snapshot.ETH = "100000000000000000"
	if e := cryptoWalletLimits(intent, q, big.NewInt(0)); e == nil {
		t.Fatal("gas reserve ignored")
	}
	q = walletTestQuote()
	q.Market.FetchedAt = time.Now().Add(-2 * time.Minute).Format(time.RFC3339)
	if e := cryptoWalletLimits(intent, q, big.NewInt(0)); e == nil {
		t.Fatal("stale market accepted")
	}
	intent.SellToken, intent.BuyToken, intent.SellAmount = "USDC", "ETH", "200000000"
	q = walletTestQuote()
	q.BuyAmount = "100000000000000000"
	q.MinBuyAmount = "99500000000000000"
	intent.BudgetUSD = "2199.99"
	if e := cryptoWalletLimits(intent, q, big.NewInt(0)); e == nil {
		t.Fatal("exposure cap ignored")
	}
	intent.BudgetUSD = "2200"
	if e := cryptoWalletLimits(intent, q, big.NewInt(0)); e != nil {
		t.Fatal(e)
	}
}
func walletRPCFixture(t *testing.T, respond func(string, []json.RawMessage) any) *App {
	t.Helper()
	return &App{wallet: &cryptoWalletRuntime{client: &http.Client{Transport: walletTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != cryptoBaseRPC || r.Method != "POST" || r.Header.Get("Authorization") != "" {
			t.Fatal("unexpected outbound request")
		}
		var req struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			t.Fatal("request body")
		}
		result := respond(req.Method, req.Params)
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(raw)))}, nil
	})}}}
}
func TestCryptoWalletRPCNeverBroadcasts(t *testing.T) {
	calls := 0
	a := walletRPCFixture(t, func(method string, _ []json.RawMessage) any { calls++; return "0x2105" })
	for _, method := range []string{"eth_sendRawTransaction", "eth_sendTransaction", "eth_sign", "personal_sign", "wallet_sendCalls"} {
		var result any
		if e := a.cryptoRPC(context.Background(), method, []any{}, &result); e == nil {
			t.Fatal("write method allowed", method)
		}
	}
	if calls != 0 {
		t.Fatal("forbidden network call")
	}
	var result string
	if e := a.cryptoRPC(context.Background(), "eth_chainId", []any{}, &result); e != nil || result != "0x2105" {
		t.Fatal(e)
	}
	client := cryptoRPCClient()
	if client.Transport.(*http.Transport).Proxy != nil || client.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatal("proxy/redirect enabled")
	}
}
func TestCryptoWalletReceiptRequiresExactIntentAndSafeCanonicalBlock(t *testing.T) {
	intent := walletTestIntent()
	q := walletTestQuote()
	expected, e := cryptoWalletTransaction(cryptoWalletRecord{Intent: intent, Quote: q}, "swap")
	if e != nil {
		t.Fatal(e)
	}
	hash := "0x" + strings.Repeat("a", 64)
	blockHash := "0x" + strings.Repeat("b", 64)
	mutate := ""
	a := walletRPCFixture(t, func(method string, params []json.RawMessage) any {
		switch method {
		case "eth_chainId":
			return "0x2105"
		case "eth_getTransactionByHash":
			if mutate == "missing" {
				return nil
			}
			input := expected.Data
			if mutate == "calldata" {
				input += "00"
			}
			return map[string]string{"hash": hash, "from": expected.From, "to": expected.To, "input": input, "value": expected.Value, "chainId": "0x2105", "blockHash": blockHash}
		case "eth_getTransactionReceipt":
			if mutate == "pending" {
				return nil
			}
			status := "0x1"
			if mutate == "reverted" {
				status = "0x0"
			}
			return map[string]string{"transactionHash": hash, "from": expected.From, "to": expected.To, "blockHash": blockHash, "blockNumber": "0x10", "status": status}
		case "eth_getBlockByNumber":
			if string(params[0]) == `"safe"` {
				if mutate == "unsafe" {
					return map[string]string{"number": "0xf"}
				}
				return map[string]string{"number": "0x10"}
			}
			if mutate == "reorg" {
				return map[string]string{"hash": "0x" + strings.Repeat("c", 64)}
			}
			return map[string]string{"hash": blockHash}
		default:
			t.Fatal(method)
		}
		return nil
	})
	for change, want := range map[string]string{"": "confirmed", "reverted": "reverted", "pending": "submitted", "unsafe": "submitted", "reorg": "submitted", "missing": "unknown", "calldata": "error"} {
		mutate = change
		got, e := a.cryptoVerifyReceipt(context.Background(), expected, hash)
		if want == "error" {
			if e == nil {
				t.Fatal("different calldata accepted")
			}
		} else if e != nil || got != want {
			t.Fatalf("%s got%s %v", change, got, e)
		}
	}
}
func TestCryptoWalletPublicReadOnlySmoke(t *testing.T) {
	if os.Getenv("FINANCE_LIVE_READ_ONLY") != "1" {
		t.Skip("explicit public read-only smoke only")
	}
	a := &App{wallet: &cryptoWalletRuntime{client: cryptoRPCClient()}}
	block, _, e := a.cryptoChainBlock(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	intent := walletTestIntent()
	data, e := cryptoQuoteData(intent)
	if e != nil {
		t.Fatal(e)
	}
	var result string
	if e = a.cryptoRPC(context.Background(), "eth_call", []any{map[string]string{"to": cryptoQuoter, "data": data}, block}, &result); e != nil {
		t.Fatal(e)
	}
	if len(result) != 258 {
		t.Fatal("unexpected public quote shape")
	}
	n, e := cryptoHexUint(result[:66])
	if e != nil || n.Sign() == 0 {
		t.Fatal("no public quote")
	}
	t.Log("Public Base chain/block and direct ETH/USDC on-chain quotation passed. No address balances read, no transaction signed or broadcast.")
}
