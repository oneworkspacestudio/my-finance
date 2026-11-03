# Limitations and unfinished acceptance gates

- Implementation and native acceptance are in progress. Do not treat this as a
  production-approved financial-data deployment.
- No actual provider sandbox or live bank consent/import has been verified yet.
  Local fixtures are not provider sandbox evidence. Provider setup and human
  authorisation are required before any real bank connection.
- Headless scheduled sync is disabled with the enforced OneAccount/E2EE setup.
  Manual/native browser Sync remains the supported path.
- Bank management is owner-only. Native Actual is not a strong tenant boundary
  for mutually untrusted households. Shared budgets expose every account in them.
  Read-only viewer and private accounts inside a shared budget are unsupported.
- Native ZIP export backups restore through Actual Import budget. Optional daily
  encrypted server snapshots have daily/weekly/monthly retention and a tested
  clean CLI restore; they require explicit Windows DPAPI recovery-key storage.
  Provider credentials must be reconfigured after either recovery path.
- Actual remains the only ledger. Money coach analyses a user-selected summary;
  it does not run unattended, update categories automatically or move money.
  Imported data must be reviewed for completeness, transfers and unusual credits.
  AI responses can be wrong and are educational, not regulated investment advice.
  Provider integration is tested with synthetic HTTP responses; no live paid
  OpenAI or DeepSeek key has been used in acceptance testing.
- The native PWA service worker is disabled on Workspace's shared origin to avoid
  controlling other apps. Previously loaded Actual local budgets can use its
  local-first store; cold-start offline installation is not certified.
- No signed Windows installer, Windows10, clean-host, reboot, power-loss, remote
  production, end-to-end bank or complete master-brief acceptance is claimed.
