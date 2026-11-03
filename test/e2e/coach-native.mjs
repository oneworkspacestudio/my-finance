// Real pinned Actual browser, dedicated fictional NTFS fixture only.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import { createRequire } from 'node:module';
const require = createRequire(process.env.FINANCE_MODULE_ROOT + '/package.json');
const { chromium } = require('playwright');
if (process.env.FINANCE_SYNTHETIC_ONLY !== '1' || !process.env.FINANCE_TEST_URL) throw Error('Set FINANCE_SYNTHETIC_ONLY=1 and an explicit synthetic FINANCE_TEST_URL');
const base = process.env.FINANCE_TEST_URL;
if (!['localhost', '127.0.0.1'].includes(new URL(base).hostname)) throw Error('Synthetic loopback fixture required');
const out = process.env.FINANCE_EVIDENCE || 'C:/Projects/.cache/finance-ai-20261103/evidence';
await fs.mkdir(out, { recursive: true });
const context = await chromium.launchPersistentContext(out + '/coach-synthetic-browser', {
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
  const login = await context.request.post(base + 'api/login', { data: { email: 'owner@example.test', password: 'Fictional-Test-Only-582!' }, headers: { Origin: new URL(base).origin } });
  assert.equal(login.status(), 200, 'synthetic account login');
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
  await attachRPC();
  await page.waitForFunction(async () => !!(await globalThis.fixtureRPC('load-prefs'))?.id, {}, { timeout: 60000 });
  const seeded = await page.evaluate(async () => {
    const rpc = globalThis.fixtureRPC;
    const categories = await rpc('api/categories-get');
    const food = categories.find(c => c.name === 'Food') || categories.find(c => !c.is_income);
    const income = categories.find(c => c.is_income && c.name !== 'Starting Balances');
    if (!food || !income) throw Error('Fictional native categories unavailable');
    const now = new Date(), dates = [];
    for (let i = 3; i > 0; i--) dates.push(new Date(Date.UTC(now.getFullYear(), now.getMonth() - i, 2)).toISOString().slice(0, 10));
    const from = dates[0].slice(0, 7) + '-01';
    const to = new Date(Date.UTC(now.getFullYear(), now.getMonth(), 0)).toISOString().slice(0, 10);
    const existing = (await rpc('api/accounts-get')).find(a => a.name === 'Fictional AI current');
    if (existing) return { from, to, before: await rpc('api/transactions-get', { startDate: from, endDate: to }), prefs: await rpc('load-prefs') };
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
    await rpc('api/sync');
    const before = await rpc('api/transactions-get', { startDate: from, endDate: to });
    return { from, to, before, prefs: await rpc('load-prefs') };
  });
  assert.ok(seeded.prefs.cloudFileId, 'native budget uploaded');
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
  await page.getByRole('button', { name: 'Money coach', exact: true }).click();
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
  assert.ok(!JSON.stringify(summary.summary).includes('Fictional'), 'payees/accounts absent from aggregate');
  assert.equal(providerRequests, 0, 'import sends no provider request');
  await page.goto(base + 'ledger/');
  await page.waitForFunction(() => !!globalThis.Actual?.getServerSocket, {}, { timeout: 60000 });
  await page.waitForTimeout(2500);
  const after = await page.evaluate(async ({ from, to, base }) => {
    const { workerReader } = await import(base + 'coach-ledger.js');
    const read = workerReader(await globalThis.Actual.getServerSocket());
    return read('api/transactions-get', { startDate: from, endDate: to });
  }, { ...seeded, base });
  assert.deepEqual(after, seeded.before, 'import does not mutate native ledger');
  await fs.writeFile(out + '/native-coach-report.json', JSON.stringify({ synthetic: true, upstream: '26.10.0', localOnlyAggregation: true, exactGrossIncomeMinor: 600100, exactOutgoingMinor: 300000, transferLegsExcluded: 2, openingBalanceExcluded: true, offbudgetExcluded: true, providerRequests, ledgerUnchanged: true, phoneOverflow: false }, null, 2));
  console.log('PASS native Actual read-only coach import: exact totals, transfers/opening/offbudget exclusion, mobile ribbon and unchanged ledger');
} catch (error) {
  await page.screenshot({ path: out + '/native-coach-failure.png' }).catch(() => {});
  console.log((await page.locator('body').innerText()).slice(0, 4000));
  throw error;
} finally { await context.close(); }
