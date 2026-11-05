// Real pinned Actual browser, dedicated fictional NTFS fixture only.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import { createRequire } from 'node:module';
import { createHash } from 'node:crypto';
import { syntheticFinanceLogin } from './synthetic-finance-login.mjs';
const require = createRequire(process.env.FINANCE_MODULE_ROOT + '/package.json');
const { chromium } = require('playwright');
if (process.env.FINANCE_SYNTHETIC_ONLY !== '1' || !process.env.FINANCE_TEST_URL) throw Error('Set FINANCE_SYNTHETIC_ONLY=1 and an explicit synthetic FINANCE_TEST_URL');
const base = process.env.FINANCE_TEST_URL;
if (!['localhost', '127.0.0.1'].includes(new URL(base).hostname)) throw Error('Synthetic loopback fixture required');
const out = process.env.FINANCE_EVIDENCE || 'C:/Projects/.cache/finance-ai-20261103/evidence';
await fs.mkdir(out, { recursive: true });
const profile = process.env.FINANCE_BROWSER_PROFILE || out + '/coach-synthetic-browser';
const context = await chromium.launchPersistentContext(profile, {
  executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true,
  viewport: { width: 390, height: 844 }
});
const page = await context.newPage();
let summary, providerRequests = 0;
page.on('request', request => { if (/api\.(openai|deepseek)\.com/.test(request.url())) providerRequests++; });
await page.exposeFunction('captureCoachSummary', value => { summary = JSON.parse(value); });
await page.addInitScript(() => {
  const original = Storage.prototype.setItem;
  Storage.prototype.setItem = function (key, value) {
    if (key === 'myfinance.coach.summary') void window.captureCoachSummary(value);
    return original.call(this, key, value);
  };
});
async function attachRPC() {
  await page.waitForFunction(() => !!globalThis.Actual?.getServerSocket, {}, { timeout: 90000 });
  await page.evaluate(async () => {
    const worker = await globalThis.Actual.getServerSocket();
    const pending = new Map();
    worker.addEventListener('message', e => pending.get(e.data?.id)?.(e.data));
    // Test-only seeding RPC. It is never included in production application assets.
    globalThis.fixtureRPC = (name, args = {}) => new Promise((resolve, reject) => {
      const id = 'fictional-fixture-' + crypto.randomUUID();
      const receive = m => {
        if (m?.id !== id) return;
        clearTimeout(timer); pending.delete(id);
        if (m.type === 'error' || m.error || m.result?.error) reject(Error(JSON.stringify(m.error || m.result?.error)));
        else resolve(m.result?.data);
      };
      const timer = setTimeout(() => { pending.delete(id); reject(Error('Fixture RPC timeout: ' + name)); }, 30000);
      pending.set(id, receive); worker.postMessage({ id, name, args, catchErrors: true });
    });
  });
}
try {
  await syntheticFinanceLogin(context, page, base);
  for (let i = 0; i < 90; i++) {
    const status = await context.request.get(base + 'api/finance/status').then(r => r.json());
    if (status.engineReady) break;
    if (i === 89) throw Error('Engine readiness timeout');
    await new Promise(resolve => setTimeout(resolve, 1000));
  }
  await page.goto(base + 'ledger/');
  const start = page.getByRole('button', { name: 'Start budgeting', exact: true });
  await page.waitForFunction(() => !!globalThis.Actual?.getServerSocket, {}, { timeout: 90000 });
  await page.waitForTimeout(2500);
  if (await start.isVisible()) await start.click();
  else if ((await page.locator('body').innerText()).startsWith('Files\n')) await page.getByText('My Finances', { exact: true }).click();
  await attachRPC();
  let ready = false;
  for (let i = 0; i < 60; i++) {
    ready = await page.evaluate(async () => {
      try { const prefs = await globalThis.fixtureRPC('load-prefs'); return !!prefs?.id && Array.isArray(await globalThis.fixtureRPC('api/accounts-get')); }
      catch { return false; }
    });
    if (ready) break;
    await new Promise(resolve => setTimeout(resolve, 1000));
  }
  assert.ok(ready, 'native budget finished opening before seeding');
  const seeded = await page.evaluate(async () => {
    const rpc = globalThis.fixtureRPC;
    // Explicit fixture currency also handles recovery of a creation interrupted
    // before the launcher's default-currency setup finished.
    await rpc('preferences/save', { id: 'defaultCurrencyCode', value: 'GBP' });
    await rpc('preferences/save', { id: 'flags.currency', value: 'true' });
    if (!(await rpc('load-prefs')).cloudFileId) {
      const uploaded = await rpc('upload-budget');
      if (uploaded?.error) throw Error('Could not upload fictional fixture budget');
    }
    const categories = await rpc('api/categories-get');
    const food = categories.find(c => c.name === 'Food') || categories.find(c => !c.is_income);
    const general = categories.find(c => c.name === 'General');
    const income = categories.find(c => c.is_income && c.name !== 'Starting Balances');
    if (!food || !income || !general) throw Error('Fictional native categories unavailable');
    const now = new Date(), dates = [];
    for (let i = 3; i > 0; i--) dates.push(new Date(Date.UTC(now.getFullYear(), now.getMonth() - i, 2)).toISOString().slice(0, 10));
    const from = dates[0].slice(0, 7) + '-01';
    const to = new Date(Date.UTC(now.getFullYear(), now.getMonth(), 0)).toISOString().slice(0, 10);
    const asOf = new Date(Date.UTC(now.getFullYear(), now.getMonth(), now.getDate())).toISOString().slice(0, 10);
    const existing = (await rpc('api/accounts-get')).find(a => a.name === 'Fictional AI current');
    if (existing) return { from, to, asOf, before: await rpc('api/transactions-get', { startDate: from, endDate: asOf }), prefs: await rpc('load-prefs') };
    const account = await rpc('api/account-create', { account: { name: 'Fictional AI current', offbudget: false }, initialBalance: 123456 });
    const card = await rpc('api/account-create', { account: { name: 'Fictional AI card', offbudget: false }, initialBalance: -50000 });
    const offbudget = await rpc('api/account-create', { account: { name: 'Fictional tracking', offbudget: true }, initialBalance: 0 });
    const payees = await rpc('api/payees-get'), starting = payees.find(p => p.name === 'Starting Balance');
    const initial = await rpc('api/transactions-get', { accountId: account });
    const opening = initial.find(t => t.payee === starting?.id);
    if (!opening) throw Error('Starting balance fixture missing');
    await rpc('api/transaction-update', { id: opening.id, fields: { date: dates[0] } });
    await rpc('api/transactions-add', { accountId: account, transactions: dates.flatMap(date => [
      { date, amount: 200000, category: income.id, payee_name: 'Fictional salary' },
      { date, amount: -100000, category: food.id, payee_name: 'Fictional household' }
    ]) });
    const transfer = payees.find(p => p.transfer_acct === card);
    await rpc('api/transactions-add', { accountId: account, runTransfers: true, transactions: [{ date: dates[2], amount: -10000, payee: transfer.id }] });
    await rpc('api/transactions-add', { accountId: offbudget, transactions: [{ date: dates[1], amount: 999999 }] });
    await rpc('api/transactions-add', { accountId: card, transactions: [{ date: dates[2], amount: 100, category: food.id, payee_name: 'Fictional refund' }] });
    await rpc('api/transactions-add', { accountId: account, transactions: [
      { date: asOf, amount: 200000, category: income.id, payee_name: 'Fictional current salary' },
      { date: asOf, amount: -12345, category: food.id, payee_name: 'Fictional current groceries' },
      { date: asOf, amount: 345, category: food.id, payee_name: 'Fictional current refund' },
      { date: asOf, amount: -5000, category: general.id, payee_name: 'Fictional current-only category' }
    ] });
    await rpc('api/transactions-add', { accountId: account, runTransfers: true, transactions: [{ date: asOf, amount: -10000, payee: transfer.id }] });
    await rpc('api/transactions-add', { accountId: offbudget, transactions: [{ date: asOf, amount: 888888 }] });
    await rpc('api/sync');
    const before = await rpc('api/transactions-get', { startDate: from, endDate: asOf });
    return { from, to, asOf, before, prefs: await rpc('load-prefs') };
  });
  assert.ok(seeded.prefs.cloudFileId, 'native budget uploaded');
  const ledgerHash = createHash('sha256').update(JSON.stringify(seeded.before)).digest('hex');
  if (process.env.FINANCE_BASELINE_LEDGER) {
    const baseline = JSON.parse(await fs.readFile(process.env.FINANCE_BASELINE_LEDGER, 'utf8'));
    assert.equal(ledgerHash, baseline.transactionsSHA256, 'managed update preserved the fictional native ledger exactly');
  }
  if (process.env.FINANCE_SEED_ONLY === '1') {
    await fs.writeFile(out + '/baseline-native-ledger.json', JSON.stringify({ synthetic: true, transactionsSHA256: ledgerHash, count: seeded.before.length, from: seeded.from, asOf: seeded.asOf }, null, 2));
    console.log('PASS managed baseline: fictional native ledger seeded for supported upgrade comparison');
  } else {
  const me = await context.request.get(base + 'api/me').then(r => r.json());
  const nonce = await page.evaluate(({ account, from, to }) => {
    const nonce = crypto.randomUUID();
    sessionStorage.setItem('myfinance.coach.request', JSON.stringify({ nonce, account, from, to, created: Date.now(), returnHash: '#coach' }));
    return nonce;
  }, { account: me.user.id, from: seeded.from, to: seeded.to });
  await page.goto(base + 'ledger/?finance-coach=' + nonce);
  await page.getByText('Selected budget: My Finances', { exact: true }).waitFor({ timeout: 60000 });
  const use = page.getByRole('button', { name: 'Use this budget', exact: true }); await use.waitFor();
  for (const width of [320, 360, 390]) {
    await page.setViewportSize({ width, height: 844 });
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, 'native coach ribbon fits phone');
    await page.screenshot({ path: out + '/native-coach-selection-' + width + '.png' });
  }
  await page.getByRole('button', { name: 'Minimise', exact: true }).click();
  await use.waitFor({ state: 'hidden' });
  await page.getByRole('button', { name: 'Money manager', exact: true }).click();
  await use.waitFor();
  await use.click();
  await page.waitForURL('**/#coach', { timeout: 60000 });
  assert.ok(summary, 'browser-local aggregate handoff');
  assert.equal(summary.account, me.user.id);
  assert.equal(summary.summary.months.length, 3);
  assert.equal(summary.summary.months.reduce((s, m) => s + m.incomeMinor, 0), 600100);
  assert.equal(summary.summary.months.reduce((s, m) => s + m.outgoingMinor, 0), 300000);
  assert.equal(summary.summary.excludedTransfers, 2);
  assert.equal(summary.summary.transactionCount, 7);
  assert.deepEqual({ month: summary.summary.current.month, asOf: summary.summary.current.asOf, incomeMinor: summary.summary.current.incomeMinor, outgoingMinor: summary.summary.current.outgoingMinor }, { month: seeded.asOf.slice(0, 7), asOf: seeded.asOf, incomeMinor: 200345, outgoingMinor: 17345 });
  assert.equal(summary.summary.current.categories.find(c => c.name === 'General')?.outgoingMinor, 5000, 'current-only category included');
  assert.equal(summary.budgetRef, createHash('sha256').update(seeded.prefs.cloudFileId).digest('hex'));
  assert.equal(summary.summary.budgetRef, undefined, 'budget scope does not enter provider summary');
  assert.ok(!JSON.stringify(summary.summary).includes(seeded.prefs.cloudFileId), 'raw budget identifier stays in native tab');
  assert.ok(!JSON.stringify(summary.summary).includes('Fictional'), 'payees/accounts absent from aggregate');
  assert.equal(providerRequests, 0, 'import sends no provider request');
  const profile = {
    essentialMonthlyMinor: 100000, emergencySavingsMinor: 300000,
    highCostDebt: 'no', horizonYears: 10, risk: 'balanced', goals: [],
    manager: { enabled: true, savingsMonthlyMinor: 10000, investmentMonthlyMinor: 5000, cashBufferMinor: 1000, categories: [] }
  };
  const financeHeaders = { Origin: new URL(base).origin, 'X-CSRF-Token': me.csrf, 'X-Workspace-Expected-Account': me.user.id };
  const planResponse = await context.request.post(base + 'api/finance/ai/plan', { data: { summary: summary.summary, profile }, headers: financeHeaders });
  assert.equal(planResponse.status(), 200, 'real server accepts the imported native snapshot');
  const plan = await planResponse.json(), manager = plan.manager;
  assert.ok(manager?.currentAvailable, 'manager uses current Actual spending');
  assert.equal(manager.spentMinor, 17345);
  assert.ok(manager.categories.every(c => c.essential), 'unclassified categories are protected by default');
  const foodPlan = manager.categories.find(c => c.name === 'Food'), generalPlan = manager.categories.find(c => c.name === 'General');
  assert.equal(foodPlan.historicalMonthlyMinor, 100000);
  assert.equal(foodPlan.spentMinor, 12345);
  assert.ok(foodPlan.limitMinor >= foodPlan.spentMinor, 'protected costs cover spending already made');
  assert.equal(generalPlan.spentMinor, 5000);
  assert.equal(generalPlan.overspentMinor, 5000, 'new category spending is flagged for review');
  assert.equal(manager.savingsReservedMinor, 10000);
  const allocation = ['categoryAllocatedMinor', 'unassignedEssentialsAllocatedMinor', 'debtReservedMinor', 'emergencyReservedMinor', 'savingsReservedMinor', 'goalReservedMinor', 'investmentReservedMinor', 'cashBufferReservedMinor', 'unallocatedMinor'].reduce((n, key) => n + manager[key], 0);
  assert.equal(allocation, manager.planningIncomeMinor, 'all category allowances and reserves conserve the forecast exactly');
  const settingsURL = base + 'api/finance/ai/manager-settings';
  try {
    const saved = await context.request.post(settingsURL, { data: { scope: summary.budgetRef, profile }, headers: financeHeaders });
    assert.equal(saved.status(), 200, 'rules save in the fictional account and native budget scope');
    const loaded = await context.request.get(settingsURL + '?scope=' + summary.budgetRef, { headers: financeHeaders });
    assert.equal(loaded.status(), 200);
    const savedRules = await loaded.json(); assert.equal(savedRules.configured, true); assert.deepEqual(savedRules.profile, profile);
    const otherScope = await context.request.get(settingsURL + '?scope=manual', { headers: financeHeaders });
    assert.equal((await otherScope.json()).configured, false, 'native rules do not leak into the manual-estimates scope');
  } finally {
    const removed = await context.request.post(settingsURL, { data: { scope: summary.budgetRef, remove: true }, headers: financeHeaders });
    assert.equal(removed.status(), 200, 'fictional rules removed after roundtrip');
  }
  const removedRules = await context.request.get(settingsURL + '?scope=' + summary.budgetRef, { headers: financeHeaders });
  assert.equal((await removedRules.json()).configured, false);
  await page.goto(base + 'ledger/');
  await page.waitForFunction(() => !!globalThis.Actual?.getServerSocket, {}, { timeout: 60000 });
  await page.waitForTimeout(2500);
  const after = await page.evaluate(async ({ from, asOf, base }) => {
    const { workerReader } = await import(base + 'coach-ledger.js');
    const read = workerReader(await globalThis.Actual.getServerSocket());
    return read('api/transactions-get', { startDate: from, endDate: asOf });
  }, { ...seeded, base });
  assert.deepEqual(after, seeded.before, 'import does not mutate native ledger');
  await fs.writeFile(out + '/native-coach-report.json', JSON.stringify({ synthetic: true, upstream: '26.10.0', localOnlyAggregation: true, exactGrossIncomeMinor: 600100, exactOutgoingMinor: 300000, currentGrossIncomeMinor: 200345, currentOutgoingMinor: 17345, currentOnlyCategoryIncluded: true, hashedBudgetScopeOutsideSummary: true, managerServerPlan: true, unclassifiedCategoriesProtected: true, forecastAllocationConserved: true, rulesSaveLoadRemove: true, transferLegsExcluded: 2, openingBalanceExcluded: true, offbudgetExcluded: true, providerRequests, ledgerUnchanged: true, phoneOverflow: false }, null, 2));
  console.log('PASS native Actual manager import: exact historical/current totals, hashed budget scope, exclusions, mobile ribbon and unchanged ledger');
  }
} catch (error) {
  await page.screenshot({ path: out + '/native-coach-failure.png' }).catch(() => {});
  console.log((await page.locator('body').innerText()).slice(0, 4000));
  throw error;
} finally { await context.close(); }
