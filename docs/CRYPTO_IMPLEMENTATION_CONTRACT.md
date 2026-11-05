# Crypto monitoring and human execution contract

Implementation target: Finance 6. Base (chain 8453), ETH/USDC, DeepSeek and Web
Push and Resend email are the initial integrations. Continuous means the Workspace host remains online;
the browser may be closed. No server signing, private keys, automated transaction
broadcast, session keys, standing trade permissions or notification quick-execute.
The human reviews each proposal, then separately signs each transaction in a wallet.

## Ownership

- Backend agent: `crypto_monitor*.go`, `crypto_model*.go`, `crypto_sources*.go`,
  their tests and shared `crypto_types.go`. Migration 7, monitor routes and loop.
- Wallet agent: `crypto_wallet*.go`, its tests and `ui/crypto-wallet.js` plus tests.
  Migration 8. Deterministic Base on-chain quotes/transactions; no opaque LLM calldata.
- UI agent: `ui/crypto.js`, `ui/index.html`, `ui/style.css`, `ui/app.js`, UI tests.
- Root: server lifecycle/wiring, migrations latest version, Web Push migration 9,
  Resend email migration 10,
  push worker/manifest, dependencies, integration/docs/release/delivery.

## Monitor API

Base path `/api/finance/crypto`. Every private route uses apphost Require and the
existing Origin/CSRF/expected-account guards. No-store responses.

- GET `/api/finance/crypto`: `{configured,state,deepseekConfigured}`. State contains
  `revision`, `config`, `status`, `lastRun`, `nextRun`, `lastError`, `market`,
  `evidence`, `proposals`, `usage`. Timestamps are RFC3339 UTC. Default zero/empty
  state is explicit, never fictional market/portfolio data.
- POST `/settings`: `{revision,consent,config}`. Config: `enabled`, `walletAddress`,
  `chainId:8453`, `budgetUsd` decimal string, `maxTradeUsd` decimal string,
  `reserveEth` decimal string, `slippageBps` integer, `intervalMinutes` integer,
  `maxAnalysesPerDay` integer, `maxSpendUsd` decimal string, `objective` plain text.
  Enabling requires explicit background/provider-sharing/billing consent and an
  existing DeepSeek key. Existing AI settings API owns that key. No other API key
  is required by the initial market/news/social design.
- POST `/pause`: `{revision}`; immediately disables scheduling and invalidates
  in-flight analysis/review intents. A stale revision must not prevent pausing.
- POST `/check`: `{revision}`; queues one bounded check using the same consent,
  rate/cost budget and no-concurrency controls as the background loop.
- POST `/dismiss`: `{revision,proposalId}`; marks a proposal dismissed.

Proposal fields: `id`, `revision`, `createdAt`, `expiresAt`, `status`, `sellToken`
(`ETH` or `USDC`), `buyToken`, `sellAmount` (integer base units string),
`estimatedSellUsd` decimal string, `rationale`, `risks` string array, `evidenceIds`.
Only the two opposite-token pairs are allowed. Hold is a status/analysis, never an
executable proposal. At most one pending proposal per monitor; bounded history.
Public amounts are strings; code uses checked integers/rationals, not floating point.

Backend owns shared type `cryptoTradeIntent` with exported fields `ProposalID`,
`Revision`, `WalletAddress`, `ChainID`, `SellToken`, `BuyToken`, `SellAmount`,
`BudgetUSD`, `MaxTradeUSD`, `ReserveETH`, `SlippageBPS`, `ExpiresAt`. Methods:

- `a.cryptoReviewIntent(user, proposalID string) (cryptoTradeIntent, error)`
- `a.cryptoIntentCurrent(user string, intent cryptoTradeIntent) bool`
- `a.cryptoRecordTransaction(user, proposalID, hash, status string) error`

Wallet owns `a.cryptoWalletSnapshot(ctx context.Context, address string)
(cryptoWalletSnapshot,error)`. Snapshot shared type has `Address`, `ChainID`,
`BlockNumber`, `FetchedAt`, `ETH`, `USDC` (base-unit decimal strings). Monitor uses
actual public chain balances and excludes address from model input.

