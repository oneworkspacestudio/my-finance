# Threat model

| Threat | Control and residual limit |
|---|---|
| Stolen Windows login | Private NTFS ACL, native apphost ownership checks, recommend OS disk encryption. A process acting as this same Windows user can still read server secrets. |
| Forged proxy identity | Gateway discards incoming identity/auth headers; random engine capability is supplied only on the loopback hop. OneAccount issuer/signature/PKCE/nonce validation is apphost's responsibility. |
| Replayed or foreign bank callback | Exact HTTPS redirect, state bound to app identity+session, five-minute TTL, consumption before exchange. Restart loses pending states and requires a fresh consent. |
| Leaked provider RSA key | Never logged or included in Finance's export wrapper. Revoke/rotate with the provider. Native upstream stores it server-readable; E2EE does not protect it. |
| Exposed tunnel URL | Finance requires its own valid OneAccount cookie. Native engine is loopback-only and capability guarded, including health/static surfaces. |
| Compromised server | Can read budget data when not E2EE and native bank secrets. Do not promise protection against the server operator. |
| Stolen backup | Argon2id (64 MiB, three passes, parallelism two), random salt, AES-256-GCM with authenticated format header. Passphrase is not persisted. Losing it means losing access to this backup. |
| IDOR/shared budgets | Native Actual file ACLs, stable actor overwrite on every request, bank endpoints owner-only. Shared budget exposes every account in it; separate budgets for private spending. Native Actual is not a strong multi-tenant boundary for mutually untrusted households. |
| Lost E2EE passphrase | Cannot recover an E2EE budget from server blobs alone. Keep verified native exports and recovery passphrases separately. Never silently switch encryption off. |
| CSRF | Native mutation routes require exact Origin plus apphost session; custom backup routes also require apphost CSRF. Cross-origin CORS headers are stripped. |
| Runtime tampering/path links | Launcher pins archive SHA256; private runtime folder; reject path escapes/symlinks and reparse ancestors. A compromised same-user session remains outside the isolation guarantee. |
| Malicious imported ZIP | Size/decompression/member allowlists, CRC checks, SQLite signature and valid metadata; no extraction to arbitrary files. Native Actual remains responsible for validating its own ledger format on import. |

No automated live consent, payment endpoint, banking-password collection or external
LLM categorisation is part of this application.
