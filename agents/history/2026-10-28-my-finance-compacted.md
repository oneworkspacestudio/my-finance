# My Finance implementation

User authorizes attached master brief, new org repository, commit/push/release and
installation on both PCs. Org is oneworkspacestudio; public my-finance created,
Actions disabled. No subagents authorized. Do not claim the full brief is complete.

App source I:/Projects/my-finance, first milestone3cd7a18. Suite integration clone
I:/Projects/workspace-finance, base51aa2f3 (public95). Read its AGENTS: every new
Workspace release must include signed Android through publish-release.ps1 and
promote public updates index. Other chats are actively doing Maps/categories;
fetch/reconcile before suite publication; never reuse a published tag.

Source/pins: Actual26.10.0 /2bebdbaae0b701120a8cad9d56aafd64fa04c577, official
sync-server/web/api npm packages; exact lock. Node24.22.1 LTS archive SHA verified.
Go1.27.3 downloaded automatically; apphost pinned public95/51aa2f3 pseudo-version.
Chose B: native Actual client/ledger and small pinned authentication bridge. Native
OIDC preferred mutable email over sub; native banking callback lacked pending-state
checks. Apphost OneAccount handles issuer/sub/PKCE/nonce; engine requires process
capability and mapped identity; native body tokens cannot override verified actor.
Only Workspace owner ADMIN; members use native budget ACL. Bank routes owner-only.

Implemented Go launcher, protected sidecar extraction, Windows supervised bundled
Node/Actual service, authenticated /ledger proxy, subpath asset/router patches,
exact-Origin mutation checks, session-bound expiring single-use bank states, GB
directory constraint, owner encrypted native ZIP backup/decrypt and optional fixed
My Drive copy with bounded retention. Native ledger untouched. No headless sync or
scheduled backup: explicitly disabled, pending secure OneAccount/E2EE/key design.
Backup wraps native db.sqlite+metadata.json export, not upstream secret databases;
restore downloads verified ZIP for native import into a fresh budget. Full docs.

Go4 tests and vet pass (backup corruption/wrong-key/format, consent state/session/
expiry/replay, route rebase). Native launch initially exposed Go ServeMux pattern
conflict (/ledger/ vs GET /); fixed by explicit methods and rebuilt. First bootstrap
already created synthetic owner even though initial ZIP was incomplete; do not
pass --bootstrap-email when restarting that existing fixture.

Build/cache C:/Projects/.cache/my-finance-20261028 (NTFS); all generated/fictional.
Runtime official npm install there; patched once, native SQLite loads. Packaging
to I: was extremely slow; stopped only task-owned packager and repacked on C.
Completed archive102204877bytes SHA21f44f6a07949693191ba547f6ea2435f73358d60b1149998425c5005c9406bd.
C cache Program has launcher+archive. Release staging I:/Projects/tooling/
my-finance-20261028/release still has older/partial I archive; replace only with
verified completed archive before any release. New launcher build currently uses
development digest; freeze source and rebuild with exact archive SHA for shipping.

Current owned fixture C cache Fixture: synthetic owner@example.test only. Native
foreground exec session89310 starts new launcher127.0.0.1:25016, origin
http://127.0.0.1:25016/finance (avoid localhost IPv6 API resolution). Browser test
test/e2e/native-browser.mjs imports Playwright from C cache, captures only synthetic
evidence. It has not yet passed! Next run with FINANCE_TEST_URL using127.0.0.1.
Inspect native client startup, worker paths/router rebasing and real ledger create/
restart/import/ACL/backup clean restore before publication. Need Actual provider
mock fixture tests; official sandbox/live NOT TESTED, no credentials available.

Owner primary C:/tmp/workspace/Private, remote workspace-2.m8e.co.uk is secondary
desktop, not mini-PC. Saved m8e-pc SSH rejects auth. Existing Chrome remote desktop
tab585734426; updates tabs owned by other tasks cannot be claimed. Create own tabs
for remote installer when ready; coordinate by observing current operations. Never
use live MyAccounts/FileBrowser owner stores as functional test fixtures.

