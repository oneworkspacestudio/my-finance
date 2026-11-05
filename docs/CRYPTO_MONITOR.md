# Crypto monitor

The Crypto view uses live data and prepares real wallet transactions for human
review. It is separate from the fictional paper portfolio. The first supported
network is Base, chain 8453, and the supported pair is native ETH and Base USDC.
Actual remains the household ledger; this feature stores monitoring settings,
evidence, proposals and transaction receipts, not a replacement accounting ledger.

## Setup

1. Enter your own DeepSeek API key in Money manager's AI settings. Monitoring uses
   the fixed `deepseek-flash` model and DeepSeek Search. Provider charges apply.
2. Open Crypto, connect MetaMask/Rabby or another compatible injected EVM wallet,
   and select Base. On a phone, use a wallet's supported browser. WalletConnect,
   exchange API keys, Solana, arbitrary tokens and bridges are not included.
3. Set the maximum total ETH exposure in USD, maximum individual trade value,
   ETH reserve for gas, slippage, analysis interval and daily analysis/API limits.
   Existing ETH counts toward exposure; USDC cash does not. Values are estimates
   based on current market data, not guaranteed liquidation values.
4. Explicitly enable background analysis and consent to sending the configured
   objective, public evidence and wallet balance amounts to DeepSeek. The wallet
   address and private keys are not sent to the model. Public RPC requests must
   include the watched address to retrieve its balances.
5. Enable notifications on the secure Finance address and use the test button.
   Browser/OS permission is required. On iOS, add the app to the Home Screen and
   enable notifications there. Push acceptance does not guarantee OS delivery.
6. For email, create a Resend sending API key and verify your sender domain in
   Resend. In Crypto settings enter one bare sender address, your recipient
   address and the key, then consent and enable alerts. Saving does not send an
   email. Use Send test email and check the inbox/spam folder. The status reports
   provider acceptance, not inbox delivery. A blank key keeps the saved key;
   removing settings deletes it locally and cancels queued alerts. Revoke the
   key in Resend separately if needed. Resend's account limits and charges apply.

No funds are added by this app. Fund your own Base wallet through your chosen
provider and retain ETH for gas. Never enter a seed phrase or wallet private key
in Finance. A connected wallet is not permission for automatic execution.

## Continuous monitoring

The Windows Workspace host must remain running, online and awake. The browser may
be closed. The persisted schedule resumes after a host restart without replaying
missed intervals in a burst. Pause disables future analysis and invalidates stale
reviews. Closing a view or signing out does not cancel explicitly enabled
background monitoring; use Pause when you want it stopped.

Market information comes from public DEX Screener pools, while news and public
social context come from bounded DeepSeek searches. Indexed social results are
not a live social firehose, private posts, comprehensive X/Reddit access or a
reliable popularity measure. Sources include retrieval and available publication
dates. Missing, stale, invalid or contradictory data can result in hold or an
unavailable state. The model has no tools and cannot invent executable endpoints.

Daily analysis counts and conservative estimated API cost reservations persist
before requests. The day resets at UTC midnight. Estimates are not the provider's
invoice; external use of the same key is outside this app's accounting. Limit
changes do not erase work already counted. Errors do not cause blind paid retries.
No proposal implies a guaranteed return or that the configured objective is viable.

## Review and signing

Notification text contains no address, balance or trade amount. Its button opens
the authenticated Crypto view. It does not approve, sign or submit anything.
Push and email are independently optional. Email uses only Resend's fixed sending
endpoint, a generic message and the configured Finance link. No trade/model text
is emailed. A durable outbox uses one stable idempotency key per alert, at most
three attempts within 30 minutes, and cancels stale proposals or changed settings.
An email already accepted by the provider cannot be recalled. The alert may arrive
after a proposal expires; always check the current authenticated view.

The app prepares a quote from the fixed Uniswap v3 0.05% ETH/USDC pool. This is a
limited direct route, not a claim to find the best route across all exchanges.
Unavailable liquidity or simulation failure blocks preparation. RPC/market data
providers may be unavailable or rate limited, in which case no cached price is
silently substituted for a fresh executable review.

Review the network, assets, input amount, minimum received, pool fee, slippage,
gas estimate and on-chain deadline. The application review window is separate
from the deadline enforced by the router contract. A wallet dialog already open
cannot reliably be closed by Finance; reject it in the wallet if circumstances
change. Network fees can still be charged for a reverted on-chain transaction.

USDC may require an exact token allowance. An existing differing allowance must
first be reset, then the exact allowance approved. Each is a separate wallet
confirmation. After its receipt, obtain and review a fresh swap quote; Finance
never automatically proceeds from approval to swap. There are no unlimited
approvals, Permit2 permissions, session signing keys or arbitrary recipients.

The final transaction is signed and submitted by the wallet after your explicit
action. Finance's server contains no signing or transaction-broadcast method.
A returned hash means submitted/unknown, not completed. Receipt checking verifies
the original sender, target, calldata, value and canonical safe block before
showing confirmed or reverted. Keep unknown transactions unresolved until checked;
there is no automatic replacement or retry.

## Storage and verification

Per-user settings, wallet snapshots, evidence, proposals, review history, VAPID
keys and push subscriptions use account-bound Windows DPAPI envelopes in private
Finance storage. Normal Workspace recovery can include these protected records.
They are not part of Actual budget exports or sanitized native server snapshots.
No wallet seed/private key or exchange trading credential is collected.
Email keys and sender/recipient addresses use a separate account/origin-bound
DPAPI record. Provider error bodies and API keys are never shown or logged.

Deterministic transport/browser tests are not evidence of a paid DeepSeek request,
actual phone push/email delivery or a real swap. The test report distinguishes these.
The user's wallet and real financial data must never be used as a test fixture.

## Provider references

- [DeepSeek Search API](https://api-docs.deepseek.com/guides/search/)
- [DeepSeek pricing](https://api-docs.deepseek.com/quick_start/pricing/)
- [DEX Screener API](https://docs.dexscreener.com/api/reference)
- [Uniswap v3 Base deployments](https://developers.uniswap.org/docs/protocols/v3/deployments/v3-base-deployments)
- [MetaMask wallet transaction confirmation](https://docs.metamask.io/metamask-connect/evm/reference/json-rpc-api/eth_sendTransaction/)
- [Web Push on iOS Home Screen apps](https://webkit.org/blog/13878/)
- [Resend email API](https://resend.com/docs/api-reference/emails/send-email)
- [Resend idempotency keys](https://resend.com/docs/dashboard/emails/idempotency-keys)
