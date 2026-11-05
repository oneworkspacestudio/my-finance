# Security notes

This is a household preview, not a public multi-tenant financial service. Complete
the acceptance gates in TEST_REPORT before real financial use. Use a trusted Windows
account, NTFS private data and OS disk encryption. Only the loopback gateway is
registered with One Workspace. Do not forward the internal Actual listener.

OneAccount sessions expire and sign-out is managed by Workspace/apphost. Native
Actual tokens do not choose the actor: the gateway supplies a verified issuer/sub
identity for each request, and native body tokens cannot override it.

Actual's native bank secrets are server-readable in its protected account database.
This preview does not claim DPAPI wraps those upstream records. Budget E2EE protects
the budget's server copy, not provider RSA keys, SimpleFIN access URLs or an already
unlocked browser. The integration adds no external telemetry.

Finance export backups deliberately omit the server databases/configuration. They
wrap only a user-exported native budget ZIP. Server snapshots include only a
sanitized metadata database and native budget/sync files; credentials/login sessions
are removed from the copy. Scheduled snapshot recovery keys use CurrentUser DPAPI
only after explicit acknowledgement; manual passphrases are never saved. Reconnect provider credentials after
restore. The ledger export can contain bank linkage references and personal ledger
data and must be treated as sensitive even before wrapping it. Avoid leaving a
plaintext exported or decrypted ZIP in a shared Downloads folder.

See THREAT_MODEL and UPSTREAM_ACTUAL for the patch boundaries and residual risks.

Money manager's AI is optional. Each OneAccount user stores their own OpenAI or DeepSeek
key under Windows CurrentUser DPAPI in Finance's private database. The key is never
returned to the browser, shared with another household account or included in
Finance ledger exports/server snapshots. Normal Workspace recovery may contain
the protected private database; restoring keys under a different Windows account
requires entering them again.

No AI request runs when opening a budget or calculating a plan. A user must review
and consent to sending the displayed monthly/category totals, profile, goals and
question to the selected provider. These summaries can still be sensitive.
OpenAI requests set store=false; this does not promise zero provider retention.
Provider terms and API charges apply. Money manager prompts and answers are not saved by Finance.
The short-lived, account-bound summary handoff uses sessionStorage only until the
manager consumes it; local browser compromise remains outside the threat model.

Explicitly saved manager profiles/rules use a separate CurrentUser DPAPI envelope
bound to the user and budget digest. No summaries or answers are persisted. Fifty
saved scopes per account and bounded profile/category fields limit storage. Saved
rules are excluded from Finance snapshots/exports but may be in normal protected
Workspace recovery. Budget digests do not grant Actual access or leave in AI data.
Current-month totals remain snapshots. AI rules are untrusted bounded proposals;
users apply them explicitly and integer arithmetic recomputes affordability.

The Investments paper portfolio is separate from Actual. Its rules, virtual
balances and bounded decision audit are per-user DPAPI records. The LLM can only
return a validated paper decision; no real order or payment adapter is available
to it. Run consent, persistent attempt limits, revision checks and deterministic
money/quantity limits prevent stale responses or model output from widening the
experiment. A paused or restarted run cannot commit an old in-flight decision.
Paper portfolio data is sent only after explicit bounded-run consent.

Trading 212 keys use a separate DPAPI account/environment envelope. The adapter
has two literal GET-only endpoints, bounded responses, no redirects or ambient
proxy, and sanitized errors. Broker data is displayed transiently, never sent to
the model or persisted. Users should create read-only keys; the app cannot verify
all permissions on an entered key. Removing a key locally does not revoke it at
the provider. Finance snapshots omit both paper state and broker credentials;
normal Workspace recovery may include their protected records.

Native ledger worker console output is suppressed so reconciliation/debug objects
cannot print transaction payloads. Engine server output is also discarded; the Go
gateway records only safe lifecycle/audit categories. Application errors still
appear in the native UI. The privacy policy blocks native upstream news/update
requests; Workspace supplies application updates.

Actual keeps local budget copies in the browser profile. Sign-out invalidates
server access and the native page checks its session on focus and every15 seconds,
but it does not securely erase that browser's IndexedDB. Use separate OS/browser
profiles on devices shared with untrusted people. No stronger browser-cache or
multi-tenant isolation guarantee is made than upstream Actual provides.
