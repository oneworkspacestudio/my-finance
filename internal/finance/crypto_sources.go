package finance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func cryptoResponse(client *http.Client, request *http.Request, limit int64) ([]byte, error) {
	response, e := client.Do(request)
	if e != nil {
		return nil, errors.New("The evidence or analysis request did not complete; no paid retry was made")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		if response.StatusCode == 401 || response.StatusCode == 403 {
			return nil, errors.New("A provider rejected access; check the DeepSeek key and permissions")
		}
		if response.StatusCode == 402 || response.StatusCode == 429 {
			return nil, errors.New("A provider credit or rate limit was reached")
		}
		return nil, errors.New("A provider could not supply the requested evidence or analysis")
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if e != nil || int64(len(raw)) > limit || !utf8.Valid(raw) {
		return nil, errors.New("A provider returned oversized or incomplete data")
	}
	return raw, nil
}

type cryptoDexPair struct {
	ChainID     string `json:"chainId"`
	PairAddress string `json:"pairAddress"`
	BaseToken   struct {
		Address string `json:"address"`
	} `json:"baseToken"`
	QuoteToken struct {
		Address string `json:"address"`
	} `json:"quoteToken"`
	PriceUSD    string `json:"priceUsd"`
	PriceNative string `json:"priceNative"`
	Liquidity   struct {
		USD json.Number `json:"usd"`
	} `json:"liquidity"`
}

func (a *App) cryptoMarketRead(ctx context.Context) (cryptoMarket, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.dexscreener.com/token-pairs/v1/base/"+cryptoWETHAddress, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "MyFinance-CryptoMonitor/1.0")
	raw, e := cryptoResponse(a.crypto.client, req, 256<<10)
	if e != nil {
		return cryptoMarket{}, e
	}
	var pairs []cryptoDexPair
	if json.Unmarshal(raw, &pairs) != nil || len(pairs) > 200 {
		return cryptoMarket{}, errors.New("The market source returned invalid pool data")
	}
	var selected cryptoMarket
	best := new(big.Rat)
	for _, pair := range pairs {
		if pair.ChainID != "base" || !strings.EqualFold(pair.BaseToken.Address, cryptoWETHAddress) || !strings.EqualFold(pair.QuoteToken.Address, cryptoUSDCAddress) || !cryptoAddress.MatchString(pair.PairAddress) {
			continue
		}
		price, e := cryptoBoundedDecimal(pair.PriceUSD, 18, "1", "1000000")
		if e != nil {
			continue
		}
		native, e := cryptoBoundedDecimal(pair.PriceNative, 18, "0.01", "10000000")
		if e != nil {
			continue
		}
		liquidity, e := cryptoBoundedDecimal(pair.Liquidity.USD.String(), 18, "100000", "1000000000000")
		if e != nil {
			continue
		}
		usdc := new(big.Rat).Quo(price, native)
		if usdc.Cmp(big.NewRat(98, 100)) < 0 || usdc.Cmp(big.NewRat(102, 100)) > 0 {
			continue
		}
		if liquidity.Cmp(best) < 0 || (liquidity.Cmp(best) == 0 && selected.PairAddress != "" && strings.ToLower(pair.PairAddress) >= selected.PairAddress) {
			continue
		}
		best = liquidity
		address := strings.ToLower(pair.PairAddress)
		selected = cryptoMarket{ChainID: cryptoChainID, PairAddress: address, SourceURL: "https://dexscreener.com/base/" + address, ETHUSD: price.FloatString(18), USDCUSD: usdc.FloatString(18), LiquidityUSD: liquidity.FloatString(2)}
	}
	if selected.PairAddress == "" {
		return selected, errors.New("No sufficiently liquid matching Base ETH/USDC market is available")
	}
	selected.FetchedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return selected, nil
}

func cryptoSourceText(s string, limit int) string {
	if !utf8.ValidString(s) {
		return ""
	}
	s = strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)), " ")
	if len(s) > limit {
		s = s[:limit]
		for !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
	}
	return s
}

func cryptoSourceURL(raw string) (string, bool) {
	if len(raw) > 512 {
		return "", false
	}
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Opaque != "" || u.Fragment != "" || net.ParseIP(u.Hostname()) != nil {
		return "", false
	}
	h := strings.ToLower(u.Hostname())
	if !strings.Contains(h, ".") || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".internal") || strings.HasSuffix(h, ".localhost") || strings.ContainsAny(raw, "\r\n\t") {
		return "", false
	}
	return u.String(), true
}
func cryptoSocialURL(raw string) bool {
	u, _ := url.Parse(raw)
	h := strings.ToLower(u.Hostname())
	for _, site := range []string{"x.com", "twitter.com", "bsky.app", "reddit.com"} {
		if h == site || strings.HasSuffix(h, "."+site) {
			return true
		}
	}
	return false
}
func cryptoEvidenceID(kind, link string) string {
	h := sha256.Sum256([]byte(kind + "|" + link))
	return hex.EncodeToString(h[:8])
}

func (a *App) cryptoSearch(ctx context.Context, key, kind string, now time.Time) ([]cryptoEvidence, error) {
	query := "ETH USDC Base crypto news after:" + now.AddDate(0, 0, -7).Format("2006-01-02")
	if kind == "indexed-social" {
		query = "ETH USDC Base site:bsky.app OR site:x.com OR site:reddit.com after:" + now.AddDate(0, 0, -7).Format("2006-01-02")
	}
	body, _ := json.Marshal(map[string]any{"query": query, "count": 3, "need_content": false})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.deepseek.com/v1/tools/search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	raw, e := cryptoResponse(a.ai.client, req, 96<<10)
	if e != nil {
		return nil, e
	}
	var response struct {
		Results []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Snippet string  `json:"snippet"`
			Date    *string `json:"date"`
		} `json:"results"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Results == nil || len(response.Results) > 50 {
		return nil, errors.New("Search returned invalid evidence")
	}
	items := []cryptoEvidence{}
	seen := map[string]bool{}
	for _, result := range response.Results {
		link, ok := cryptoSourceURL(result.URL)
		if !ok || seen[link] || (kind == "indexed-social") != cryptoSocialURL(link) {
			continue
		}
		title := cryptoSourceText(result.Title, 180)
		if title == "" {
			continue
		}
		published := ""
		if result.Date != nil && *result.Date != "" {
			at, e := time.Parse(time.RFC3339, *result.Date)
			if e != nil || at.After(now.Add(5*time.Minute)) || at.Before(now.AddDate(0, 0, -7)) {
				continue
			}
			published = at.UTC().Format(time.RFC3339)
		}
		seen[link] = true
		items = append(items, cryptoEvidence{ID: cryptoEvidenceID(kind, link), Kind: kind, Title: title, URL: link, Snippet: cryptoSourceText(result.Snippet, 360), PublishedAt: published, FetchedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		if len(items) == 3 {
			break
		}
	}
	return items, nil
}
