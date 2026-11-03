# Money manager, 3 November 2026

User extends Finance3 coaching into an automatic category-budget manager with
spending limits, savings/investment reservations and UK investing options.
Prior release/local-install authorization continues; second PC remains deferred.
Preserve Actual as sole ledger, owner data and credentials, immutable assets and
disabled Actions. Tests use fictional NTFS data only.

Manager is enabled by the UI and automatically recalculates on summary/rule
changes, without paid inference. Lowest complete-month income is a forecast,
not cash held. Integer allocation protects essential requirements and incurred
spending, then debt/emergency, extra savings/named goals, eligible investment
reservation and requested cash cushion, then flexible targets. Stable largest
remainder avoids invented pennies. Essential/funding gaps and overspending remain
visible. Manager and legacy plan fields/buckets are coherent. Emergency basis
includes larger manager essential targets. No ledger writes, payments or trades.

Native bridge reads history plus current month through today, same exclusions.
Current totals are separate from historical counts. readNativeSnapshot returns
summary plus SHA256 cloud-file scope outside the summary/provider payload;
readNativeSummary stays compatible. Actual access/account/budget checks retained.
Refreshed selection remains required after native sync/import or month change.

Schema4 stores opt-in profiles/rules per user and scope in CurrentUser DPAPI;
does not store summaries/answers. GET/POST ai/manager-settings use auth, CSRF,
expected-account and no-store, bounded fifty scopes. DPAPI envelope binds user
and scope. Dedicated128KiB profile protection fixes existing4KiB session-helper
limit; original session/key bound unchanged. Cipher/plain reads bounded. Corrupt
rules use422 (409 is account-change in UI) and UI allows explicit removal.
Schema3→4 retains existing key bytes; future schema/downgrade refused.

AI may return bounded known-category managerSuggestion. Explicit apply copies
rules and recalculates; save is separate. No model output auto-execution. Unknown
category classification defaults essential. Existing fixed provider adapters,
per-request preview/consent, timeout/quota/no-retry, plain-text output retained.
UK guide links MoneyHelper/GOV.UK/FCA; no current rates/allowances or securities
picks. Live paid APIs still await user key; synthetic responses aren't provider
qualification. Current snapshot freshness and historical/context gaps gate
investment reservations, which are plans rather than invested money.

Acceptance: Go tests/vet,600-scenario allocation conservation, max-money rounding,
schema/profile/key isolation/DPAPI200category roundtrip, nine native-summary Node
tests pass. UI acceptance covers mobile, auto recalculation, saved scopes and AI
proposal races. Native real Actual current/historical totals pass on isolated
fixture; final shipping-byte repetition/publication/delivery receipts to append.
Initial native harness exposed creation-readiness/GBP-fixture setup races; fixed
test seeding, no production workaround. Phone screenshot close/resize transition
must settle before capture. UI duplicate change/input blur race fixed.

Version target4, same verified Actual26.10 runtimeSHA
f3c4675bf401042356310325a35de736de3c7b2f53aab2e74b611839f98da3ca.
Same5 package contracts, route and minimum manager100/integration3. Corrected
stale compatibility.json integration2 to match already-published manifest3;
packager now validates guard equality and reads data schema from metadata.

Primary baseline Workspace113/Finance3/37apps,36 services, normal completed
transaction. Suite source advanced to c2350a5 preserving concurrent MyAI23
promotion. No new suite/APK release required. Use independent Finance-only normal
protected update; preserve every unrelated app/payload/credential/location.
Tooling I:/Projects/tooling/finance-manager-20261103; dedicated NTFS fixture
C:/Projects/.cache/finance-manager-20261103, loopback48642. Track/stop task processes
and remove only verified disposable root at completion. Release path is existing
package-release.py + verified draft upload + public channel/component pin.

Published Finance4 from11e8dd24e2942dbe9b05a7bff38bf34720ebe6df. Ten immutable
assets verified against GitHub digests; public anonymous checksum bytes match.
PortableSHA8e9cc6bcd513fba0616757ee37c8a81e3cdd06e2b5a61dd53ee0feaab93cb366.
ShippingEXE21c88be7bce7ac26c2ca6f1ba4d82ba40faf7ca7cbd81597c46cdc67f52e6bf4
passed enhanced native test after clean-commit rebuild: current17345p out,
historical300000p out, exact manager allocation conservation, saved-rule real
DPAPI roundtrip and native ledger unchanged. No provider request. Source helper
tests9, full Go/vet and final UI/exponent-input regression pass.

Suitec05fb03 promoted only Finance4, preserving all existing public rows and
concurrent MyAI23 pin. Same anonymous channel now includes all10Finance4assets.
Primary normal public113 Finance-only update launched; append completion receipt.
Synthetic service28916 and browser stopped; exact NTFS root verified without
reparse/active references and removed. No owner backup manually removed.

Local delivery COMPLETE at19:59:46UTC: Workspace113 retained, Finance4 installed,
all5 owned payload hashes and ai.js/coach-ledger.js/style.css/index served bytes
match source. All423 unselected package contracts/existence/bytes, other app
contracts, existing Workspace credentials and configured locations retained.
36 services healthy, no new startup failure. Public HTTPS returns the expected
unauthenticated Workspace sign-in gate. Owner ledger was never a test fixture.

Normal backup382.4MB verified (290.6MB reused,91.9MB newly copied; no live data
hard-links). Normal cleanup completed, retiring one eligible older automatic
backup and retaining the latest verified recovery. Cleanup took over20minutes
while actively verifying many small files; do not interrupt an active healthy
update merely because its completed transaction predates cleanup completion.
Sanitized delivery/served/update-cleanup receipts are in the evidence directory.
No synthetic service/browser/cache remains; build/public assets retained on I.
Second PC remains deferred for this change. No live paid API test was performed.