Need suite finance registration: foundationApp, appWorkingDir, Config port base+33
(proposed9023), signins/navigation, update package ownership/storage profiles,
catalog build and org pins, shared icon/README generated branding. Not done yet.
Need synthetic real MyAccounts managed SSO/sharing and signed release, normal
backed-up installs/preservation receipts on both machines. User status answered
honestly: no release/install completion, work continuing. Stop task previews and
keep owner/recovery data; no reboot/clean-host/production claim.

## Native acceptance progress (supersedes earlier pending notes)

Native Windows app now works through /finance/ledger: SQLite worker, static imports,
Router basename, default data manifest, automatic Header method after apphost auth
(active:true was required), new GBP/currency flag/British dates through native
preferences. Root WASM/baseURL and duplicate kcab-prefix errors were fixed.
Further source fixes rebase only rooted asset strings (avoid corrupting external
URLs), handle webmanifest icons, retain zoom accessibility, suppress ledger-worker
payload console logs, and check native page session on focus/every15s. Native secret
server output is discarded. Source per-request display names come from OneAccount;
roles remain configured owner/BASIC. Native local browser copies remain upstream's
shared-profile limitation; docs recommend separate profiles for untrusted people.

IMPORTANT: diagnostic runtime in existing synthetic Fixture/engine/21f44... was
edited temporarily to prove worker/auth/default-currency fixes quickly. Do NOT ship
that folder or count it as frozen-artifact acceptance. Source has the real fixes.
Clean final archive/launcher must be installed into a NEW synthetic fixture and
all relevant flows rerun from exact shipping bytes before release.

