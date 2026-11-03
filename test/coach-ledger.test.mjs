import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { aggregateBudget, periodMonths, readNativeSnapshot, readNativeSummary, workerReader } from '../internal/finance/ui/coach-ledger.js';

const now = new Date('2026-11-03T12:00:00Z');
const period = { from: '2026-08-01', to: '2026-10-31' };
function fixture() {
  return {
    currency: 'GBP', accounts: [{ id: 'current' }, { id: 'card' }, { id: 'investment', offbudget: true }],
    categories: [{ id: 'salary', name: 'Income', is_income: true }, { id: 'food', name: 'Groceries' }, { id: 'bills', name: 'Bills' }],
    payees: [{ id: 'starting', name: 'Starting Balance' }, { id: 'transfer', name: 'Fictional card', transfer_acct: 'card' }],
    transactions: [
      { id: 'opening', account: 'current', date: '2026-08-01', amount: 50000, payee: 'starting', category: 'salary' },
      { id: 'salary1', account: 'current', date: '2026-08-02', amount: 200000, category: 'salary', notes: 'Private note', payee: 'private' },
      { id: 'repayment1', account: 'current', date: '2026-08-03', amount: -10000, payee: 'transfer', transfer_id: 'repayment2' },
      { id: 'repayment2', account: 'card', date: '2026-08-03', amount: 10000, transfer_id: 'repayment1' },
      { id: 'split', account: 'current', date: '2026-08-04', amount: -2000, is_parent: true, subtransactions: [
        { id: 'split-food', amount: -1500, category: 'food' }, { id: 'split-bills', amount: -500, category: 'bills' }
      ] },
      { id: 'refund', account: 'card', date: '2026-10-01', amount: 100, category: 'food' },
      { id: 'unknown', account: 'current', date: '2026-10-02', amount: -99 },
      { id: 'offbudget', account: 'investment', date: '2026-10-01', amount: 99999 },
      { id: 'future', account: 'current', date: '2026-11-02', amount: 500000, category: 'salary' }
    ]
  };
}

test('summary uses integer gross flows, zero months and split children; excludes transfers/opening/offbudget', () => {
  const result = aggregateBudget(fixture(), period.from, period.to, now);
  assert.deepEqual(result.months, [
    { month: '2026-08', incomeMinor: 200000, outgoingMinor: 2000 },
    { month: '2026-09', incomeMinor: 0, outgoingMinor: 0 },
    { month: '2026-10', incomeMinor: 100, outgoingMinor: 99 }
  ]);
  assert.equal(result.transactionCount, 5);
  assert.equal(result.excludedTransfers, 2);
  assert.equal(result.categories.reduce((s, c) => s + c.incomeMinor, 0), 200100);
  assert.equal(result.categories.reduce((s, c) => s + c.outgoingMinor, 0), 2099);
  for (const forbidden of ['Private note', 'Fictional card', 'private', 'salary1', 'starting']) assert.ok(!JSON.stringify(result).includes(forbidden));
  assert.deepEqual(result.current, { month: '2026-11', asOf: '2026-11-03', incomeMinor: 500000, outgoingMinor: 0, categories: [{ name: 'Income', incomeMinor: 500000, outgoingMinor: 0 }] });
});

test('current month is separate, includes new categories and refunds, and excludes future/transfer/opening/tracking rows', () => {
  const f = fixture();
  f.categories.push({ id: 'health', name: 'Health' });
  f.transactions.push(
    { id: 'current-split', account: 'current', date: '2026-11-03', amount: -1234, is_parent: true, subtransactions: [
      { id: 'current-food', amount: -1000, category: 'food' }, { id: 'current-health', amount: -234, category: 'health' }
    ] },
    { id: 'current-refund', account: 'current', date: '2026-11-03', amount: 50, category: 'food' },
    { id: 'current-transfer', account: 'current', date: '2026-11-03', amount: -10000, payee: 'transfer', transfer_id: 'another-leg' },
    { id: 'current-opening', account: 'current', date: '2026-11-03', amount: 50000, payee: 'starting' },
    { id: 'current-tracking', account: 'investment', date: '2026-11-03', amount: 20000 },
    { id: 'future-tomorrow', account: 'current', date: '2026-11-04', amount: -80000, category: 'bills' }
  );
  const result = aggregateBudget(f, period.from, period.to, now);
  assert.equal(result.transactionCount, 5, 'historical count stays historical');
  assert.equal(result.excludedTransfers, 2, 'historical exclusion count stays historical');
  assert.equal(result.current.incomeMinor, 500050);
  assert.equal(result.current.outgoingMinor, 1234);
  assert.deepEqual(result.current.categories.find(c => c.name === 'Health'), { name: 'Health', incomeMinor: 0, outgoingMinor: 234 });
  assert.equal(result.current.categories.reduce((n, c) => n + c.outgoingMinor, 0), result.current.outgoingMinor);
  assert.equal(result.current.categories.reduce((n, c) => n + c.incomeMinor, 0), result.current.incomeMinor);
  assert.ok(!result.categories.some(c => c.name === 'Health'), 'new current category does not change historical totals');
});

