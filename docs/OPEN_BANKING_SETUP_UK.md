# Connect a UK bank

1. Register your own Enable Banking application. Begin with its sandbox mode.
2. Set the exact Finance callback shown on the Bank connections page:
   https://YOUR_WORKSPACE/finance/ledger/enablebanking/auth_callback.
   This preserves the native /enablebanking/auth_callback suffix behind the proxy.
   OneAccount's /finance/auth/callback is a different flow.
3. For restricted production, activate by linking your own permitted accounts in
   Enable Banking's control panel. Only whitelisted accounts can be fetched.
4. Securely enter your application ID/private-key credential file in Actual's
   More → Bank Sync → Set up Enable Banking. Do not paste them into chat or logs.
5. In an Actual account choose Link account → Enable Banking, select a UK bank
   offered by your configured application's live directory, and start consent.
6. Complete the bank/provider's own page yourself, select the accounts, check the
   opening balance/history strategy and review the first import for duplicates.
7. Use native Sync while your budget is open. Check the actual imported records,
   account balance and consent expiry; a successful HTTP response alone is not proof.

The gateway rejects unrecognised/foreign/replayed states before provider exchange
and requires the exact HTTPS callback. Restart/expiry requires starting again.
Do not substitute another callback path or strip the native route in the proxy.

This native integration is experimental. No credentials are bundled. Provider
sandbox/live evidence is separate from local mocks in TEST_REPORT.

For another household member's account, confirm the provider permits that account
under your application; otherwise use their separately authorised provider setup
where supported or the optional fallback. Do not bypass restricted-use rules.

Optional Lunch Flow: create its SimpleFIN Bridge destination, select your accounts,
generate a one-use setup token and paste it into Actual's SimpleFIN setup. You need
your own subscription and current institution coverage. Revoke/rotate in Lunch Flow;
disconnecting a ledger account does not necessarily revoke provider access. Keep
already imported data unless you intentionally delete it.

Manual CSV/OFX/QIF import is always available. GoCardless is not offered as a new-user
default. No app page collects a bank password or requests payment permission.
