# Investment workspace, 5 November 2026

User asked for an LLM with agreed funds/goals to execute investments and bank
transfers autonomously. Explained boundary and implemented the safe alternative:
finite consented PAPER automation, read-only Trading212 portfolio connection and
manual bank/broker handoff. No real order, payment or investment-account creation
endpoint. Actual remains sole real ledger; do not portray simulation as live
investing. Original push/release/local-install authorization continues. Other PC
remains deferred by user; other chats' completed app deliveries do not revoke it.

Paper schema5: one per-user DPAPI envelope, three fixed fictional baskets, manual
GBP scenario prices, whole units/integer pence, user initial/target/loss/order/
position/attempt limits. No leverage/shorts. Atomic decisions preserve exact
cash+costbasis=initial+realized. Bounded50 audit. At most20 lifetime attempts and
15min session/model-bound run consent; existing10/hour30sec AI throttle shared.
Attempt persisted before paid call. Browser drives approved finite steps>=31sec;
no unattended scheduler or automatic resume. Revision checks and tagged inflight
cancellation discard stale pause/reset/expiry/logout/restart completions. AI busy
remains set through result postprocessing. Price-rise overcap permits holding or
reducing exposure, not adding it. No market quote/realistic fee/tax/FX simulation.

Broker schema6: user/environment-bound DPAPI key pair, explicit refresh, only fixed
GET /equity/account/summary and /equity/positions at demo/live.trading212.com.
Account+Portfolio read permissions requested, no permission introspection claim.
No proxy/redirect or URL passthrough; bounded strict numeric/UTF8 response checks,
30sec refresh limit and postfetch credential/environment check. Totals use actual
account currency; quantity-only positions avoid guessing instrument FX. IDs,
credentials and broker data never enter model requests; no snapshot persistence.
Removal is local only; provider revocation is manual. Bank AIS is not PIS consent.

API/UI contracts in source and docs/INVESTMENTS.md. New #investments route retains
mobile hamburger/nav/buttons. Frontend clears account data and aborts steps on
pause; request errors don't authorize retries. Optional api signal parameter keeps
existing callers unchanged. New risk field renamed paperRisk to avoid ambiguous
existing coach selectors. All new model/provider text uses textContent.

Verification: full Go/vet,700scenario paper sweep and WindowsDPAPI/auth/race tests;
broker fixedtransport/binding/malformed/inflight tests;9native-summary Node tests;
focused suite catalog/distribution pass. UI/new manager regressions and managed
shipping checks append on completion. Only synthetic NTFS fixtures/mocked
providers; no paid AI or official broker/bank sandbox qualification claimed.

Target version5, same5 payload contracts, runtimeSHA
f3c4675bf401042356310325a35de736de3c7b2f53aab2e74b611839f98da3ca.
Minimum manager100/integration3 unchanged; data schema6. Private paper/key records
excluded from Finance native exports/sanitized snapshots, included only in normal
protected Workspace recovery. Corrupt/unopenable paper records require compatible
recovery; destructive per-user delete UI isn't implemented. Valid experiment can
be explicitly replaced/reset. Published assets immutable, Actions disabled.

Reconciled suite main1f505e0 and actual primary Workspace114/Finance4,37apps.
Use verified public114 installerSHA
208be163bf3b3180eb9e1e80916226448c68219b0ece319b92f9878634f4e125.
Tooling I:/Projects/tooling/finance-investments-20261105. Disposable NTFS root
C:/Projects/.cache/finance-investments-20261105; managed fixture24620, programs onI.
Actual-data agent owns synthetic setup/processes; root owns release/real delivery.
Source publication, channel/pin, exact final managed upgrade and protected owner
install pending. Preserve all unrelated apps/credentials/data/latest recovery.
