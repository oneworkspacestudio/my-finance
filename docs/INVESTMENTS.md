# Investments

Open **Investments** from the Finance navigation. This workspace contains an AI
paper portfolio and an independent read-only Trading 212 view. It does not submit
real orders, create investment accounts, initiate bank payments or grant the LLM
access to a trading account. Actual remains the ledger for real finances.

## Agree the experiment

Choose virtual starting funds and a target portfolio value. Set maximum
loss, maximum order and position sizes, and the number of AI decisions you allow.
Choose from the three fixed fictional equity, bond and cash baskets and enter
their GBP scenario prices. Starting funds are
simulated: no bank transfer occurs and the amount does not reserve real cash.
The target is an experiment objective, not a forecast or promised return.

Each run needs explicit consent to send the displayed paper portfolio, goal and
limits to the selected AI provider. Existing personal OpenAI/DeepSeek keys from
Money manager are reused. Provider charges apply; request counts are bounded but
are not a provider spending cap. Configure a financial API usage limit with the
provider if required. Opening this page never starts a paid run.

During a run the model may propose paper buys, sells or holding cash. The server
checks the entire decision before applying it to the virtual portfolio. It uses
whole units and integer pence, rejects borrowing, short sales, unknown instruments
and orders beyond the agreed limits. The model cannot increase its funds, change
the experiment rules, change prices or access a broker/payment tool.
If a manual price change lifts an existing position above its limit, the model
may hold or reduce that position through bounded sales; it cannot add exposure.

The page runs a finite sequence of decisions at least 31 seconds apart. Pause,
leaving the page, signing out or a failed decision stops the sequence. Reloading
does not resume paid calls automatically. A server restart also invalidates an
unfinished run. Review the activity history and remaining decision allowance
before approving another run. A request already sent may still be charged if
you pause; a stale response cannot overwrite the newer paused state.
Each approval expires after fifteen minutes and applies to the current sign-in
session. A portfolio permits at most twenty AI attempts across its runs, while
the shared AI limit permits ten requests per hour. Failed, rejected and interrupted
provider attempts remain counted. The count is saved before the provider call,
so a server restart does not silently restore a consumed allowance.

Update scenario prices explicitly to explore a price change. Prices are supplied
by you, are not a live feed and do not move just because the model trades.
Consequently repeated decisions at unchanged prices cannot generate market
profits. Simulation does not model real execution, spreads, liquidity, dividends,
tax, corporate actions or FX. Results do not establish an investment's suitability
or predict performance. Loss limits stop further paper decisions; they cannot
guarantee a maximum real investment loss.

## View Trading 212

In the broker card choose **Demo (practice)** or **Live**, then enter that environment's
API key and secret. Create a dedicated key with only **Account** and **Portfolio**
read permissions. Leave trading permissions disabled. This app cannot inspect or
certify a key's complete permissions, but its adapter only issues the two fixed
GET requests for the account summary and open positions. It has no order, pie,
payment or withdrawal endpoint.

Refresh explicitly to read the account. The displayed totals use the account's
primary currency, which may differ from the GBP paper experiment. Position rows
show quantity; instrument prices are not multiplied into a guessed GBP value.
The demo/live badge and fetch time identify the snapshot. Provider data may
change after refresh; it is not an execution quote. Broker data is not copied into
the simulation, saved as history or sent to the LLM.

The official API supports Invest and Stocks & Shares ISA accounts, subject to
your provider account and API availability. This connection has been tested with
synthetic HTTP responses; no official provider sandbox or live account is claimed
as verified. See [Trading 212's API documentation](https://docs.trading212.com/api),
[account summary](https://docs.trading212.com/api/accounts/getaccountsummary) and
[positions](https://docs.trading212.com/api/positions/getpositions).

Removing the key deletes Finance's local protected copy. To revoke the API key
itself, use Trading 212's settings. A read-only refresh is separate from giving
the AI permission to send a paper-portfolio request.

## Funding and real investments

Use the funding checklist to decide an affordable amount after bills, debt and
emergency savings. Review current UK guidance in Money manager. To fund an account
or place a real investment order, open the provider's own app and confirm the
details there. An amount written in this workspace is a plan, not an enforceable
bank mandate or permission to transfer money.

Actual's existing bank-data consent does not authorize payments. Payment
initiation would require a separately configured provider and its bank consent
flow. It is not included here. Official background:
[Enable Banking payment FAQ](https://enablebanking.com/docs/faq/),
[Open Banking variable recurring payments](https://www.openbanking.org.uk/variable-recurring-payments-vrps/).

## Privacy and recovery

Paper state, agreed rules and bounded decision history are saved per OneAccount
with Windows CurrentUser DPAPI. The envelope is bound to the account. Broker keys
are protected separately and bound to both account and demo/live environment.
Keys are never returned to the browser or included in model input. API errors do
not echo provider responses or secrets. Same-Windows-account compromise remains
outside this guarantee.

Paper history retains the latest fifty activity entries, including bounded model
decisions and reasons, unlike Money
manager's ephemeral coaching answers. Clear/reset the experiment explicitly when
you want to replace it. No real trades or funding are recorded by that reset.
If a protected paper record fails verification, it is preserved and setup cannot
overwrite it. There is no separate deletion endpoint for a damaged record;
restore a verified Workspace recovery copy or use a separate fresh installation.

Finance's credential-free snapshots and Actual exports do not include paper state
or broker keys. Normal protected Workspace recovery can include the private
database. Protected paper records cannot be opened merely by re-entering an AI
key under a different Windows account. Recover under the original Windows account
or use a separate fresh Finance installation. Schema 6 requires recovery of a compatible pre-update backup when
rolling back to an older Finance executable.
