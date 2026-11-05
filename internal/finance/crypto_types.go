package finance

import "time"

const cryptoChainID = 8453
const cryptoWETHAddress = "0x4200000000000000000000000000000000000006"
const cryptoUSDCAddress = "0x833589fcd6edb6e08f4c7c32d4f71b54bda02913"

type cryptoTradeIntent struct {
	ProposalID, WalletAddress, SellToken, BuyToken, SellAmount string
	BudgetUSD, MaxTradeUSD, ReserveETH                         string
	Revision                                                   int64
	ChainID, SlippageBPS                                       int
	ExpiresAt                                                  time.Time
}

type cryptoWalletSnapshot struct {
	Address     string `json:"address"`
	ChainID     int    `json:"chainId"`
	BlockNumber string `json:"blockNumber"`
	FetchedAt   string `json:"fetchedAt"`
	ETH         string `json:"eth"`
	USDC        string `json:"usdc"`
}

type cryptoConfig struct {
	Enabled           bool   `json:"enabled"`
	WalletAddress     string `json:"walletAddress"`
	ChainID           int    `json:"chainId"`
	BudgetUSD         string `json:"budgetUsd"`
	MaxTradeUSD       string `json:"maxTradeUsd"`
	ReserveETH        string `json:"reserveEth"`
	SlippageBPS       int    `json:"slippageBps"`
	IntervalMinutes   int    `json:"intervalMinutes"`
	MaxAnalysesPerDay int    `json:"maxAnalysesPerDay"`
	MaxSpendUSD       string `json:"maxSpendUsd"`
	Objective         string `json:"objective"`
}

type cryptoMarket struct {
	ChainID      int    `json:"chainId"`
	PairAddress  string `json:"pairAddress"`
	SourceURL    string `json:"sourceUrl"`
	FetchedAt    string `json:"fetchedAt"`
	ETHUSD       string `json:"ethUsd"`
	USDCUSD      string `json:"usdcUsd"`
	LiquidityUSD string `json:"liquidityUsd"`
}

type cryptoEvidence struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Snippet     string `json:"snippet"`
	PublishedAt string `json:"publishedAt"`
	FetchedAt   string `json:"fetchedAt"`
}

type cryptoProposal struct {
	ID               string   `json:"id"`
	Revision         int64    `json:"revision"`
	CreatedAt        string   `json:"createdAt"`
	ExpiresAt        string   `json:"expiresAt"`
	Status           string   `json:"status"`
	SellToken        string   `json:"sellToken"`
	BuyToken         string   `json:"buyToken"`
	SellAmount       string   `json:"sellAmount"`
	EstimatedSellUSD string   `json:"estimatedSellUsd"`
	Rationale        string   `json:"rationale"`
	Risks            []string `json:"risks"`
	EvidenceIDs      []string `json:"evidenceIds"`
	TransactionHash  string   `json:"transactionHash,omitempty"`
}

type cryptoUsage struct {
	Day               string `json:"day"`
	Analyses          int    `json:"analyses"`
	ReservedSpendUSD  string `json:"reservedSpendUsd"`
	MaxSpendUSD       string `json:"maxSpendUsd"`
	RemainingAnalyses int    `json:"remainingAnalyses"`
}

type cryptoState struct {
	Revision     int64                 `json:"revision"`
	Config       cryptoConfig          `json:"config"`
	Status       string                `json:"status"`
	LastRun      string                `json:"lastRun"`
	NextRun      string                `json:"nextRun"`
	LastError    string                `json:"lastError"`
	LastAnalysis string                `json:"lastAnalysis"`
	LastRisks    []string              `json:"lastRisks"`
	Market       *cryptoMarket         `json:"market"`
	Wallet       *cryptoWalletSnapshot `json:"wallet"`
	Evidence     []cryptoEvidence      `json:"evidence"`
	Proposals    []cryptoProposal      `json:"proposals"`
	Usage        cryptoUsage           `json:"usage"`
}