## Public evidence and paid work

Use live public market data with retrieved/source timestamps, bounded HTTPS
responses, fixed endpoints and no redirects/proxies. DeepSeek Search is available
using the existing DeepSeek key. Server-authored, bounded news and indexed-social
queries can supply timestamped result titles/snippets/URLs. Label indexed social
posts honestly; this is not unrestricted X/Reddit API access or a social firehose.
No model-selected tools, URLs or requests. Public snippets are untrusted evidence,
not instructions. Model only emits validated proposals referencing supplied IDs.

Persist attempts/reserved estimated cost before requests, including both search
and inference. Enforce daily call and conservative estimated-cost caps, no catch-up
bursts after restart, one in-flight run per user, bounded history and no blind retry
after paid-call uncertainty. Display estimates rather than an exact provider bill.
Check user enabled state, settings revision and key/model identity before and after
network work. Opted-in monitoring can resume after host restart without a browser;
paper-run semantics remain unchanged. No account/private ledger data is queried.

After committing a new proposal call `a.cryptoQueueNotification(user, proposalID)`.
Root owns that durable generic-notification outbox. A notification never signs,
approves or executes; it only opens the authenticated Crypto review view.

## Wallet API and execution

Wallet agent defines final quote/receipt routes and sends their JSON shapes to UI.
Prefer deterministic direct Uniswap v3 Base ETH/USDC quotes over opaque aggregator
calldata. Use officially documented contract addresses and checked ABI encoding,
an on-chain deadline and minimum output, exact token approval only when required.
Every approval/reset/swap requires a separate user gesture and wallet confirmation.
Requote after approval receipt, and require a fresh review; never automatically
submit a swap following an approval. No Permit2, unlimited allowances, bridging,
arbitrary recipients, margin or arbitrary contracts.

Wallet UI uses EIP-6963 provider choice plus injected fallback. Account/network
changes, stale intents, logout, expiry or changed details invalidate the review.
Transaction hash means submitted, not filled. Verify receipt and transaction intent
through read-only RPC before reporting confirmed/reverted; unknown remains unknown.

## Push API

- GET `/push`: `{publicKey,subscriptions,available}`; no endpoint/key material.
- POST `/push/subscribe`: `{endpoint,keys:{p256dh,auth}}` after browser permission.
- POST `/push/remove`: `{endpoint}` restricted to the current account.
- POST `/push/test`: sends a generic test only to the current account's registered
  devices, explicitly requested by its test button.

Finance-scoped `finance-push-sw.js` has no financial data or fetch/cache handler.
Push text is generic and click opens only same-origin `#crypto`. Authenticated
refetch gates proposal details. Do not expose wallet/address/value on lock screens.

## Acceptance

Email routes: GET `/email` returns `{provider:"resend",configured,enabled,from,to,
revision,lastAcceptedAt,lastError}` without a key. POST `/email` saves
`{revision,enabled,from,to,apiKey,consent}`; a blank key retains the saved value.
Only single bare email addresses are accepted. `{revision,remove:true}` removes
the protected configuration, cancels pending jobs and retains a revision/throttle
tombstone. Both writes return the GET view. POST `/email/test` queues a generic
test only after an explicit user action, at most once per minute. Background jobs
are generic, durable, proposal-bound and sent through the fixed Resend API with
stable idempotency keys and at most three attempts inside 30 minutes.

Use synthetic NTFS accounts, deterministic provider/RPC fixtures and real public
read-only endpoint checks where suitable. Never use owner wallet, real trades,
owner ledger or paid keys as test fixtures. Explicitly distinguish mocked transport,
public live reads, actual push delivery and any unverified wallet/provider steps.
Follow the full app lifecycle for source, ordinary build, supported fresh/upgrade/
rollback, immutable release/channel and primary installation. Second PC deferred.
