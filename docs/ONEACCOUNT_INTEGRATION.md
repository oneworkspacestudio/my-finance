# OneAccount

The standard Workspace apphost performs OIDC code exchange with PKCE, state,
nonce, issuer and token validation. Finance receives issuer/client/owner/home
settings from the managed runtime. Its callback is /finance/auth/callback.

The engine is a separate private loopback service. Finance overrides incoming
identity headers and binds every native request to the verified issuer/sub hash.
Native API tokens cannot be used to select a different user. Only the configured
Workspace owner receives Actual ADMIN; later first logins cannot become owner.

Members sign in individually. Open an Actual budget, choose User Access Management,
and grant or revoke the member's access. Budget sharing is all-or-nothing at account
level; all authorised members see all transactions in that budget. A read-only
viewer is unsupported. Bank credentials/sync and encrypted server backup controls
are owner-only. Actual's own budget export remains subject to its file ACLs.

Do not change native usernames to reuse another person's identity. Those usernames
are immutable OneAccount identity hashes; display names are presentation only.
