// Read-only bridge for pinned Actual 26.10.0. Raw ledger rows stay in this tab.
// Nothing here calls an AI provider or exposes a general-purpose worker API.
const REQUEST_KEY = 'myfinance.coach.request';
const SUMMARY_KEY = 'myfinance.coach.summary';
const MAX_AGE = 15 * 60 * 1000;
const READ_METHODS = new Set(['load-prefs', 'preferences/get', 'api/accounts-get',
  'api/categories-get', 'api/payees-get', 'api/transactions-get']);

export function periodMonths(from, to, now = new Date()) {
  if (!/^\d{4}-\d{2}-01$/.test(from || '') || !/^\d{4}-\d{2}-\d{2}$/.test(to || '')) throw Error('Choose complete calendar months.');
  const start = new Date(from + 'T00:00:00Z'), end = new Date(to + 'T00:00:00Z');
  if (!Number.isFinite(+start) || !Number.isFinite(+end) || start.toISOString().slice(0, 10) !== from || end.toISOString().slice(0, 10) !== to || start > end) throw Error('The analysis period is invalid.');
  const after = new Date(+end + 86400000);
  if (after.getUTCDate() !== 1 || +after > Date.UTC(now.getFullYear(), now.getMonth(), 1)) throw Error('Choose completed months, excluding the current month.');
  const result = [];
  for (let d = new Date(start); d <= end; d.setUTCMonth(d.getUTCMonth() + 1)) {
    result.push(d.toISOString().slice(0, 7));
    if (result.length > 12) throw Error('Choose at most 12 months.');
  }
  return result;
}

function addMinor(a, b) {
  const result = a + b;
  if (!Number.isSafeInteger(result) || Math.abs(result) > 100000000000) throw Error('A budget amount is outside the supported range.');
  return result;
}

export function aggregateBudget({ accounts, categories, payees, transactions, currency }, from, to, now = new Date()) {
  if (currency !== 'GBP') throw Error('Money coach currently supports GBP budgets. Set and verify the budget currency before continuing.');
  for (const items of [accounts, categories, payees, transactions]) if (!Array.isArray(items)) throw Error('The budget could not be read completely.');
  if (transactions.length > 100000) throw Error('Choose a shorter period with fewer than 100,000 transaction lines.');
  const months = periodMonths(from, to, now).map(month => ({ month, incomeMinor: 0, outgoingMinor: 0 }));
  const monthMap = new Map(months.map(m => [m.month, m]));
  const accountMap = new Map(accounts.map(a => [a.id, a]));
  const categoryMap = new Map(categories.map(c => [c.id, c]));
  const payeeMap = new Map(payees.map(p => [p.id, p]));
  const totals = new Map();
  const seen = new Set();
  let transactionCount = 0, excludedTransfers = 0;
  const rows = [];
  for (const parent of transactions) {
    if (Array.isArray(parent.subtransactions) && parent.subtransactions.length) {
      const total = parent.subtransactions.reduce((n, t) => addMinor(n, t.amount), 0);
      if (total !== parent.amount) throw Error('A split transaction is incomplete. Review it in your budget first.');
      for (const child of parent.subtransactions) rows.push({ ...child, account: child.account || parent.account, date: child.date || parent.date });
    } else {
      if (parent.is_parent) throw Error('A split transaction is incomplete. Review it in your budget first.');
      rows.push(parent);
    }
  }
  if (rows.length > 100000) throw Error('Choose a shorter period with fewer than 100,000 transaction lines.');
  for (const row of rows) {
    if (!row || typeof row.id !== 'string' || seen.has(row.id)) throw Error('The budget returned duplicate or invalid transaction lines.');
    seen.add(row.id);
    if (row.tombstone || row.date < from || row.date > to) continue;
    if (!/^\d{4}-\d{2}-\d{2}$/.test(row.date || '')) throw Error('A transaction has an invalid date.');
    const transactionDate = new Date(row.date + 'T00:00:00Z');
    if (!Number.isFinite(+transactionDate) || transactionDate.toISOString().slice(0, 10) !== row.date) throw Error('A transaction has an invalid date.');
    const month = monthMap.get(String(row.date).slice(0, 7));
    if (!month) throw Error('A transaction has an invalid date.');
    const account = accountMap.get(row.account);
    if (!account) throw Error('A transaction account could not be verified.');
    if (account.offbudget) continue;
    const payee = payeeMap.get(row.payee), category = categoryMap.get(row.category);
    if (row.transfer_id || payee?.transfer_acct) { excludedTransfers++; continue; }
    if (payee?.name?.trim().toLowerCase() === 'starting balance' || category?.is_income && category.name?.trim().toLowerCase() === 'starting balances') continue;
    if (!Number.isSafeInteger(row.amount)) throw Error('A transaction has an invalid amount.');
    if (row.amount === 0) continue;
    // Incoming credits include refunds; no inference that every credit is earnings.
    const name = [...String(category?.name || 'Uncategorised').replace(/[\u0000-\u001f\u007f]/g, ' ').trim()].slice(0, 80).join('') || 'Uncategorised';
    const key = name.toLowerCase();
    if (!totals.has(key)) totals.set(key, { name, incomeMinor: 0, outgoingMinor: 0 });
    if (totals.size > 200) throw Error('This period has more than 200 active categories. Choose a shorter period.');
    const bucket = totals.get(key), field = row.amount > 0 ? 'incomeMinor' : 'outgoingMinor';
    month[field] = addMinor(month[field], Math.abs(row.amount));
    bucket[field] = addMinor(bucket[field], Math.abs(row.amount));
    transactionCount++;
  }
  return { source: 'actual', currency, months, categories: [...totals.values()].sort((a, b) => b.outgoingMinor - a.outgoingMinor || a.name.localeCompare(b.name)), transactionCount, excludedTransfers };
}

