# Architecture decision 001 — retain Actual, authenticate at the native Workspace boundary

28 October 2026. Actual 26.10.0, upstream commit `2bebdbaae0b701120a8cad9d56aafd64fa04c577`, MIT.

Inspected the published sync-server, native React client, One Workspace apphost,
catalogue, runtime supervision, MyAccounts OIDC and protected update mechanisms.
One Workspace uses per-app loopback processes, issuer/client credentials, PKCE,
stable subjects, per-app cookies, protected NTFS data and a path-based HTTPS proxy.
My Drive is FileBrowser Quantum; an application must not assume its entire files
root belongs to the currently signed-in user.

Pattern A would preserve the whole engine but native Actual OIDC currently chooses
preferred_username/email before sub, resolves callbacks from the hostname root,
and its Enable Banking callback exchanges codes without first requiring a pending
state. These are incompatible with this brief without changes. Pattern C would
duplicate a mature local-first UI and increase ledger risk. Choose pattern B:
unmodified Actual ledger/client, a small source-pinned server authentication bridge,
and a Go launcher/integration gateway built on Workspace's apphost.

The gateway is the only public surface. OneAccount verifies code+PKCE+nonce and
maps issuer+sub to a stable identity. Every engine request receives a fresh trusted
identity and a process-random capability over loopback. Incoming identity/token
headers are discarded. The patched server refuses requests without that capability,
maps identities only into Actual's authentication tables, and uses native file ACLs.
It never writes directly to ledger tables. Only the configured Workspace owner is
an Actual administrator. Members use native budget sharing; bank administration
is owner-only because upstream bank credentials are instance-wide. Read-only roles
and account-level privacy inside shared budgets are unsupported.

Actual runs under a kill-on-close Windows job with a bundled Node 24 LTS runtime
and native SQLite sidecars. No Docker, WSL or end-user Node installation. All
writable state is in the private per-installation folder. The portable distribution
has MyFinance.exe plus a runtime archive: this is not a single-file runtime.

Enable Banking and Lunch Flow/SimpleFIN reuse native adapters. An additional gateway
gate binds native consent states to the initiating app session/user and exact callback,
expires and consumes them, and rejects replay. Country selection is constrained to GB.
No provider credentials are discovered from other apps or used automatically.

The native browser engine owns transactions, matching, splits, transfers, schedules,
rules, envelope budgets, goals and reports. No parallel ledger is created.
Headless scheduled bank sync remains disabled until its supported API can securely
operate with enforced OneAccount and E2EE without saving a decrypting password.

Finance backup archives are authenticated AES-256-GCM with an Argon2id passphrase.
Only native budget exports are accepted: no server credential database is copied.
The user exports a budget through Actual, encrypts it in Finance, and restores by
decrypting and importing into a fresh Actual budget. Owner-only My Drive copies use
a configured dedicated folder, with link/traversal checks; no automatic per-user Drive
mapping is presumed. Optional daily server snapshots quiesce native requests, use
SQLite online backup for metadata, scrub credentials/sessions from that copy and
retain opaque native budget/sync files. An authenticated encrypted manifest records
allowed paths and SHA256. Scheduling requires explicit CurrentUser DPAPI recovery
key consent and remains disabled on installation. Clean CLI restore refuses existing
budget history. Daily/weekly/monthly retention deletes only hash-receipted copies.
