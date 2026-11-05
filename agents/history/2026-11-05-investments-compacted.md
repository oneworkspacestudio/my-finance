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

Public Finance5 is released fromc1b2bc75179c25a2a0c9ec6bbfc92b983bf0a875;
all10 GitHub asset digests and anonymous checksum bytes verified. PortableSHA
28bb36eb27fd8cb040ad8de6641862af61859df0d9cfb8281547b00816696a43,
EXE9d1d226f3ff8a4a49813d0f9d1e7e8e3323748fc4f37ac74968aa45644d9b03f.
Suite d7cdffb promotes5/channel; exact anonymous raw-main URL includesall10.

Found/corrected separate ordinary-build bug: managedSeed ignored myFinance pin,
so public114 fresh setup containsFinance1 despite prior4pin. New strict Finance
component mapper consumes5, matchesfivepayloads and preserves36 unrelatedapps/
446 packagecontracts. Legacy guard2→3 corrected. Candidate planner tests cover
selection/omission, customportretention, missingruntime and tamper rejection.

Real public114 fresh accounts+finance setup, OneAccounts sign-in, supported1→5
upgrade and rollback5→1 pass. Original fictional Actual ledgerhash/identities/
decrypted credentials/paths/unselected bytes retained, fivepayloads and15recovery
files verified. Native exact history600100/300000 andcurrent200345/17345 pass;
real paper setup/prices/reload/missingkey/consent/realticker guards andfiveviewport
sizes pass. No paid/provider calls. Reapply5/finaldelivery receipts toappend.

Fixture runtime cache contains16546files; first pretransaction backup was slow
and safelycancelled only on the disposable fixture. Alloriginalprograms andfour
Actualserverfiles remainedhash-identical. Removedonly its incompletebackup and
deterministicallygeneratedenginecache afterpath/process/reparse checks; normal
updaterthenverified31.6MBrecovery and re-extractedpinnedruntime. This is cold-cache
syntheticacceptance, never a workaround forownerdata/recovery. DPAPIciphertext
re-randomizes duringnormalupdate; preservationcompares plaintext onlyinRAM.

Final cold-cache lifecycle1→5→1→5 now COMPLETE. Reapply5, native ledger and real
paper tests passed again; all five served UI assets, shipping EXE/runtime and
schema6 verified. Managed fixture processes/listeners/browser are stopped; guarded
NTFS managed root removed at12:47:22UTC. Sanitized reapply/shipping/cleanup receipts
retained in evidence. Public test programs and reports stay onI. Root primary
normal protected Finance-only delivery started after cleanup, with no owner cache
removal. Final ordinary QA115 bundle verification and owner receipt pending.

Ordinary suite build verification is now complete. scripts/rebuild.ps1 from
d7cdffb4b205312bd324f7690d866ec6c1973d40 produced local QA115 with 37 apps and
451 packages. Every archive member size/SHA and the complete catalog audit pass;
Finance's five payloads exactly match public release5. See managed-build-proof.json.
QA115 was not published or installed. All test processes and the remaining empty
NTFS task cache were removed; evidence and public test programs remain on I:.

Primary delivery is COMPLETE at 2026-11-05 13:14 UTC, superseding all pending
owner-install notes above. Normal public114 Finance-only update 4→5 completed,
all five installed payloads/served assets match, 423 unrelated package records
and bytes plus other apps/settings/locations/existing credentials are preserved.
All 36 services run with no new failures. Normal 382.5 MB recovery was verified;
automatic cleanup retained it, removed one older automatic backup and finished
without warnings. Managed OIDC bootstrap/index match; anonymous public HTTPS
requires Workspace sign-in. No owner ledger opened or owner cache manually removed.
Sanitized delivery/served-verification/update-cleanup receipts are in evidence.

Current outcome: Finance5 is public, its channel/pin are promoted, and the primary
PC has it installed. Second PC remains user-deferred. No real trading/transfers,
investment account creation, paid model calls or official broker/bank sandbox/live
qualification are claimed. Future work needs an explicit scope; do not resume the
deferred remote target or change immutable assets based on older pending notes.