test/integration/ledger-backup.mjs PASSES9 checks using official Actual API and real
native engine via test-only authenticated proxy: opening123456p, monthly budget,
100 transactions ten imports, manual edit preservation, stable-ID pending/booked
no duplicate, encrypted native export, identical authenticated decrypt, complete
ledger/categories/accounts/balances/monthly budget restore into clean Actual data
folder, restart persistence. Synthetic only, no live owner/Drive/account data.
Harness initially needed mkdir and groupId (Actual's documented sync ID); signed
native totalBudgeted is-10000 for a10000p allocation, corrected test expectation.

test/integration/household-access.mjs PASSES six checks: BASIC native actor, private
file list/download denied, member bank/backup/admin denied, exact Origin required,
native budget sharing grants file, revoke denies file. Uses synthetic local apphost
owner/member sessions, NOT real MyAccounts OIDC. Native provider mock PASSES directory,
AIS consent/session,100 tx across two pages,123456p balance, member banking denial,
forged owner-native-token denial. Actual mock provider HTTP intercept only; official
sandbox and live remain NOT TESTED. Raw native bank balance float conversion was
patched to exact decimal/BigInt minor units; repeated/exhausted pagination errors
fail instead of returning partial success. No background scheduler delivered.

Browser welcome desktop/360px screenshots reviewed; native GBP budget/account works,
1234.56 opening and28/10/2026 date visible. Native CSV preview shows two fictional
transactions. Recent test corrected its completion wait (preview text was present
before applied import); must await Import2 button hidden and confirm ledger rows.
test/e2e/native-browser.mjs uses a dedicated synthetic persistent Chrome profile.
Initial anonymous assertion was corrected to use cookie-free Node fetch after
switching to persistent fixture profile. No actual user browser profile as fixture.

Npm audit:3 MODERATE records (sync-server->disabled Akahu->uuid buffer-bound-check
advisory); zero high/critical. Review/document residual reachability; no version
downgrade. Source server blocks Akahu/Pluggy/GoCardless, safe unavailable status
responses avoid native manual-account discovery errors. Need final scan/evidence.

Suite finance registry/storage/ports/signins/navigation/update ownership/build args
and central icon/README directory added. Catalogue, distribution, suite and apphost
tests all pass. Generate PNGs with scripts/sync-branding.cjs --local-only using
SHARP_MODULE=C:/Projects/.cache/my-finance-20261028/tools/node_modules/sharp; it also
changed unrelated generated icons due encoder version, which were restored. Only
Finance artwork + overview/index/README should remain. Central £wallet source matches
app source. Current overview app-suite-b844350d.png. build-org Finance pin still
needs actual released asset hash; compatibility target97 provisional, check collision.
Fetched suite origin/maincb29f96 (Maps11 and categories source) but local changes
not yet committed/rebased; preserve new origin changes. Other chats use same org.

Final runtime repack session7490 wrote C cache/finance-runtime-final.zip, still
running at last check (~93MB); bridge source changed display-name update afterward,
so restore pristine native app chunk, reapply final patch and repack once more.
Current dev binary has not incorporated latest Go transforms/session monitor/log
suppression/icon. Need format/build with hash-pinned final archive and full tests.
Keep artifacts/staging on I but package on C for speed. No release yet; first source
milestone is pushed. No live installation altered. Real OneAccount fixture, clean
shipping-byte browser/Drive verification, signed Workspace release via required
Android policy, channel promotion and both protected owner installs still remain.


## Shipping candidate verification (supersedes earlier pending backup/SSO notes)

New source adds explicit DPAPI-protected provider permission metadata/check/revoke
and optional daily credential-free encrypted server snapshots with 7/4/6 retention.
Recovery key remembered only after explicit CurrentUser DPAPI consent. Native HTTP
quiescence + online metadata backup, secret/session scrub/VACUUM/integrity; opaque
E2EE blobs retained. CLI clean restore refuses any existing budget history. No bank
headless worker; remains explicitly deferred. Docs and full FIN01–22 matrix updated.

Candidate runtime now deterministic fixed ZIP metadata, hash f3c4675bf401042356310325a35de736de3c7b2f53aab2e74b611839f98da3ca.
Hash-pinned launcher verified on NEW ShippingFixture, not modified diagnostic engine.
Go tests/vet pass; provider mock passes consent/pagination/exact balance/check/revoke,
actor/member protection and credential-free snapshot. Official API passes ten
ledger/dedup/manual-edit/pending/credit-transfer/export/clean restore/restart checks.
Drive encrypted copies verified. Server snapshot scheduled browser-closed test,
clean CLI restore/file equivalence/credential scrub/overwrite refusal/key removal
all pass. Restored server boots and exposes its budget to the mapped administrator.
Native applied CSV confirmed £1224.72; 360/768 no overflow screenshots reviewed.
CSV harness corrected native Csv capitalisation and loading race. Full keyboard-only
editing, public callbacks, outage/429, power-loss/reboot remain unverified.

Fresh real MyAccounts/Workspace/Finance managed fixture3 passes six browser checks:
real code/PKCE -> Finance -> native owner ADMIN, BASIC member isolation, share/revoke,
forged callback denial, member signout with independent owner session preserved.
First fixture caught missing runtime in catalog.InstallPackageIDs; source fixed
and regression passes; fresh fixture3 verifies four packages including archive.
Suite source rebased origin ecf9d74, own commits5785eb2/1ba96e5 (not pushed yet).
Do not rerun setup before dependent build has completed. All fixture users fictional.

Runtime includes GPL3 HyperFormula3.5.0. Full GPL plus Actual MIT/notices collected;
matching Actual26.10 commit source archive and HyperFormula3.5 source downloaded.
Publish source archives beside binaries; own gateway remains separate MIT process.
Go govulncheck zero affected symbols/imported packages, three unreachable module
records; npm zero high/critical, three moderate disabled Akahu/uuid records.

No public release or owner installation yet. Next freeze clean app source, build
final launcher against f3c runtime, verify final bytes, publish app + source/notices/
managed manifest, pin suite, build/publish signed Android suite and promote index.
Then normal backed-up primary + DESKTOP-Workstation installs, preservation receipts.
User confirmed DESKTOP-Workstation is workspace-2 and will unlock Chrome Remote
Desktop PIN themselves. Own tab585734450 marked handoff. Do not claim remote access
until connected. No credential guessing. No Actions, no subagents.

Suite97 became public during work; current main also adds Forms using base+33.
Finance moves to base+34/default9024 and minimum planned Workspace100 to avoid
concurrent release collisions. App compatibility updated; final binary must
record the latest clean commit. Suite rebase preserves Forms/Drive broker paths.

Final launcher restart caught an ordered migration bug before publication: calling
apphost.Migrate(1) against schema2 is rejected as a downgrade. Finance now checks
latest supported schema, skips already applied steps, and still refuses future
schema3. Regression closes/reopens the store and verifies settings preserved and
future-schema rejection. All Go tests pass. Old AppRelease/Final binaries built
from0ce5731 are NOT releasable; rebuild and rerun restart/ledger/snapshot checks.

Release1 is now PUBLIC at tagv0.1.0-preview.1 from f85eeab. Final executable
SHA3eb8a47edded3df880136197815bfbd099d8b27a164538dba172c15edc0af1ed,
portable104778580bytes SHAff42aa929855eaf0422a4f1ba55857b10ae54edb8b5e84c325d719b7ac3b9004.
All ten uploaded asset sizes/GitHub digests checked; anonymous download matches.
Full source/licences/Actual+HyperFormula preferred sources provided. Final fixed
launcher boots existing schema2 data; ledger10/ACL6/snapshot6 checks pass. Defender
old candidate directory and final rebuilt executable found no threats. Nothing
installed on owner hosts yet. Suite100 publication/delivery still outstanding.

## Supplemental acceptance and final suite rebuild

Shipping bytes also pass repeated native CSV review (0 new rows, no date filter),
keyboard-only Tab/Enter notes/payment save (101p; cash122371p), and a fresh real
Workspace100/MyAccounts/Drive fixture: encrypted snapshot files visible in My Drive;
UI download SHA matches Finance bytes. Exact stateful harnesses and synthetic
screenshots/report are under test/e2e and docs/evidence. Syntax checks pass.
No owner or real banking fixtures were used. All task-owned test services stopped.

Suite100 first candidate remained unpublished. New origin2252e9b adds Forms2 and
a critical protected-update startup fix, so canceled only own draft/uploads before
publication, retained upstream changes and froze effdc0e for a rebuild. Supported
signed Android publisher and public-channel verification must complete before
normal protected delivery. App1 remains immutable/published/promoted. Primary
still96/30apps at last read; secondary Forms98/Forms2 per its handoff. Inspect actual
state before installing, preserving credentials/contracts/bytes and recovery.
Chrome DESKTOP-Workstation remains at PIN; user must connect it personally. Do not
claim both installations, provider sandbox/live, clean-host or reboot qualification.

Primary protected installation96→100 completed on29OctoberUTC from exact verified
offline effdc0e candidate while its GitHub upload continues. Normal updater verified
1.47GB recovery, kept newest rollback, all31 services healthy. In-memory comparisons
prove all30 old app contracts,143 package records checked/138 unselected payload
bytes, all prior owner/settings/credentials retained. Finance1 EXE/runtime match
public app release. Direct/public anonymous ledger denied, OneAccount configured,
native engine running, deployed blue icon hash matches central source; Forms ready.
Real Chrome HTTPS Finance welcome and native Actual entry reuse existing signed-in
session without second password; no budget created or owner banking accessed.
Sanitized delivery/runtime/browser/cleanup reports are under docs/evidence.

Signed Android6 sourceeffdc0e SHA6adc13fa722b4aea9fdf3cbfb53c84705b09284b3ce661d4a89c4dedc2415b63.
Supported publisher draft427317950 uploading remaining large assets; must verify
public asset hashes/exact index/anonymous update check before announcing release.
Retained candidate and source on I; removed own C cache only after process/reparse
and source-copy checks. All synthetic services stopped; owner services untouched
by cleanup. User connected the other chat's reserved remote tab; this chat's own
connection still needs PIN. User explicitly chose continue with local machine for
now, so secondary installation remains deferred, never claim both completed.
