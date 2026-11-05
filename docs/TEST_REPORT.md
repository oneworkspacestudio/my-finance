# Acceptance evidence — Windows preview, 28–29 October 2026

This working preview is under final managed-install acceptance. It is **not a
complete pass of the master brief**. No official provider sandbox or live banking
account was used. All users, transactions, statements and screenshots are fictional.
The original October primary delivery installed Workspace100/Finance1 after its normal
protected offline update. Suite100 is public: all30 assets match local hashes,
the installed candidate matches those published bytes, and the exact anonymous
update channel and installer check pass.
DESKTOP-Workstation delivery is deferred at the user's request while this PC finishes.

## Investment workspace acceptance, 5 November 2026

Finance5 adds a virtual paper portfolio and an independent read-only Trading212
connection. Full Go tests and vet pass. Paper tests cover700 allocation/conservation
scenarios, exact cost basis, atomic rejected batches, order/cash/position/loss limits,
bounded history, strict provider responses, persisted attempts, account/DPAPI
isolation, cancellation races, expired sessions and restart safety. Schema4→6
retains existing protected settings; older executables reject newer data.

Broker tests use synthetic HTTP responses and verify the two fixed GET endpoints,
HTTP Basic credentials, redirect rejection, strict numeric data, response bounds,
rate limits, no account-ID/FX leakage, DPAPI environment/user binding and in-flight
credential changes. No broker or bank account was used as a fixture and no paid
AI call was made. Provider sandbox/live qualification remains unverified.

Investment browser acceptance exercises agreed limits, explicit finite-run consent,
pause/cancel, no automatic retry or resume, key clearing, account switching, plain
text model output, and paper/broker layouts at320/390/768/1280px. Existing manager
UI and nine native-summary regressions are retained. Dedicated managed fresh
setup/upgrade and installed-byte receipts are recorded under
`docs/evidence/investments-20261105/` as each delivery gate completes.

Real public114 fresh selected setup installs Finance1. Its supported upgrade to
the shipping Finance5 EXE (`9d1d226f…4d9b03f`) passes MyAccounts/proxy acceptance. Five owned
payloads and fifteen recovery files match their receipts; the original fictional
Actual ledger, identity, decrypted credentials, paths and unrelated program bytes
are preserved. Real paper UI/HTTP setup, scenario prices, reload, missing-key and
consent checks pass. Supported rollback to 1 and re-upgrade to 5 retain the same
original ledger. Final shipping-byte checks verify the executable, runtime,
schema 6 and all five served UI assets. The test installation and its remaining
empty task cache directories were removed after process and path checks.
The synthetic test used a cold regenerated engine cache after safely cancelling
an excessively slow pretransaction fixture backup; no owner recovery is bypassed.

Release5's ten immutable assets and exact anonymous public update channel are
verified. The suite's managed-seed builder now consumes the Finance pin: it had
previously retained bundled Finance1. Projection tests retain all36 unrelated
apps/446 unrelated package contracts and candidate checks reject missing runtime
or corrupt members. The ordinary Workspace build from d7cdffb also passes its
complete 37-app catalog audit and verifies all 451 archive members, including the
exact five public Finance 5 payloads. Local QA115 was not published or installed;
the independent Finance release updates existing Workspace installations.

Primary delivery completed through public Workspace114's normal protected
Finance-only update from 4 to 5. All five Finance payloads and served assets match
the release; 423 unrelated package contracts/bytes, other app definitions,
existing credentials and locations are preserved. All 36 services are running
with no new failures. The 382.5 MB recovery copy verified; automatic cleanup
retained it and finished without warnings. Public HTTPS requires Workspace
sign-in. See delivery.json, served-verification.json and update-cleanup.json.
No owner ledger was opened for QA. The second PC remains deferred by the user.

## Money manager acceptance, 3 November 2026

Finance4 adds current-month category allowances, saved manager rules and reviewed
AI rule proposals. Go tests/vet pass, including a600-case allocation conservation
sweep, extreme-value rounding, essential/flexible priorities, existing spending,
goal/cash-buffer investment gating, current-date checks and proposal validation.
Schema3→4 preserves AI keys; encrypted rule tests cover user/budget substitution,
CSRF/Origin/account guards, fifty-scope bounds and a200-category DPAPI roundtrip.
The existing4KiB credential bound is preserved with a separate bounded manager
envelope. Focused suite catalog/update-group checks pass.

Nine native-summary tests pass. Real pinned Actual verification reads exact
historical600100p incoming/300000p outgoing and current200345p incoming/17345p
outgoing, including a current-only category, while keeping every ledger row
unchanged and making no provider request. Mobile helper fits320/360/390px.
The UI acceptance suite covers auto recalculation, download without AI, scoped
save/load/remove, proposal application, invalid/stale responses and phone layouts.
Final source, screenshot and installed delivery receipts are recorded separately
under `docs/evidence/manager-20261103/` after completion.

