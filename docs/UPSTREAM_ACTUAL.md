# Pinned upstream

Actual 26.10.0 / `2bebdbaae0b701120a8cad9d56aafd64fa04c577`.
Official npm packages @actual-app/sync-server, @actual-app/web and @actual-app/api,
all 26.10.0; exact transitive packages and integrity hashes in package-lock.json.
Node 24.22.1 LTS is bundled. Keep upstream and Node licences in the runtime package.

See scripts/patch-actual.mjs for the complete, narrow server bridge. It verifies
the upstream version and exact patch anchors, refuses already altered/unknown
sources, and leaves accounting logic untouched. Upgrading upstream requires
reviewing the patch and rerunning native auth, file ACL, import and callback tests.

References checked during implementation:
- https://github.com/actualbudget/actual/tree/v26.10.0
- https://actualbudget.org/docs/config/oauth-auth/
- https://actualbudget.org/docs/config/multi-user/
- https://actualbudget.org/docs/advanced/bank-sync/enable-banking/
- https://actualbudget.org/docs/api/reference/
- https://enablebanking.com/docs/api/reference/
- https://enablebanking.com/docs/api/linked-accounts
- https://www.lunchflow.app/docs/guides/destinations/simplefin-bridge

The documented Enable Banking integration remains experimental. Restricted production
permits only individually whitelisted own accounts; do not infer permission to access
another family member's accounts. Lunch Flow requires the user's subscription and
supported institutions. No price or universal UK coverage is hardcoded.