test('empty current month remains explicit and history gaps are not mistaken for current spending', () => {
  const f = fixture(); f.transactions = f.transactions.filter(t => t.date < '2026-11-01');
  const result = aggregateBudget(f, '2026-08-01', '2026-09-30', now);
  assert.deepEqual(result.current, { month: '2026-11', asOf: '2026-11-03', incomeMinor: 0, outgoingMinor: 0, categories: [] });
  assert.equal(result.categories.reduce((n, c) => n + c.outgoingMinor, 0), 2000);
});

test('invalid currency, duplicate rows, incomplete split and invalid amounts fail without a partial result', () => {
  for (const mutate of [
    f => { f.currency = 'USD'; },
    f => { f.transactions.push(f.transactions[1]); },
    f => { f.transactions[4].amount = -3000; },
    f => { f.transactions[1].amount = 0.01; },
    f => { f.transactions[1].account = 'missing'; }
  ]) { const f = fixture(); mutate(f); assert.throws(() => aggregateBudget(f, period.from, period.to, now)); }
});

test('period allows only 1-12 completed calendar months and real dates', () => {
  assert.equal(periodMonths(period.from, period.to, now).length, 3);
  for (const [from, to] of [['2026-11-01', '2026-11-30'], ['2026-02-01', '2026-02-30'], ['2026-01-02', '2026-10-31'], ['2025-10-01', '2026-10-31'], ['2026-08-01', '2026-10-30']]) assert.throws(() => periodMonths(from, to, now));
});

test('normalises duplicate category labels and bounded plain names without losing totals', () => {
  const f = fixture(); f.categories.push({ id: 'bills2', name: 'bills' });
  f.transactions.push({ id: 'bill', account: 'current', date: '2026-10-03', category: 'bills2', amount: -10 });
  const result = aggregateBudget(f, period.from, period.to, now);
  assert.equal(result.categories.filter(c => c.name.toLowerCase() === 'bills').length, 1);
  assert.equal(result.categories.find(c => c.name === 'Bills').outgoingMinor, 510);
});

test('read-only bridge checks current cloud access twice and rejects a switched budget', async () => {
  const f = fixture(); let metadataReads = 0; const methods = [], access = [];
  const read = async name => { methods.push(name); return ({
    'load-prefs': { id: ++metadataReads < 100 ? 'budget' : 'different', cloudFileId: 'cloud' },
    'preferences/get': { defaultCurrencyCode: 'GBP' },
    'api/accounts-get': f.accounts, 'api/categories-get': f.categories,
    'api/payees-get': f.payees, 'api/transactions-get': f.transactions
  })[name]; };
  const result = await readNativeSummary(read, async id => { access.push(id); }, period, now);
  assert.equal(result.transactionCount, 5);
  assert.deepEqual(access, ['cloud', 'cloud']);
  assert.ok(methods.every(name => !/create|update|sync|save|delete/.test(name)));
  let count = 0;
  await assert.rejects(readNativeSummary(async name => name === 'load-prefs' ? { id: ++count === 1 ? 'a' : 'b', cloudFileId: 'cloud' } : read(name), async () => {}, period, now), /changed/);
  await assert.rejects(readNativeSummary(read, async () => { throw Error('Revoked'); }, period, now), /Revoked/);
});

test('native snapshot reads through today and scopes rules with a stable hash outside the financial summary', async () => {
  const f = fixture(), requests = [];
  let cloudFileId = 'synthetic-cloud-one';
  const read = async (name, args) => {
    requests.push({ name, args });
    return ({ 'load-prefs': { id: 'budget', cloudFileId }, 'preferences/get': { defaultCurrencyCode: 'GBP' },
      'api/accounts-get': f.accounts, 'api/categories-get': f.categories, 'api/payees-get': f.payees, 'api/transactions-get': f.transactions })[name];
  };
  const first = await readNativeSnapshot(read, async () => {}, period, now);
  assert.equal(first.budgetRef, createHash('sha256').update(cloudFileId).digest('hex'));
  assert.deepEqual(requests.find(r => r.name === 'api/transactions-get').args, { startDate: period.from, endDate: '2026-11-03' });
  assert.equal(first.summary.current.incomeMinor, 500000);
  assert.ok(!JSON.stringify(first.summary).includes('synthetic-cloud-one'));
  assert.equal(first.summary.budgetRef, undefined, 'reference is never part of provider summary');
  assert.equal((await readNativeSnapshot(read, async () => {}, period, now)).budgetRef, first.budgetRef);
  cloudFileId = 'synthetic-cloud-two';
  assert.notEqual((await readNativeSnapshot(read, async () => {}, period, now)).budgetRef, first.budgetRef);
});

test('pinned desktop worker adapter without removeEventListener supports bounded concurrent reads', async () => {
  const listeners = [], calls = [];
  const worker = {
    addEventListener: (type, callback) => { assert.equal(type, 'message'); listeners.push(callback); },
    postMessage: message => { calls.push(message); queueMicrotask(() => {
      for (const callback of listeners) callback({ data: { type: 'reply', id: message.id, result: { data: message.name, error: null } } });
    }); }
  };
  const read = workerReader(worker);
  assert.equal(workerReader(worker), read);
  assert.deepEqual(await Promise.all([read('load-prefs'), read('api/accounts-get')]), ['load-prefs', 'api/accounts-get']);
  assert.equal(listeners.length, 1);
  await assert.rejects(read('api/transaction-update'), /Unsupported/);
  assert.equal(calls.length, 2);
});
