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
