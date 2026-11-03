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

No automated bank consent, payment endpoint or banking-password collection is
provided. Optional AI management guidance transmits only the reviewed summary and context
when the signed-in user explicitly requests it.
