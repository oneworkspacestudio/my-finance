# Acceptance evidence — Windows preview, 28 October 2026

This working preview is under final managed-install acceptance. It is **not a
complete pass of the master brief**. No official provider sandbox or live banking
account was used. All users, transactions, statements and screenshots are fictional.
Neither owner installation has been modified by this Finance task yet.

The hash-pinned candidate launcher ran against fresh NTFS data with runtime SHA256
`f3c4675bf401042356310325a35de736de3c7b2f53aab2e74b611839f98da3ca`.
The earlier hand-edited diagnostic runtime is excluded. Release1 final launcher f85eeab and anonymous portable download are verified;
real local managed OneAccount acceptance passes. Owner delivery remains pending.

| ID | Result | Evidence and remaining limit |
|---|---|---|
| FIN-01 | PARTIAL | Bundled Windows Node/native SQLite starts from fresh data; corrected managed installer includes runtime. Separate host without development tools not tested. |
| FIN-02 | PASS (local) | Real local MyAccounts code/PKCE -> Finance -> native budget passes, with no second password; owner maps to ADMIN. |
| FIN-03 | PASS (local) | BASIC member cannot list/download unshared file; sharing grants access and revocation denies it. Real local MyAccounts owner/member flow also passes. |
| FIN-04 | PARTIAL | Anonymous/forged native tokens and missing mutation Origin denied. Real managed callback forgery and member logout denied; independent owner session retained. Expiry-by-clock acceptance pending. |
| FIN-05 | PASS | Native account opens at 123456 pence; GBP/dates, monthly allocation and clean-store persistence verified. |
| FIN-06 | NOT TESTED | Official sandbox GB directory needs provider credentials. Deterministic GB mock passes. |
| FIN-07 | PARTIAL | Pinned native adapter completes mocked AIS consent/session, balance and 100 transactions over two pages. Official sandbox/live not tested. |
| FIN-08 | PASS (local) | State tests reject foreign session/user, expiry and replay; forged native actor token cannot select owner. |
| FIN-09 | PARTIAL | Invalid/expired state denied; status distinguishes authorisation from import. Provider cancellation/expiry UI not sandbox-tested. |
| FIN-10 | PASS | 100 stable-ID transactions imported ten times; exactly 100 records, manual category/notes preserved. |
| FIN-11 | PARTIAL | Stable-ID pending/booked remains one record. Changing provider IDs require separate qualification. |
| FIN-12 | PASS | Linked credit repayment creates opposite transfer, preserves combined balance and has no spending category. |
| FIN-13 | NOT TESTED | Outage/429 and last-good/stale UI acceptance outstanding. Pagination repeated-cursor/limit fails instead of partial success. |
| FIN-14 | DEFERRED | No unattended bank-sync worker under enforced OneAccount/E2EE. Native open-budget Sync supported. |
| FIN-15 | PARTIAL | Mock expiry/check/revocation passes; ledger retained. Real bank revocation/renewal not tested. |
| FIN-16 | PASS | Native CSV applied import gives Grocer -1234p / Refund +250p and correct 122472p balance. Repeating the same CSV reports zero new transactions with an empty date filter. |
| FIN-17 | PASS (local) | Export encrypt/decrypt byte-identical; clean import preserves ledger/categories/balances/budget. Snapshot/My Drive encrypted copies match; browser-closed scheduled run and clean CLI restore pass; overwrite refused. Fresh Workspace100/real MyAccounts/Drive displays encrypted snapshots; Drive UI download matches Finance snapshot SHA256. |
| FIN-18 | PASS (local) | File ACL and owner-only bank/backup/snapshot/private-primitive/admin guards exercised. Real local MyAccounts member checks also pass. |
| FIN-19 | PARTIAL | Go tests/vet pass. govulncheck: zero affected symbols/imported-package vulnerabilities; three unreachable required-module records. npm: zero high/critical, three moderate records through disabled Akahu/uuid. Defender candidate-directory and rebuilt final executable scans found no threats. Public asset size/SHA256 and anonymous portable download verified. |
| FIN-20 | PARTIAL | Welcome/native 360px and 768px have no document overflow; screenshots reviewed. Keyboard Tab/Enter reaches Add New, edits notes/payment and saves exactly 101p; balance becomes 122371p. Full accessibility audit remains outstanding. |
| FIN-21 | PARTIAL | Native API restart/clean server restore preserve data. Installer sidecar selection regression fixed/tested. Launcher interruption/restart exposed and fixed ordered-migration downgrade rejection; restart regression and final ledger/snapshot checks pass. Owner update/rollback and reboot/power-loss pending. |
| FIN-22 | NOT TESTED | Public HTTPS callbacks not tested. Real local MyAccounts redirect observed; banking needs provider application/consent. |

## Tests

- `go test ./...`, `go vet ./...`: authenticated backups/ZIP guards, consent
  binding/replay/expiry, rebasing, Windows DPAPI, snapshot manifest/path/hash and
  retention/changed-file preservation.
- `node --test test/provider-mock.test.mjs`: pinned native adapter with mocked
  provider HTTP, actor/owner guards, permission check/revoke, sanitized snapshot.
- `test/integration/ledger-backup.mjs`: ten native ledger/transfer/restore checks.
- `test/integration/household-access.mjs`: six native ACL/privilege checks.
- `test/integration/server-snapshots.mjs`: six Drive/key/schedule/restore checks;
  sets a due time only in the explicitly named synthetic fixture.
- `test/e2e/native-browser.mjs`: dedicated Chrome profile, native GBP account,
  applied CSV and responsive screenshots. Harness assertions account for native
  payee capitalisation (CSV becomes Csv) and await loaded account rows.
- Suite catalog/distribution/suite/apphost tests pass. Finance sidecar selection
  has a regression test; corrected fresh setup verifies all four fixture packages.

Integration scripts require explicit FINANCE_MODULE_ROOT (pinned npm cache),
FINANCE_TEST_URL and named synthetic paths. Never target owner data. Sanitized
reports/screenshots go under docs/evidence; cookies/keys/raw logs are excluded.

Supplemental `test/e2e/repeat-csv-review.mjs` and `keyboard-transaction.mjs`
record the exact shipping-fixture browser checks. Run repeated CSV against the
fictional native-browser fixture after its two-row import; run keyboard entry
once against its 122472p balance. Both require `FINANCE_SYNTHETIC_ONLY=1` plus
the explicit module root, test URL and evidence folder. These are stateful
acceptance harnesses rather than independent unit tests.

`managed-drive-snapshot.mjs` followed by `managed-drive-download.mjs` records
the real Drive flow in disposable ManagedFixture4 on localhost47300, with generated
example.test users from its private test-users.json. It requires the same explicit
synthetic acknowledgement. Three encrypted retention-tier files are visible and
the UI download equals the snapshot bytes. Private browser profiles, session
state, test-user credentials and raw captures are excluded from published evidence.

## Provider qualification still required

Follow OPEN_BANKING_SETUP_UK with the user's own application/key and exact HTTPS
callback. Select a sandbox GB institution, approve consent personally and compare
accounts/balances/transactions. Test foreign/replayed/cancelled/expired callbacks,
outage/429 and revocation. Record sandbox and live separately; mocks are not sandbox
evidence. No real bank credentials were requested, discovered or used by tests.

Real-local managed acceptance: `test/e2e/managed-oneaccount.mjs` passes six checks,
including owner/native budget, BASIC isolation, share/revoke, callback forgery and
sign-out. Fresh Accounts/Finance fixture uses an actual MyAccounts executable,
not a mock issuer. No public HTTPS/provider callback claim follows from this.
