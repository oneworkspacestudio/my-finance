# Money manager

Open **My Finance → Money manager** to set monthly category allowances and keep
savings and investment reserves out of your spending plan. Existing `#coach`
links still work. Actual remains your ledger.

## Using the manager

1. Choose a budget, open/unlock it in Actual and use native **Sync**. Select
   **Use this budget** to bring back complete-month history and this month's
   spending through today. Only totals return. A new budget with no complete-month
   transactions needs manual estimates until it has historical income to plan from.
2. Review totals, essential costs, emergency savings, debt, horizon and goals.
   Incoming includes refunds and other credits, so check irregular amounts.
3. Review category rules. New categories default to essential to protect bills.
   Mark adjustable categories flexible. A blank target uses historical spending;
   zero means a deliberate zero target. Set desired monthly savings, investments
   and a cash buffer. Savings here are additional to your named goals.
4. The manager automatically recalculates on summary and rule changes. It shows
   allowances, spent/remaining money, overspending, reserves and funding gaps.
   Save rules explicitly to remember them for this budget and OneAccount user.
   Next visit, select the budget again to refresh figures and load saved rules.
   Remove saved rules or download a private plan whenever you want.

This is a monthly plan, **not a bank balance or cash available today**. Planning
income uses the lowest incoming month in the selected period, and might not have
arrived yet. Spending is a snapshot as of its displayed date, not a live bank
feed. Refresh after native sync/import and when the month changes. Check upcoming
bills and money actually held before spending or setting up transfers.

## Allocation rules

Integer-pence arithmetic protects essential requirements and already-incurred
spending first. Essentials above configured category costs create an additional
reserve. Funding gaps remain visible. Priority debt/emergency savings precede
general savings and ordered goals, eligible investment reservations and the cash
buffer. Flexible categories share remaining funds up to their targets with
deterministic penny rounding. Total funded allocations cannot exceed planning
income. Already-spent money cannot be reused for saving. Investment reservations
respect the requested cash cushion. A category can remain over its original
target even when the plan must fund that spending.

The emergency estimate starts at three months of essentials. Debt, emergency
funding, recent history, horizon and risk answers gate investment reservations.
This checklist does not establish product suitability. Reserved savings and
investments are unavailable for spending **in this plan**; the app does not move
cash, trade or write Actual budgets. Apply the plan in Actual to money you have.
Check existing savings and pension payments to avoid counting them twice.

Transfers and off-budget/tracking accounts are excluded. Actual has no stable
opening-balance flag: renamed/translated opening-balance labels can be included,
so check unusual totals. Category names are combined case-insensitively. Review
rules after renaming categories; absent categories' saved rules are ignored.

## AI proposals

Choose OpenAI or DeepSeek and save your own key. Review the exact payload and
consent for each request. AI can propose category limits/classifications and
savings/investment reserves, explain trade-offs and discuss UK options. Review
and explicitly apply a proposal; the deterministic allocator then recalculates
affordability. Save the resulting rules if you want to retain them. Models have
no tools or ledger-write access. Provider requests are manual and charge your
API account; calculation and rule saving do not contact providers.

## UK options

Guidance checked **3 November 2026**; this is general guidance rather than a
regulated personal investment recommendation.

- Emergency money and near-term goals: consider accessible cash savings.
  [MoneyHelper: saving or investing](https://www.moneyhelper.org.uk/en/savings/how-to-save/should-i-save-or-invest).
- Retirement: check workplace pension employer matching and access restrictions.
  Payroll contributions may already be deducted from the net pay in your budget.
  [MoneyHelper: workplace pensions](https://www.moneyhelper.org.uk/en/pensions-and-retirement/pensions-basics/workplace-pensions).
- Long-term surplus: compare a stocks-and-shares ISA and diversified fund types,
  such as global index/tracker funds or multi-asset funds. A UK wrapper does not
  mean UK-only shares. Check eligibility/current allowances on
  [GOV.UK](https://www.gov.uk/individual-savings-accounts). Diversification cannot
  prevent losses: [FCA guidance](https://www.fca.org.uk/investsmart/diversification).
- Compare fund, platform, dealing and currency costs, access and loss capacity.
  Check [FCA permissions](https://www.fca.org.uk/consumers/fca-firm-checker) and
  consider regulated advice when choosing products. The app does not promise
  returns, verify product suitability or recommend individual shares.

Current rates, tax allowances and security prices are not hard-coded. Follow
official links for rules that can change.

## Privacy and recovery

Only monthly/current/category totals and exclusion counts leave the native
browser bridge; payees, notes, account identifiers and transactions do not.
The short-lived account-bound sessionStorage handoff is consumed and deleted.
A digest of the budget ID scopes saved rules and is not sent to the AI provider.

Saved profiles/rules contain financial context and possibly private goal labels.
They are explicitly saved with Windows CurrentUser DPAPI, per user and budget,
in Finance's private database. API keys are separately protected and never
returned to the browser. Shared budget members do not share manager rules.
Finance's credential-free snapshots and native exports omit rules/keys; normal
Workspace recovery can include their protected database. Another Windows account
must enter them again. Working summaries and model answers are not persisted.

Each consented AI request sends the reviewed summary/current snapshot, profile,
calculated plan and question. Review labels/questions too. OpenAI sets
`store:false`; provider retention terms still apply. Fixed HTTPS endpoints,
bounded data, timeouts, throttling and no automatic paid retry limit exposure.
AI failure leaves arithmetic available. Output is untrusted plain text; bounded
proposals must use known category names and require explicit application.

Adapters use fictional data and synthetic responses in acceptance tests. Live
model availability, quota and billing depend on the user's key.

API references: [OpenAI](https://developers.openai.com/api/docs/guides/text),
[GPT-4.1 mini](https://developers.openai.com/api/docs/models/gpt-4.1-mini),
[DeepSeek](https://api-docs.deepseek.com/api/create-chat-completion/),
[DeepSeek JSON](https://api-docs.deepseek.com/guides/json_mode/).
