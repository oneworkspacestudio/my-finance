# My Finance

Household budgeting for **One Workspace**, powered by **Actual Budget**.

Plan your month in pounds, keep accounts and transactions together, track bills,
set savings goals and share a budget with your household. Your ledger stays on
your devices and your own Windows server.

Open **My Finance** from Workspace, continue with your OneAccount and open your
budgets. Start with a manual account or import a CSV, OFX or QIF statement.
Actual also offers an explicitly fictional demo budget when you choose it.

Bank connectivity uses your own Enable Banking application or an optional Lunch
Flow subscription through SimpleFIN. You approve access on the provider/bank's
own page. Installing My Finance does not link a bank, and no payment actions are
available. Bank support depends on your provider, application and institution.

The current preview is under implementation and acceptance testing. See the
[test report](docs/TEST_REPORT.md) and [limitations](docs/KNOWN_LIMITATIONS.md)
before putting real financial data into it.

- [Windows setup](docs/INSTALL_WINDOWS.md)
- [Connect a UK bank](docs/OPEN_BANKING_SETUP_UK.md)
- [Encrypted backups and restore](docs/MY_DRIVE_BACKUPS.md)
- [OneAccount and household access](docs/ONEACCOUNT_INTEGRATION.md)
- [Security and threat model](docs/SECURITY.md)
- [Architecture and pinned upstream](docs/ARCHITECTURE.md)

MyFinance.exe runs with a bundled runtime archive; no separate Node, Docker,
WSL, Git or Python installation is required to use the packaged app.

Actual Budget's MIT licence and all runtime dependency notices are retained.