const workerReaders = new WeakMap();
export function workerReader(worker) {
  if (workerReaders.has(worker)) return workerReaders.get(worker);
  // Actual's desktop SharedWorker adapter implements addEventListener but not
  // removeEventListener. Use one bounded dispatcher per worker, also on mobile.
  const pending = new Map();
  worker.addEventListener('message', event => {
    const message = event.data, handler = pending.get(message?.id);
    if (handler && ['reply', 'error'].includes(message.type)) handler(message);
  });
  const read = (name, args = {}) => new Promise((resolve, reject) => {
    if (!READ_METHODS.has(name)) { reject(Error('Unsupported budget operation.')); return; }
    if (pending.size >= 16) { reject(Error('The budget is busy. Wait a moment and try again.')); return; }
    const id = 'finance-coach-' + crypto.randomUUID();
    const finish = () => { clearTimeout(timer); pending.delete(id); };
    const receive = message => {
      finish();
      if (message.type === 'error' || message.error || message.result?.error) reject(Error('Open and unlock a budget, then try again.'));
      else resolve(message.result?.data);
    };
    const timer = setTimeout(() => { finish(); reject(Error('The budget took too long to respond. Reload it and try again.')); }, 20000);
    pending.set(id, receive);
    try { worker.postMessage({ id, name, args, catchErrors: true }); }
    catch { finish(); reject(Error('The budget connection closed. Reload it and try again.')); }
  });
  workerReaders.set(worker, read);
  return read;
}

export async function readNativeSummary(read, authorise, request, now = new Date()) {
  periodMonths(request.from, request.to, now);
  const before = await read('load-prefs');
  if (!before?.id || !before.cloudFileId) throw Error('Open your budget and sync it to this Finance server before using Money coach.');
  if (request.expectedBudget && before.id !== request.expectedBudget) throw Error('The open budget changed. Review its name and try again.');
  await authorise(before.cloudFileId);
  const prefs = await read('preferences/get');
  const [accounts, categories, payees, transactions] = await Promise.all([
    read('api/accounts-get'), read('api/categories-get'), read('api/payees-get'),
    read('api/transactions-get', { startDate: request.from, endDate: request.to })
  ]);
  const after = await read('load-prefs');
  if (before.id !== after?.id || before.cloudFileId !== after?.cloudFileId) throw Error('The open budget changed. Select it again before continuing.');
  await authorise(after.cloudFileId);
  return aggregateBudget({ accounts, categories, payees, transactions, currency: prefs?.defaultCurrencyCode }, request.from, request.to, now);
}

