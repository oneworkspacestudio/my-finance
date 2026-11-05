package finance

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/sha3"
)

// Addresses/ABI: developers.uniswap.org/docs/protocols/v3/deployments/v3-base-deployments
// and Uniswap/swap-router-contracts IV3SwapRouter + MulticallExtended. USDC is
// Circle's native Base USDC, not a bridged token. This is one direct v3 0.05% pool.
const cryptoBaseRPC = "https://mainnet.base.org"
const cryptoRouter = "0x2626664c2603336e57b271c5c0b26f421741e481"
const cryptoQuoter = "0x3d4e44eb1374240ce5f1b871ab261cd16335b76a"
const cryptoWETH = "0x4200000000000000000000000000000000000006"
const cryptoUSDC = "0x833589fcd6edb6e08f4c7c32d4f71b54bda02913"
const cryptoPoolFee = 500

var cryptoWalletAddressRE = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
var cryptoWalletHashRE = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)
var cryptoUintRE = regexp.MustCompile(`^(0|[1-9][0-9]{0,77})$`)
var cryptoHexUintRE = regexp.MustCompile(`^0x[0-9a-fA-F]{1,64}$`)

func cryptoWalletAddress(s string) bool {
	return cryptoWalletAddressRE.MatchString(s) && !strings.EqualFold(s, "0x0000000000000000000000000000000000000000")
}
func cryptoUint(s string) (*big.Int, error) {
	n, ok := new(big.Int).SetString(s, 10)
	if !cryptoUintRE.MatchString(s) || !ok || n.BitLen() > 256 {
		return nil, errors.New("Invalid unsigned amount")
	}
	return n, nil
}
func cryptoHexUint(s string) (*big.Int, error) {
	if !cryptoHexUintRE.MatchString(s) {
		return nil, errors.New("Invalid chain quantity")
	}
	n, ok := new(big.Int).SetString(s[2:], 16)
	if !ok || n.Sign() < 0 {
		return nil, errors.New("Invalid chain quantity")
	}
	return n, nil
}
func cryptoWord(n *big.Int) string { return strings.Repeat("0", 64-len(n.Text(16))) + n.Text(16) }
func cryptoIntWord(n int64) string { return cryptoWord(big.NewInt(n)) }
func cryptoAddressWord(a string) string {
	return strings.Repeat("0", 24) + strings.ToLower(strings.TrimPrefix(a, "0x"))
}
func cryptoSelector(signature string) string {
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte(signature))
	return hex.EncodeToString(h.Sum(nil)[:4])
}
func cryptoCallData(signature string, words ...string) string {
	return "0x" + cryptoSelector(signature) + strings.Join(words, "")
}
func cryptoBytesArray(calls []string) string {
	head, tail := "", ""
	offset := len(calls) * 32
	for _, call := range calls {
		b := strings.TrimPrefix(call, "0x")
		head += cryptoIntWord(int64(offset))
		padding := (64 - len(b)%64) % 64
		entry := cryptoIntWord(int64(len(b)/2)) + b + strings.Repeat("0", padding)
		tail += entry
		offset += len(entry) / 2
	}
	return cryptoIntWord(int64(len(calls))) + head + tail
}
func cryptoSwapData(intent cryptoTradeIntent, minimum string, deadline int64) (string, error) {
	amount, e := cryptoUint(intent.SellAmount)
	if e != nil || amount.Sign() == 0 || !cryptoWalletAddress(intent.WalletAddress) || intent.ChainID != 8453 {
		return "", errors.New("Invalid swap intent")
	}
	min, e := cryptoUint(minimum)
	if e != nil || min.Sign() == 0 || deadline <= 0 {
		return "", errors.New("Invalid swap minimum/deadline")
	}
	in, out, recipient := cryptoWETH, cryptoUSDC, strings.ToLower(intent.WalletAddress)
	if intent.SellToken == "USDC" && intent.BuyToken == "ETH" {
		in, out, recipient = cryptoUSDC, cryptoWETH, cryptoRouter
	} else if intent.SellToken != "ETH" || intent.BuyToken != "USDC" {
		return "", errors.New("Unsupported token pair")
	}
	swap := cryptoCallData("exactInputSingle((address,address,uint24,address,uint256,uint256,uint160))", cryptoAddressWord(in), cryptoAddressWord(out), cryptoIntWord(cryptoPoolFee), cryptoAddressWord(recipient), cryptoWord(amount), cryptoWord(min), cryptoIntWord(0))
	calls := []string{swap}
	if intent.SellToken == "USDC" {
		calls = append(calls, cryptoCallData("unwrapWETH9(uint256,address)", cryptoWord(min), cryptoAddressWord(intent.WalletAddress)))
	} else {
		calls = append(calls, cryptoCallData("refundETH()"))
	}
	return cryptoCallData("multicall(uint256,bytes[])", cryptoIntWord(deadline), cryptoIntWord(64), cryptoBytesArray(calls)), nil
}
func cryptoApproveData(amount string) (string, error) {
	n, e := cryptoUint(amount)
	if e != nil {
		return "", e
	}
	return cryptoCallData("approve(address,uint256)", cryptoAddressWord(cryptoRouter), cryptoWord(n)), nil
}
func cryptoQuoteData(intent cryptoTradeIntent) (string, error) {
	n, e := cryptoUint(intent.SellAmount)
	if e != nil || n.Sign() == 0 {
		return "", errors.New("Invalid sell amount")
	}
	in, out := cryptoWETH, cryptoUSDC
	if intent.SellToken == "USDC" && intent.BuyToken == "ETH" {
		in, out = out, in
	} else if intent.SellToken != "ETH" || intent.BuyToken != "USDC" {
		return "", errors.New("Unsupported pair")
	}
	return cryptoCallData("quoteExactInputSingle((address,address,uint256,uint24,uint160))", cryptoAddressWord(in), cryptoAddressWord(out), cryptoWord(n), cryptoIntWord(cryptoPoolFee), cryptoIntWord(0)), nil
}
func cryptoRPCClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil
	return &http.Client{Transport: t, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func (a *App) cryptoRPC(ctx context.Context, method string, params any, out any) error {
	switch method {
	case "eth_chainId", "eth_blockNumber", "eth_getBlockByNumber", "eth_getBalance", "eth_call", "eth_estimateGas", "eth_gasPrice", "eth_getTransactionByHash", "eth_getTransactionReceipt":
	default:
		return errors.New("Unsupported read-only chain method")
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req, e := http.NewRequestWithContext(ctx, "POST", cryptoBaseRPC, bytes.NewReader(body))
	if e != nil {
		return errors.New("Chain request unavailable")
	}
	req.Header.Set("Content-Type", "application/json")
	client := a.wallet.client
	resp, e := client.Do(req)
	if e != nil {
		return errors.New("Base read-only RPC is unavailable; try later")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("Base read-only RPC is unavailable; try later")
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 128<<10))
	if e != nil || len(raw) >= 128<<10 {
		return errors.New("Invalid chain response")
	}
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.JSONRPC != "2.0" || envelope.ID != 1 || len(envelope.Result) == 0 || (len(envelope.Error) > 0 && string(envelope.Error) != "null") {
		return errors.New("Base could not validate this operation")
	}
	if json.Unmarshal(envelope.Result, out) != nil {
		return errors.New("Invalid chain response")
	}
	return nil
}
func (a *App) cryptoChainBlock(ctx context.Context) (string, int64, error) {
	var chain, block string
	if e := a.cryptoRPC(ctx, "eth_chainId", []any{}, &chain); e != nil {
		return "", 0, e
	}
	if chain != "0x2105" {
		return "", 0, errors.New("RPC returned a different chain")
	}
	if e := a.cryptoRPC(ctx, "eth_blockNumber", []any{}, &block); e != nil {
		return "", 0, e
	}
	if _, e := cryptoHexUint(block); e != nil {
		return "", 0, e
	}
	var header struct {
		Number    string `json:"number"`
		Timestamp string `json:"timestamp"`
	}
	if e := a.cryptoRPC(ctx, "eth_getBlockByNumber", []any{block, false}, &header); e != nil {
		return "", 0, e
	}
	n, e := cryptoHexUint(header.Timestamp)
	if e != nil || !n.IsInt64() || header.Number != block {
		return "", 0, errors.New("Invalid Base block")
	}
	if delta := time.Now().Unix() - n.Int64(); delta > 60 || delta < -15 {
		return "", 0, errors.New("Base block is stale; refresh later")
	}
	return block, n.Int64(), nil
}
func (a *App) cryptoContractUint(ctx context.Context, to, data, block string) (*big.Int, error) {
	var result string
	if e := a.cryptoRPC(ctx, "eth_call", []any{map[string]string{"to": to, "data": data}, block}, &result); e != nil {
		return nil, e
	}
	if len(result) != 66 {
		return nil, errors.New("Invalid token balance response")
	}
	return cryptoHexUint(result)
}
func (a *App) cryptoSnapshotAt(ctx context.Context, address, block string) (cryptoWalletSnapshot, error) {
	var v cryptoWalletSnapshot
	var eth string
	if !cryptoWalletAddress(address) {
		return v, errors.New("Enter a valid Base wallet address")
	}
	if e := a.cryptoRPC(ctx, "eth_getBalance", []any{address, block}, &eth); e != nil {
		return v, e
	}
	n, e := cryptoHexUint(eth)
	if e != nil {
		return v, e
	}
	usdc, e := a.cryptoContractUint(ctx, cryptoUSDC, cryptoCallData("balanceOf(address)", cryptoAddressWord(address)), block)
	if e != nil {
		return v, e
	}
	v = cryptoWalletSnapshot{Address: strings.ToLower(address), ChainID: 8453, BlockNumber: block, FetchedAt: time.Now().UTC().Format(time.RFC3339), ETH: n.String(), USDC: usdc.String()}
	return v, nil
}
func (a *App) cryptoWalletSnapshot(ctx context.Context, address string) (cryptoWalletSnapshot, error) {
	block, _, e := a.cryptoChainBlock(ctx)
	if e != nil {
		return cryptoWalletSnapshot{}, e
	}
	return a.cryptoSnapshotAt(ctx, address, block)
}
func cryptoDecimalUnits(s string, decimals int) (*big.Int, error) {
	if len(s) > 90 || !regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`).MatchString(s) {
		return nil, errors.New("Invalid decimal amount")
	}
	p := strings.Split(s, ".")
	f := ""
	if len(p) == 2 {
		f = p[1]
	}
	if len(f) > decimals {
		return nil, errors.New("Too many decimal places")
	}
	digits := strings.TrimLeft(p[0]+f+strings.Repeat("0", decimals-len(f)), "0")
	if digits == "" {
		digits = "0"
	}
	return cryptoUint(digits)
}
