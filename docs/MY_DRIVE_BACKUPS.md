# Encrypted backups and clean restore

1. In the native budget choose Settings → Export data and download its ZIP.
2. In Finance → Backups choose the ZIP and a recovery passphrase of at least 12
   characters. Download the authenticated encrypted .mfb file. If Workspace has
   configured its dedicated backup destination, optionally copy it to My Drive.
3. Keep the recovery passphrase separately. Manual-export passphrases are never saved.
4. Verify/restore by choosing the .mfb and its passphrase. Finance authenticates
   and decrypts it, then downloads the original native budget ZIP.
5. On a fresh installation/browser choose Import budget and select that ZIP.
   Check accounts, categories, exact balances, budgets and transactions. Existing
   budgets are not overwritten. Remove temporary plaintext exports securely using
   your normal device-data policy when finished.

Format: MYFINANCE-BACKUP-1 header, 16-byte random salt, Argon2id (64 MiB, three passes,
parallelism two), 12-byte nonce and AES-256-GCM ciphertext. Header is authenticated.
Corruption, wrong passphrase and truncation are rejected. The .mfb is not a ZIP.
64 MiB compressed/uncompressed export limit in this preview.

Only native db.sqlite+metadata.json exports are accepted. Server databases, private
RSA keys, OneAccount configuration and provider credential stores are excluded.
Budget data can retain linkage references; reconnect providers after restoration.

MYFINANCE_BACKUP_DIR is the fixed owner-only server destination supplied by Workspace,
usually its My Drive/One Workspace Backups/My Finance directory. Browser requests
cannot choose arbitrary paths. Links/reparse points are rejected. Copies are verified
byte-for-byte before retention. MYFINANCE_BACKUP_RETENTION is 1–366 copies, default 7.

For unattended protection, Finance also offers **server snapshots (.mfs)**. These
preserve native server budget blobs, sync messages and sharing metadata, including
opaque E2EE content, without opening/decrypting budgets. A quiescent snapshot copies
the native account database through SQLite's online backup API, removes provider
secrets/authentication/login sessions from the copy, vacuums it and checks integrity.
The running ledger and credential store are not edited. Busy bank setup requests
cause a retryable failure rather than an inconsistent snapshot. Preview limit:
128 MiB of snapshot files. Keep a native export as an independent recovery option.

Create one now under Backups, or enable the optional daily schedule. It is off by
default. Scheduling requires your explicit acknowledgement to remember the recovery
passphrase with **Windows CurrentUser DPAPI**. It can only be opened by the same
Windows user/device; it is unrelated to the separate budget E2EE passphrase. Keep
the recovery passphrase separately for recovery on another machine. Disabling the
schedule deletes the remembered key. Windows-user/owner changes require enabling
the schedule again. The app must be running; missed runs resume when it starts.

Verified encrypted copies go to the private snapshots folder and, when selected,
the configured My Drive folder. Default retention is seven daily periods, four ISO
weeks and six months, with one latest snapshot per period. Files are deleted only
when their saved receipt/hash matches; unknown or owner-modified files are kept.
Status records the last successful snapshot, next run and safe failure category.
Failures retry after 15 minutes; successful runs schedule the next day.

Restore a server snapshot into a **new empty stopped** private folder. Stop the
target Finance process first. Set MYFINANCE_RECOVERY_PASSPHRASE in that local
process environment, run MyFinance.exe with `--data NEW_ABSOLUTE_NTFS_FOLDER
--restore-snapshot SNAPSHOT_PATH --confirm-restore`, then remove the environment
value. Never type a real recovery passphrase into shell history. The launcher
checks authenticated contents, paths, hashes, SQLite integrity and credential
exclusion before promoting the staged folder. Existing budget history is refused.
An old empty server folder is retained for rollback. Start with the same managed
OneAccount issuer/subjects so sharing identities continue to match. Reconfigure
providers and snapshot settings. E2EE budgets still require their own passphrase.

Workspace's separate full-installation recovery is a different mechanism: it can
include the private credential/configuration store and must stay protected. The
credential exclusion above applies specifically to Finance .mfb/.mfs backups.