export function install({ base, expected }) {
  let request;
  const nonce = new URL(location.href).searchParams.get('finance-coach');
  if (!nonce || !/^[a-f0-9-]{36}$/.test(nonce)) return;
  try { request = JSON.parse(sessionStorage.getItem(REQUEST_KEY)); } catch { return; }
  if (!request || request.nonce !== nonce || request.account !== expected || !Number.isFinite(request.created) || Date.now() - request.created < 0 || Date.now() - request.created > MAX_AGE) {
    sessionStorage.removeItem(REQUEST_KEY); sessionStorage.removeItem(SUMMARY_KEY); return;
  }
  try { periodMonths(request.from, request.to); } catch { sessionStorage.removeItem(REQUEST_KEY); return; }
  const show = () => {
    const tray = document.createElement('section');
    tray.setAttribute('aria-label', 'Money coach budget selection');
    tray.style.cssText = 'position:fixed;z-index:100000;bottom:12px;left:12px;right:12px;max-width:560px;margin:auto;padding:14px;border:1px solid #c5d7e8;border-radius:14px;background:#fff;color:#172b3a;box-shadow:0 4px 24px #172b3a33;font:14px/1.45 system-ui;box-sizing:border-box';
    const title = document.createElement('strong'); title.textContent = 'Choose a budget for Money coach';
    const status = document.createElement('p'); status.style.margin = '6px 0 10px'; status.setAttribute('role', 'status');
    status.textContent = 'Open and unlock the budget you want, then continue. Only monthly and category totals will be carried back. Nothing is sent to an AI yet.';
    const controls = document.createElement('div'); controls.style.cssText = 'display:flex;flex-wrap:wrap;gap:8px';
    const use = document.createElement('button'); use.type = 'button'; use.textContent = 'Use this budget';
    const cancel = document.createElement('button'); cancel.type = 'button'; cancel.textContent = 'Cancel';
    const minimise = document.createElement('button'); minimise.type = 'button'; minimise.textContent = 'Minimise';
    const restore = document.createElement('button'); restore.type = 'button'; restore.textContent = 'Money coach'; restore.hidden = true;
    for (const button of [use, cancel, minimise, restore]) button.style.cssText = 'min-height:44px;padding:9px 14px;border:1px solid #b9cbdc;border-radius:8px;font:600 14px/1.3 system-ui;cursor:pointer';
    restore.style.cssText += ';position:fixed;z-index:100000;right:12px;bottom:12px;background:#fff;color:#172b3a;box-shadow:0 2px 12px #172b3a33';
    minimise.onclick = () => { tray.hidden = true; restore.hidden = false; restore.focus(); };
    restore.onclick = () => { tray.hidden = false; restore.hidden = true; minimise.focus(); };
    use.style.background = '#2457bf'; use.style.color = '#fff'; cancel.style.background = '#fff'; cancel.style.color = '#172b3a';
    let disposed = false;
    cancel.onclick = () => { disposed = true; sessionStorage.removeItem(REQUEST_KEY); sessionStorage.removeItem(SUMMARY_KEY); location.assign(base + '/#coach'); };
    const authorise = async fileId => {
      const options = { cache: 'no-store', headers: { 'X-Workspace-Expected-Account': expected } };
      const session = await fetch(base + '/api/me', options);
      if (!session.ok || (await session.json()).user?.id !== expected) throw Error('Your sign-in changed or expired. Return to Money coach and sign in again.');
      const filesResponse = await fetch(base + '/ledger/sync/list-user-files', options);
      if (!filesResponse.ok) throw Error('Your budget access could not be verified. Connect to your Finance server and try again.');
      const files = await filesResponse.json();
      if (files.status !== 'ok' || !Array.isArray(files.data) || !files.data.some(f => f.fileId === fileId && !f.deleted)) throw Error('This account no longer has access to that budget. Choose an accessible synced budget.');
    };
    let displayedBudget = '', busy = false, polling = false;
    const refreshName = async () => {
      if (busy || polling || !globalThis.Actual?.getServerSocket) return;
      polling = true;
      try {
        const read = workerReader(await globalThis.Actual.getServerSocket()), prefs = await read('load-prefs');
        if (busy) return;
        displayedBudget = prefs?.id || '';
        title.textContent = displayedBudget ? 'Selected budget: ' + String(prefs.budgetName || 'Untitled budget').slice(0, 120) : 'Choose a budget for Money coach';
      } catch { displayedBudget = ''; } finally { polling = false; }
    };
    const nameTimer = setInterval(refreshName, 2000);
    addEventListener('pagehide', () => { disposed = true; clearInterval(nameTimer); }, { once: true });
    void refreshName();
    use.onclick = async () => {
      if (use.disabled) return;
      if (!displayedBudget) { await refreshName(); status.textContent = 'Check the selected budget name above, then press Use this budget. If no budget is shown, open and unlock one first.'; return; }
      const selectedBudget = displayedBudget;
      busy = true; use.disabled = true; status.textContent = 'Reading the selected budget on this device…';
      try {
        if (Date.now() - request.created > MAX_AGE) throw Error('This selection expired. Return to Money coach and choose a budget again.');
        if (!globalThis.Actual?.getServerSocket) throw Error('Wait for your budget to finish opening, then try again.');
        const worker = await globalThis.Actual.getServerSocket();
        const summary = await readNativeSummary(workerReader(worker), authorise, { ...request, expectedBudget: selectedBudget });
        if (disposed) return;
        sessionStorage.setItem(SUMMARY_KEY, JSON.stringify({ nonce, account: expected, created: Date.now(), summary }));
        sessionStorage.removeItem(REQUEST_KEY);
        location.assign(base + '/#coach');
      } catch (error) { status.textContent = error.message || 'Could not read this budget. Try again.'; busy = false; use.disabled = false; }
    };
    controls.append(use, minimise, cancel); tray.append(title, status, controls); document.body.append(tray, restore);
  };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', show, { once: true }); else show();
}
