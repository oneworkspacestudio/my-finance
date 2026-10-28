# Windows setup

The portable Windows x64 package contains MyFinance.exe and finance-runtime.zip.
Keep both together in the selected application directory. The launcher extracts
its hash-pinned Node/native SQLite sidecars into the protected private data folder.
This is a portable directory with a one-click executable, not a single-binary runtime.

For One Workspace, use the managed app registration and its normal backed-up
updater. It provisions the OneAccount client, chooses a free app port, starts/stops
Finance with Workspace and retains financial data across upgrades. Existing
Workspace autostart applies; no separate admin service is required.

For standalone evaluation, double-click MyFinance.exe. Data defaults to your
per-user AppData/OneWorkspace/MyFinance folder. First setup uses the private one-time
link printed/opened by the launcher. Managed installations use only OneAccount.

Development flags: --data ABSOLUTE_NTFS_FOLDER --listen 127.0.0.1:5016
--origin http://localhost:5016/finance --no-browser. Check the port is unused first.
Remote origins require HTTPS. Do not expose the process directly on a LAN address.

Uninstall removes the application binaries only. Keep the private data directory,
native browser budget and recovery backups unless you deliberately delete them.
Close the app before replacing binaries; managed upgrades use Workspace's verified
backup/rollback transaction. Do not run two launchers against the same data folder.

Only Windows 11 x64 is targeted. Windows 10, clean-host and physical reboot support
must not be inferred from a local runtime test.