Published Finance4 shipping-byte native verification passes on source11e8dd24.
The real plan endpoint conserves forecast income, protects unclassified costs,
uses current spending and persists/reads/removes encrypted budget-scoped rules.
Every native transaction remains unchanged afterward. See shipping-byte-check,
native-coach-report, package-validation and publication receipts. Ten public
assets and the anonymous channel were verified. Local protected update status
is recorded in the delivery receipt when complete; second PC remains deferred.

Primary delivery completed: Finance4 on Workspace113, five Finance payloads and
served UI match the release;423 unselected package contracts/bytes, other apps,
existing credentials and locations preserved.36 services healthy, no new failure.
Recovery382.4MB verified; normal cleanup completed and retained the latest copy.
Public HTTPS requires Workspace sign-in. See delivery.json, served-verification.json
and update-cleanup.json. No owner ledger was opened for testing.

## Money coach acceptance, 3 November 2026 (Finance3)

The Finance3 change adds optional OpenAI/DeepSeek coaching over reviewed aggregates
and a provider-independent plan calculator. `go test ./...` and `go vet ./...`
pass, including integer rounding, deficit/goal affordability, essential-cost
overrides, investment prerequisites, schema2→3/restart/future-schema refusal,
DPAPI credential binding, account isolation, CSRF/Origin/session checks, explicit
consent, request limits, redirected/failed/truncated/malformed provider responses
and sanitized errors. Provider transport tests use fictional responses only.

`test/coach-ledger.test.mjs` verifies splits, complete months including zero months,
currency/amount/date validation, transfers/opening/tracking exclusions, category
reconciliation, revoked/switched budget checks and the pinned desktop worker
adapter. Standard starting-balance labels are recognized; renamed/localized
opening entries need user review.

`test/e2e/coach-ui.cjs` runs on its own synthetic loopback server. It covers summary
handoff consumption, manual input, plan preview without an AI call, provider
settings, per-request consent, failed requests, edits during verification/inference,
safe text output and complete account-change clearing. Screenshots are reviewed
at320/390/768/1280px with no document overflow; physical-phone testing is not claimed.

No live OpenAI/DeepSeek account, personal transaction data or owner's installation
was used as a development fixture. Existing official-bank/clean-host/reboot gates
below remain unchanged. Final native and delivery receipts live in
`docs/evidence/ai-20261103/` when completed.

The hash-pinned candidate launcher ran against fresh NTFS data with runtime SHA256
`f3c4675bf401042356310325a35de736de3c7b2f53aab2e74b611839f98da3ca`.
The earlier hand-edited diagnostic runtime is excluded. Release1 final launcher f85eeab and anonymous portable download are verified;
real local managed OneAccount acceptance passes. Primary delivery preserves all
30 existing app contracts, 143 prior package records checked, 138 unselected package
bytes and existing identity/hosting credentials. All31 services run; latest verified
recovery is retained. The live HTTPS welcome/native entry uses the existing signed-in
session without a second password. No owner budget or bank was used as a test fixture.

| ID | Result | Evidence and remaining limit |
|---|---|---|
| FIN-01 | PARTIAL | Bundled Windows Node/native SQLite starts from fresh data; corrected managed installer includes runtime. Separate host without development tools not tested. |
| FIN-02 | PASS (local/live entry) | Real local MyAccounts code/PKCE -> Finance -> native budget passes, with no second password; owner maps to ADMIN. Installed primary HTTPS welcome/native entry also reuses the existing signed-in browser session. |
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
| FIN-21 | PARTIAL | Native API restart/clean server restore preserve data. Installer sidecar selection regression fixed/tested. Launcher interruption/restart exposed and fixed ordered-migration downgrade rejection; restart regression passes. Primary protected96→100 update retains all30 existing contracts,138 unselected package bytes and credentials; all31 services healthy,1.47GB recovery copy verified. Owner rollback/reboot/power-loss pending. |
| FIN-22 | PARTIAL | Real local MyAccounts redirect observed; installed primary HTTPS welcome/native entry works with its existing signed-in session. Public bank callback/official provider consent remains unverified. |

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

Native shipping-source acceptance also passes against the real pinned Actual
browser: exact3-month totals600100p in/300000p out,7included lines,2transfer legs
excluded, standard opening balance/off-budget exclusions and identical ledger
before/after. No provider request. The selection helper fits320/360/390px and
minimises/restores without covering native controls. See native-coach-report.json.

Finance3 is public from09a7029 with10 verified immutable assets and promoted
channel790dd02. Local normal protected Finance-only update2→3 on Workspace113
completed. All5 owned payloads/served assets match,423 unselected payload records
and bytes retained, existing credentials/settings/locations retained,36 services
healthy, verified recovery kept and normal cleanup finished. Secondary installation
remains deferred. See ai-20261103/publication.json and delivery.json. The public
HTTPS proxy still requires Workspace sign-in; no owner ledger was used for QA.
