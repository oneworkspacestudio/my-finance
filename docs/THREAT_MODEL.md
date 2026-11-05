# Threat model

| Threat | Control and residual limit |
|---|---|
| Stolen Windows login | Private NTFS ACL, native apphost ownership checks, recommend OS disk encryption. A process acting as this same Windows user can still read server secrets. |
| Forged proxy identity | Gateway discards incoming identity/auth headers; random engine capability is supplied only on the loopback hop. OneAccount issuer/signature/PKCE/nonce validation is apphost's responsibility. |
| Replayed or foreign bank callback | Exact HTTPS redirect, state bound to app identity+session, five-minute TTL, consumption before exchange. Restart loses pending states and requires a fresh consent. |
| Leaked provider RSA key | Never logged or included in Finance's export wrapper. Revoke/rotate with the provider. Native upstream stores it server-readable; E2EE does not protect it. |
| Exposed tunnel URL | Finance requires its own valid OneAccount cookie. Native engine is loopback-only and capability guarded, including health/static surfaces. |
| Compromised server | Can read budget data when not E2EE and native bank secrets. Do not promise protection against the server operator. |
| Stolen backup | Argon2id (64 MiB, three passes, parallelism two), random salt, AES-256-GCM with authenticated format header. Manual passphrases are not persisted. Optional scheduled snapshots store their recovery key only after explicit CurrentUser DPAPI consent. Losing the recovery passphrase means losing access on another machine. |
| IDOR/shared budgets | Native Actual file ACLs, stable actor overwrite on every request, bank endpoints owner-only. Shared budget exposes every account in it; separate budgets for private spending. Native Actual is not a strong multi-tenant boundary for mutually untrusted households. |
| Lost E2EE passphrase | Cannot recover an E2EE budget from server blobs alone. Keep verified native exports and recovery passphrases separately. Never silently switch encryption off. |
| CSRF | Native mutation routes require exact Origin plus apphost session; custom backup routes also require apphost CSRF. Cross-origin CORS headers are stripped. |
| Runtime tampering/path links | Launcher pins archive SHA256; private runtime folder; reject path escapes/symlinks and reparse ancestors. A compromised same-user session remains outside the isolation guarantee. |
| Malicious imported ZIP | Size/decompression/member allowlists, CRC checks, SQLite signature and valid metadata; no extraction to arbitrary files. Native Actual remains responsible for validating its own ledger format on import. |
| AI prompt injection or invented figures | Only bounded summary data is accepted; no tools or ledger writes. Arithmetic and allocation limits run in Go. Model output is untrusted plain text and must be reviewed. Prompts cannot guarantee factual advice. |
| AI data disclosure | Per-request explicit consent and exact payload preview. No raw transactions, payees, notes or account identifiers sent by the native summary bridge. Category/goal labels and the user's question can still contain private information. |
| AI key theft or cross-account use | Per-user CurrentUser DPAPI credentials, authenticated CSRF-protected routes, fixed provider endpoints, redirect rejection and sanitized errors. Same-Windows-user compromise remains outside the guarantee. |
| Saved manager rule disclosure or substitution | Profiles use DPAPI envelopes bound to user and budget digest, no-store responses and authenticated/CSRF-protected writes. Fifty scopes per user; no ledger access is granted by a digest. |
| AI allocation or stale-data error | Explicit proposal review/application, known-category and money bounds, deterministic penny allocation. Current snapshots display date; refresh after sync and month change. Planned allowances are not cash available today. |
| Model attempts real trades or money movement | Paper actions affect only virtual portfolios. Separate crypto proposals cannot sign or broadcast; deterministic Base ETH/USDC requests require individual human wallet confirmation. No model-accessible broker or payment tools. |
| Replayed or interrupted paper decisions | Per-user revision checks, counted attempts before paid request, finite session-bound run consent, pause/restart invalidation and all-or-nothing decision application. |
| Broker key theft, substitution or redirected request | DPAPI binds user/environment; credentials never returned. Only fixed summary/position GET URLs, redirect denial, request/response bounds, refresh rate limit and in-flight credential-change check. |
| Misleading simulation performance | Persistent paper labels, user-entered scenario prices, visible activity and stated absence of market execution, fees/tax/FX modelling or promised returns. |
| Malicious crypto news/social snippets | Fixed bounded searches, dates and supplied evidence IDs; untrusted plain text, no model tools or arbitrary URL fetches. Source claims may still be wrong. |
| Unbounded background API charges | Explicit opt-in, persisted pre-call reservations, interval/daily caps, fixed flash model and token bounds; no catch-up or uncertain paid retries. Estimates are not invoices or limits on other uses of the key. |
| Substituted wallet transaction | Fixed Base router/token/fee, independent browser ABI reconstruction, exact amount/recipient/allowance, fresh market/balance/reserve checks, on-chain minimum/deadline and safe-block receipt verification. Compromised wallet/browser/RPC remains a risk. |
| Stale wallet approval | Session/revision/expiry checks before preparation and wallet prompt; separate human gestures per operation. An already-open wallet prompt cannot be revoked by Finance; decline it if circumstances change. |
| Notification leakage, redirect or duplicate email | Generic encrypted push, Finance-only worker scope, known push hosts; fixed Resend endpoint, user/origin-bound DPAPI recipient/key, stable idempotency key and three attempts inside30min. Alerts only open authenticated review. Already accepted alerts cannot be recalled. |

No automated bank consent, payment endpoint or banking-password collection is
provided. Optional AI management guidance transmits only the reviewed summary and context
when the signed-in user explicitly requests it. Separately enabled crypto
monitoring can send consented wallet balance amounts/objective/public evidence to
DeepSeek while the browser is closed; it never sends the wallet address or ledger.
