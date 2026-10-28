# Encrypted backups and clean restore

1. In the native budget choose Settings → Export data and download its ZIP.
2. In Finance → Backups choose the ZIP and a recovery passphrase of at least 12
   characters. Download the authenticated encrypted .mfb file. If Workspace has
   configured its dedicated backup destination, optionally copy it to My Drive.
3. Keep the recovery passphrase separately. It is never saved by Finance.
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
Daily/weekly/monthly tiering and unattended backups are not implemented/enabled;
they require a separately protected recovery-key design.
