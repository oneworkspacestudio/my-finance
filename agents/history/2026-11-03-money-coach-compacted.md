# Money coach, 3 November 2026

User requested AI help with income/outgoings, buckets, savings and investments,
supporting their OpenAI or DeepSeek API key. Earlier delivery authorization still
applies, local PC only; second PC was explicitly deferred. Keep Actions disabled
and public assets immutable. Actual remains the sole ledger; owner data is never
a test fixture.

Implementation adds #coach mobile UI, manual estimates and read-only native browser
summary. User opens/unlocks a synced budget and clicks Use this budget; fixed
native worker reads aggregate only. Account and cloud-file ACL checked before and
after, selected budget stable, 15-minute selection TTL and 3-minute one-use summary
handoff in sessionStorage. No raw transaction/payee/notes/account data leaves the
browser. Existing labelled starting balances, transfers and off-budget accounts
excluded; renamed/localized starting entries need review. Refunds are gross credits.
Desktop Actual worker adapter has no removeEventListener; use one bounded pending
dispatcher, with regression coverage. Selection helper minimizes for native mobile
controls and checks the currently displayed budget.

Go schema3 adds per-user/provider DPAPI key envelopes to private apphost DB. Require
and CSRF protect settings/plan/analyse; expected-account header rejects split sessions.
No key readback, shared owner key, conversation persistence or background AI calls.
Arithmetic reserves max(average spending, stated essentials) against lowest monthly
income. Debt/emergency/goals precede remaining buffer; investment readiness is only
an educational checklist, requires spare money after goals and actual current data.
No model-controlled ledger writes, payments, trades or security recommendations.

Fixed HTTPS OpenAI Responses(store=false, default gpt-4.1-mini) and DeepSeek Chat
Completions(JSON, thinking disabled, default deepseek-flash). Models configurable;
90-second timeout, no retries/redirects, capped input/output, per-user cooldown/hourly
limits, sanitized failures. Explicit per-request preview/consent. Frontend uses
textContent, invalidates stale data/advice and clears financial fields on account
change/logout. Live paid provider calls not tested; user supplies key in app.

Go tests/vet, synthetic provider/auth/DPAPI/migration regressions and UI acceptance
pass. Native integration runs on new NTFS fixture48641 using unchanged verified
public Finance2 runtimeSHAf3c4675bf401042356310325a35de736de3c7b2f53aab2e74b611839f98da3ca.
Final native/full shipping-byte receipt and release/install status to append below.
Build version3 now consistent in main/package/compatibility; build-windows passes
version explicitly. scripts/package-release.py retains verified runtime/licenses,
requires clean committed source, regenerates public source/manifest/checksums.

Primary baseline actually Workspace113/Finance2/37apps, not old100; public113
installer hash guarded in tooling install script. Preserve every unrelated app,
payload, identity/credential and recovery copy through normal Finance-only update.
No new suite release is needed for unchanged package/route/runtime contract. Public
channel and organization component pin must advance after app publication.

Final source acceptance: real native Actual browser integration passes exact
three-month600100p incoming/300000p outgoing,7included transaction lines,
2excluded transfer legs, standard opening balance/tracking exclusions, no provider
call and identical ledger before/after. 320/360/390px helper and minimise/restore
pass. UI320/390/768/1280 passes, including provider-save/delete cancellation races.
Fixtures are strictly fictional. Production key/provider use awaits user setup.
Evidence: docs/evidence/ai-20261103. Native browser closed after verification.
