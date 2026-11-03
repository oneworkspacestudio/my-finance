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
