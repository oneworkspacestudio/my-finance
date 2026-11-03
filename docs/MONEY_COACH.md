# Money coach

Open **My Finance → Money coach** to turn income and spending into a plan.
Actual remains your ledger. The coach never moves money, makes investments or
changes your budget automatically.

1. Choose **Choose a budget** and open the budget you want to review in Actual.
   Use the coach button there to bring back a summary of the last three complete
   months. An encrypted budget must be unlocked by you in Actual first.
2. Review the incoming/outgoing totals and categories. Incoming includes refunds
   and other credits, so it is not a salary estimate. Internal transfers and
   standard opening balances are excluded; check that all relevant accounts are present.
   You can also start with monthly figures entered yourself.
3. Enter essential monthly costs, money already reserved for emergencies, debt
   status and savings goals. Include irregular bills and commitments not captured
   in the reviewed months. Calculate a plan without contacting an AI provider.
4. For an explanation, choose OpenAI or DeepSeek and save your own API key. Each
   household member has separate settings and keys. A saved key can be removed.
5. Review the exact information shown in the send preview, tick the consent box
   and request AI guidance. Your provider charges your account for the request.
   Saving a key does not make any paid API call.

The plan uses integer pence. Planning income uses the lower of the observed
average and lowest monthly incoming amount. It reserves the greater of average outgoings and your stated essential costs
before allocating surplus. Goals show the monthly amount needed, the affordable amount
and the estimated time to reach them without assuming investment returns.
If income is short or your financial context is incomplete, the plan says so.

Buckets are proposed envelopes, not bank accounts. Review the suggested amounts
and create categories/category groups and targets in Actual, or adapt existing
ones. Only budget money you actually have; future monthly income is a planning
estimate. Transfers between your own accounts do not create income.

Native Actual identifies starting balances by their usual payee/category label.
If you renamed or translated those labels, check the reviewed totals: an opening
balance may appear as an incoming credit or outgoing amount. Tracking/off-budget
accounts are excluded. Check irregular credits before treating them as reliable
income and committing to a savings schedule.

Investment readiness is an educational checklist. It considers data coverage,
cash flow, emergency savings, debt, time horizon and risk. It does not assess
product suitability or recommend particular funds, shares, platforms, tax
wrappers or trades. Returns are not guaranteed and investments can lose value.
For personal investment decisions, consider a regulated financial adviser.

## What leaves your computer

The native bridge reads the open budget in your browser. It returns only monthly
and category totals and transaction/exclusion counts. It never sends payees,
transaction notes, bank/account identifiers or raw transactions to the coach
server. A short-lived, account-bound sessionStorage handoff is deleted on receipt.

Nothing is sent to OpenAI or DeepSeek until you consent for that request. The
reviewed summary, financial context, goal labels, question and calculated plan
then pass through the Finance server to your selected provider. Labels and your
question can contain personal information, so review them too. Do not enter bank
credentials, account numbers or information you do not want that provider to see.

Keys are stored on the Windows server under your Windows account's DPAPI
protection and your OneAccount user ID. Finance does not return them to the
browser or store conversation history. OpenAI requests disable response storage
with `store: false`; provider retention policies still apply. Finance's encrypted
budget exports and credential-free server snapshots omit AI keys. A normal
Workspace recovery copy can include the DPAPI-protected private database.

The service uses fixed provider addresses, bounded requests and responses,
timeouts, per-user throttling and no automatic paid retry. If a provider is
unavailable or returns an invalid response, your arithmetic plan still works.
Neither provider's output is a verified financial fact; review the explanation
against the figures and your current circumstances.

## Sources and verification

Implementation references checked 3 November 2026:

- [OpenAI text generation and Responses API](https://developers.openai.com/api/docs/guides/text)
- [OpenAI GPT-4.1 mini](https://developers.openai.com/api/docs/models/gpt-4.1-mini)
- [DeepSeek chat completions](https://api-docs.deepseek.com/api/create-chat-completion/)
- [DeepSeek JSON output](https://api-docs.deepseek.com/guides/json_mode/)
- [DeepSeek model names](https://api-docs.deepseek.com/quick_start/pricing/)

The educational guidance uses MoneyHelper's
[emergency savings guide](https://www.moneyhelper.org.uk/en/savings/types-of-savings/emergency-savings-how-much-is-enough)
and [investing introduction](https://www.moneyhelper.org.uk/en/savings/investing/investing-beginners-guide).
An accessible emergency buffer of three to six months of essential costs is a
general rule of thumb, not an individual recommendation. The app uses three
months as its initial calculation and highlights uncertainty.

Provider adapters are tested with synthetic responses. No paid provider account
or real financial data was used as a test fixture. Model availability, quota and
billing are determined by your provider account.
