// Money coach keeps its working data in memory. A short-lived, account-bound
// sessionStorage envelope is used only to return a summary from the native ledger.
(() => {
  const q = selector => document.querySelector(selector);
  const money = value => new Intl.NumberFormat('en-GB', { style: 'currency', currency: 'GBP' }).format(value / 100);
  const monthName = value => new Date(value + '-01T12:00:00').toLocaleDateString('en-GB', { month: 'short', year: 'numeric' });
  const node = (tag, text, className) => { const element = document.createElement(tag); if (text !== undefined) element.textContent = text; if (className) element.className = className; return element; };
  const defaults = { openai: 'gpt-4.1-mini', deepseek: 'deepseek-flash' };
  const requestKey = 'myfinance.coach.request', summaryKey = 'myfinance.coach.summary';
  let summary = null, source = '', plan = null, advice = null, providers = [], revision = 0, readyAccount = '';
  let planning = false, analysing = false, saving = false, initializing = false;

  const status = text => { q('#coach-status').textContent = text; };
  const providerName = () => q('#coach-provider').value === 'deepseek' ? 'DeepSeek' : 'OpenAI';
  const selectedProvider = () => providers.find(provider => provider.id === q('#coach-provider').value);
  const minor = value => {
    if (value === '') return 0;
    if (!/^\d{1,8}(?:\.\d{1,2})?$/.test(String(value))) throw Error('Enter money as a positive amount with up to two decimal places.');
    const [whole, decimal = ''] = String(value).split('.');
    return Number(whole) * 100 + Number(decimal.padEnd(2, '0'));
  };
  function completedMonths(count) {
    const now = new Date(), months = [];
    for (let offset = count; offset >= 1; offset--) {
      const date = new Date(now.getFullYear(), now.getMonth() - offset, 1);
      months.push(date.getFullYear() + '-' + String(date.getMonth() + 1).padStart(2, '0'));
    }
    return months;
  }
  function resetConsent() { q('#coach-consent').checked = false; }
  function updateControls() {
    q('#coach-plan-button').disabled = !summary || planning || analysing;
    q('#coach-open-budget').disabled = planning || analysing;
    q('#coach-consent').disabled = !plan || !selectedProvider()?.configured || analysing;
    q('#coach-analyse').disabled = !plan || !selectedProvider()?.configured || !q('#coach-consent').checked || analysing || saving;
    q('#coach-remove-key').disabled = !selectedProvider()?.configured || saving || analysing;
    q('#coach-settings-form button[type=submit]').disabled = saving || analysing;
    for (const selector of ['#coach-provider','#coach-model','#coach-key']) q(selector).disabled = saving || analysing;
    q('#coach-plan-button').textContent = planning ? 'Calculating…' : 'Calculate my starting plan';
    q('#coach-analyse').textContent = analysing ? 'Preparing your guidance…' : 'Get AI guidance';
    q('#coach-consent-provider').textContent = providerName();
    q('#coach-provider-badge').textContent = selectedProvider()?.configured ? '· ' + providerName() + ' key saved' : '· No key saved';
    q('#coach-ai-hint').textContent = analysing ? 'This can take up to 90 seconds. Your budget stays unchanged.' : !plan ? 'Calculate your starting plan before asking for AI guidance.' : !selectedProvider()?.configured ? 'Save an API key for this provider to continue.' : 'Each request shares only the reviewed information. The agreement resets after every request.';
  }
  function profile() {
    const fields = q('#coach-profile-form').elements;
    return {
      essentialMonthlyMinor: minor(fields.essential.value), emergencySavingsMinor: minor(fields.emergency.value),
      highCostDebt: fields.debt.value, horizonYears: Number(fields.horizon.value), risk: fields.risk.value,
      goals: [...q('#coach-goals').children].map(card => ({
        name: card.querySelector('[data-goal=name]').value.trim(), targetMinor: minor(card.querySelector('[data-goal=target]').value),
        savedMinor: minor(card.querySelector('[data-goal=saved]').value), months: Number(card.querySelector('[data-goal=months]').value)
      }))
    };
  }
  function payload() { return { summary, profile: profile(), provider: q('#coach-provider').value, consent: true, question: q('#coach-question').value.replace(/[\r\n\t]/g, ' ').trim() }; }
  function updatePreview() {
    try {
      if (!summary) throw Error('Choose a budget or enter totals, then calculate your plan.');
      const request = payload(), data = { provider: request.provider, model: selectedProvider()?.model || defaults[request.provider], summary: request.summary, profile: request.profile, plan, question: request.question };
      q('#coach-payload').textContent = JSON.stringify(data, null, 2);
    } catch (error) { q('#coach-payload').textContent = error.message; }
  }
  function invalidate(clearPlan = true) {
    revision++; resetConsent(); advice = null; q('#coach-advice').hidden = true;
    if (clearPlan) { plan = null; q('#coach-plan').hidden = true; }
    updatePreview(); updateControls();
  }
  function validSummary(value) {
    const safeAmount = number => Number.isSafeInteger(number) && number >= 0 && number <= 100000000000;
    return value && ['actual','manual'].includes(value.source) && value.currency === 'GBP' && Array.isArray(value.months) && value.months.length > 0 && value.months.length <= 12 &&
      value.months.every(item => /^\d{4}-\d{2}$/.test(item.month) && safeAmount(item.incomeMinor) && safeAmount(item.outgoingMinor)) &&
      Array.isArray(value.categories) && value.categories.length <= 200 && value.categories.every(item => typeof item.name === 'string' && item.name.length <= 120 && safeAmount(item.incomeMinor) && safeAmount(item.outgoingMinor)) &&
      safeAmount(value.transactionCount) && safeAmount(value.excludedTransfers);
  }
  function row(label, income, outgoing) {
    const element = node('div', undefined, 'coach-row');
    element.append(node('strong', label), node('span', 'In ' + money(income)), node('span', 'Out ' + money(outgoing)));
    return element;
  }
  function useSummary(value, description) {
    if (!validSummary(value)) throw Error('This budget summary could not be read. Open your budget and try again.');
    summary = value; source = description; invalidate();
    q('#coach-data').hidden = false; q('#coach-source').textContent = source;
    const income = value.months.reduce((total, item) => total + item.incomeMinor, 0), outgoing = value.months.reduce((total, item) => total + item.outgoingMinor, 0);
    const metrics = q('#coach-metrics'); metrics.replaceChildren();
    for (const [label, amount] of [['Average in / month', Math.floor(income / value.months.length)], ['Average out / month', Math.ceil(outgoing / value.months.length)]]) {
      const metric = node('div'); metric.append(node('span', label), node('strong', money(amount))); metrics.append(metric);
    }
    q('#coach-months').replaceChildren(...value.months.map(item => row(monthName(item.month), item.incomeMinor, item.outgoingMinor)));
    q('#coach-categories').replaceChildren(...value.categories.map(item => row(item.name, item.incomeMinor, item.outgoingMinor)));
    status('Summary ready to review. No information has been sent to an AI provider.');
  }
  async function verifyAccount() {
    const current = await (await api('api/me')).json();
    if (!session || !current.user?.id || current.user.id !== session.user.id || current.user.id !== readyAccount) {
      clearData(); throw Error('Your signed-in account changed. Reload Finance before continuing.');
    }
    session = current;
  }
  function renderManualMonths() {
    const container = q('#coach-manual-months'); container.replaceChildren();
    for (const month of completedMonths(Number(q('#coach-period').value))) {
      const group = node('fieldset'); group.dataset.month = month; group.append(node('legend', monthName(month)));
      for (const [field, title] of [['income', 'Money in (£)'], ['outgoing', 'Money out (£)']]) {
        const label = node('label', title), input = node('input');
        input.type = 'number'; input.min = '0'; input.max = '10000000'; input.step = '0.01'; input.inputMode = 'decimal'; input.required = true; input.dataset.field = field;
        label.append(input); group.append(label);
      }
      container.append(group);
    }
  }
  function consumeHandoff() {
    let raw;
    try { raw = sessionStorage.getItem(summaryKey); sessionStorage.removeItem(summaryKey); sessionStorage.removeItem(requestKey); }
    catch { status('Browser storage is unavailable. You can enter monthly totals instead.'); return; }
    if (!raw) return;
    try {
      if (raw.length > 128 * 1024) throw Error('The returned summary is too large.');
      const envelope = JSON.parse(raw);
      if (!envelope.account || envelope.account !== session.user.id || !Number.isFinite(envelope.created) || Date.now() - envelope.created > 3 * 60 * 1000 || envelope.created > Date.now() + 10000) throw Error('This summary expired or belongs to another account. Choose the budget again.');
      if (envelope.error) throw Error(String(envelope.error).slice(0,300));
      useSummary(envelope.summary, 'From your selected Actual budget · ' + envelope.summary.transactionCount + ' transactions · ' + envelope.summary.excludedTransfers + ' transfer entries excluded');
    } catch (error) { status(error.message); }
  }
  q('#coach-open-budget').onclick = async () => {
    try {
      await verifyAccount();
      const months = completedMonths(Number(q('#coach-period').value)), nonce = crypto.randomUUID();
      const last = new Date(months.at(-1) + '-01T12:00:00'); last.setMonth(last.getMonth() + 1, 0);
      const to = months.at(-1) + '-' + String(last.getDate()).padStart(2, '0');
      sessionStorage.removeItem(summaryKey);
      sessionStorage.setItem(requestKey, JSON.stringify({ nonce, account: session.user.id, from: months[0] + '-01', to, created: Date.now(), returnHash: '#coach' }));
      location.href = endpoint('ledger/?finance-coach=' + encodeURIComponent(nonce));
    } catch (error) { status(error.message); }
  };
  function dropSummary() { summary = null; source = ''; q('#coach-data').hidden = true; invalidate(); }
  q('#coach-period').onchange = () => { dropSummary(); renderManualMonths(); status('The review period changed. Choose your budget again or enter totals for these months.'); };
  q('#coach-manual-form').addEventListener('input', () => { if (summary) { dropSummary(); status('Use your updated totals before calculating another plan.'); } });
  q('#coach-manual-form').onsubmit = event => {
    event.preventDefault();
    try {
      const months = [...q('#coach-manual-months').children].map(group => ({ month: group.dataset.month, incomeMinor: minor(group.querySelector('[data-field=income]').value), outgoingMinor: minor(group.querySelector('[data-field=outgoing]').value) }));
      useSummary({ source: 'manual', currency: 'GBP', months, categories: [{ name: 'Manual totals (not categorised)', incomeMinor: months.reduce((n,m)=>n+m.incomeMinor,0), outgoingMinor: months.reduce((n,m)=>n+m.outgoingMinor,0) }], transactionCount: 0, excludedTransfers: 0 }, 'Manual estimates · No transactions or category detail were imported');
      q('#coach-manual').open = false;
    } catch (error) { status(error.message); }
  };
  let goalId = 0;
  q('#coach-add-goal').onclick = () => {
    if (q('#coach-goals').children.length >= 3) return;
    const card = node('fieldset', undefined, 'coach-goal'); card.append(node('legend', 'Savings goal'));
    for (const [field, title, type, value] of [['name','Goal name','text',''],['target','Target (£)','number',''],['saved','Already saved (£)','number','0'],['months','Months to reach it','number','12']]) {
      const label = node('label', title), input = node('input'); input.id = 'coach-goal-' + ++goalId; input.type = type; input.dataset.goal = field; input.required = true; input.value = value;
      if (type === 'number') { input.min = field === 'months' ? '1' : '0'; input.max = field === 'months' ? '600' : '10000000'; input.step = field === 'months' ? '1' : '0.01'; input.inputMode = field === 'months' ? 'numeric' : 'decimal'; }
      else { input.maxLength = 80; input.placeholder = 'For example, holiday'; }
      label.append(input); card.append(label);
    }
    const remove = node('button', 'Remove goal', 'secondary'); remove.type = 'button'; remove.onclick = () => { card.remove(); q('#coach-add-goal').disabled = false; invalidate(); q('#coach-add-goal').focus(); };
    card.append(remove); q('#coach-goals').append(card); q('#coach-add-goal').disabled = q('#coach-goals').children.length >= 3;
    invalidate(); card.querySelector('input').focus();
  };
  q('#coach-profile-form').addEventListener('input', () => invalidate());
  q('#coach-profile-form').addEventListener('change', () => invalidate());
  function renderPlan(value) {
    plan = value; q('#coach-plan').hidden = false;
    q('#coach-plan-basis').textContent = 'Planning income: ' + money(value.planningIncomeMinor) + ' a month, using the lowest income month in your review. Spending reserve: ' + money(value.spendingReserveMinor) + ', covering the higher of your average outgoings or stated essentials. ' + (value.deficitMinor > 0 ? 'There is a shortfall of ' + money(value.deficitMinor) + ' before new savings.' : 'That leaves up to ' + money(value.surplusMinor) + ' for the suggestions below.');
    q('#coach-warnings').replaceChildren(...value.warnings.map(text => node('li', text)));
    q('#coach-warnings').hidden = value.warnings.length === 0;
    q('#coach-buckets').replaceChildren(...value.buckets.map(bucket => {
      const card = node('div', undefined, 'coach-bucket'); card.append(node('h3', bucket.name), node('strong', money(bucket.monthlyMinor) + ' / month'), node('p', bucket.purpose)); return card;
    }));
    q('#coach-emergency').textContent = 'A starting target of ' + money(value.emergencyTargetMinor) + ' covers three months of essential spending. ' + money(value.emergencyGapMinor) + ' remains to reach it. Adjust the target for your circumstances.';
    q('#coach-emergency-progress').value = value.emergencyTargetMinor ? Math.min(100,Math.max(0,(value.emergencyTargetMinor-value.emergencyGapMinor)/value.emergencyTargetMinor*100)) : 0;
    const goals = q('#coach-goal-plan'); goals.replaceChildren();
    if (value.goals.length) goals.append(node('h3', 'Your savings goals'));
    for (const goal of value.goals) {
      const card = node('div', undefined, 'coach-goal-result');
      card.append(node('h4', goal.name), node('p', money(goal.remainingMinor) + ' still to save · ' + money(goal.requiredMonthlyMinor) + ' a month needed for your target date.'), node('p', 'This plan allows ' + money(goal.allocatedMonthlyMinor) + ' a month. ' + (goal.remainingMinor === 0 ? 'Your goal is already covered.' : goal.monthsToGoal === null ? 'There is no spare contribution for this goal yet; review the amount, date or priorities.' : 'At that rate: about ' + goal.monthsToGoal + ' months, before interest or changes.')));
      goals.append(card);
    }
    q('#coach-investment').textContent = value.investmentReason;
    updatePreview(); updateControls();
  }
  q('#coach-profile-form').onsubmit = async event => {
    event.preventDefault(); if (!summary || planning || analysing) return;
    const version = revision; planning = true; updateControls(); status('Calculating your plan on your Finance server…');
    try {
      await verifyAccount(); if (version !== revision) return;
      const response = await (await api('api/finance/ai/plan', { summary, profile: profile() })).json();
      if (version !== revision) return;
      renderPlan(response); status('Your starting plan is ready. No AI provider was contacted.'); q('#coach-plan').scrollIntoView({ behavior: 'smooth', block: 'start' });
    } catch (error) { if (version === revision) status(error.message); }
    finally { planning = false; updateControls(); }
  };
  async function loadSettings() {
    const account = readyAccount, response = await (await api('api/finance/ai/settings')).json();
    if (!session || session.user.id !== account || readyAccount !== account) return false;
    providers = response.providers;
    if (!Array.isArray(providers)) throw Error('AI provider settings could not be read.');
    q('#coach-model').value = selectedProvider()?.model || defaults[q('#coach-provider').value];
    updateControls();
    return true;
  }
  q('#coach-provider').onchange = () => {
    q('#coach-key').value = ''; q('#coach-model').value = selectedProvider()?.model || defaults[q('#coach-provider').value]; invalidate(false);
  };
  q('#coach-model').oninput = () => { resetConsent(); updateControls(); };
  q('#coach-settings-form').onsubmit = async event => {
    event.preventDefault(); if (saving || analysing) return;
    const version = revision, name = providerName(), request = { provider: q('#coach-provider').value, model: q('#coach-model').value.trim(), apiKey: q('#coach-key').value.trim(), remove: false };
    saving = true; updateControls();
    try {
      await verifyAccount(); if (version !== revision) return;
      await api('api/finance/ai/settings', request);
      q('#coach-key').value = ''; if (version !== revision || !await loadSettings()) return;
      invalidate(false); status('Provider settings saved for ' + name + ' and your OneAccount. No paid request was made.');
      q('#coach-provider-details').open = false;
    } catch (error) { q('#coach-key').value = ''; if (version === revision) status(error.message); }
    finally { request.apiKey = ''; saving = false; updateControls(); }
  };
  q('#coach-remove-key').onclick = async () => {
    if (!selectedProvider()?.configured || saving || analysing) return;
    const version = revision, name = providerName(), request = { provider: q('#coach-provider').value, remove: true };
    saving = true; updateControls();
    try {
      await verifyAccount(); if (version !== revision) return;
      await api('api/finance/ai/settings', request);
      q('#coach-key').value = ''; if (version !== revision || !await loadSettings()) return;
      invalidate(false); status('The saved key for ' + name + ' was removed.');
    } catch (error) { if (version === revision) status(error.message); }
    finally { saving = false; updateControls(); }
  };
  q('#coach-question').oninput = () => invalidate(false);
  for (const button of document.querySelectorAll('[data-coach-question]')) button.onclick = () => {
    q('#coach-question').value = button.dataset.coachQuestion; invalidate(false); q('#coach-question').focus();
  };
  q('#coach-consent').onchange = updateControls;
  function renderAdvice(result) {
    advice = result; renderPlan(result.plan); q('#coach-advice').hidden = false;
    q('#coach-answer-meta').textContent = (result.provider === 'deepseek' ? 'DeepSeek' : 'OpenAI') + ' · ' + result.model + ' · Based only on your reviewed summary';
    q('#coach-answer-summary').textContent = result.advice.summary;
    const output = q('#coach-answer-sections'); output.replaceChildren();
    for (const [title, text] of [['What stands out',result.advice.observations],['Practical next steps',result.advice.actions],['Buckets to consider',result.advice.buckets],['Saving towards your goals',result.advice.savings],['Investment considerations',result.advice.investing],['Questions to revisit',result.advice.questions]]) {
      if (!text || Array.isArray(text) && !text.length) continue;
      output.append(node('h3', title));
      if (Array.isArray(text)) { const list = node('ul'); list.append(...text.map(item=>node('li',item))); output.append(list); }
      else output.append(node('p', text));
    }
    q('#coach-advice').focus({ preventScroll: true }); q('#coach-advice').scrollIntoView({ behavior: 'smooth', block: 'start' });
  }
  q('#coach-advice-form').onsubmit = async event => {
    event.preventDefault(); if (!plan || !q('#coach-consent').checked || analysing || !selectedProvider()?.configured) return;
    const version = revision, request = payload(); analysing = true; advice = null; q('#coach-advice').hidden = true; resetConsent(); updateControls();
    status('Sending the reviewed summary to ' + providerName() + '. Your budget will not be changed.');
    try {
      await verifyAccount(); if (version !== revision) return;
      const response = await (await api('api/finance/ai/analyse', request)).json();
      if (version !== revision) return;
      renderAdvice(response); status('Your guidance is ready. Check the assumptions before acting.');
    } catch (error) { if (version === revision) status(error.message); }
    finally { analysing = false; resetConsent(); updateControls(); }
  };
  q('#coach-download').onclick = () => {
    if (!advice) return;
    const data = { created: new Date().toISOString(), source, summary, profile: profile(), ...advice };
    const url = URL.createObjectURL(new Blob([JSON.stringify(data,null,2)],{type:'application/json'})), link = node('a');
    link.href = url; link.download = 'MyFinance-money-plan.json'; link.click(); setTimeout(()=>URL.revokeObjectURL(url),10000);
    status('Plan downloaded to your device. It contains your financial summary; keep it private.');
  };
  function clearData() {
    summary = null; source = ''; plan = null; advice = null; revision++;
    q('#coach-data').hidden = true; q('#coach-plan').hidden = true; q('#coach-advice').hidden = true;
    q('#coach-profile-form').reset(); q('#coach-advice-form').reset(); q('#coach-key').value = ''; q('#coach-goals').replaceChildren(); q('#coach-add-goal').disabled = false;
    for (const selector of ['#coach-months','#coach-categories','#coach-metrics','#coach-buckets','#coach-goal-plan','#coach-answer-sections']) q(selector).replaceChildren();
    for (const selector of ['#coach-answer-summary','#coach-answer-meta','#coach-source','#coach-plan-basis','#coach-emergency','#coach-investment','#coach-warnings']) q(selector).textContent = '';
    q('#coach-emergency-progress').value = 0; renderManualMonths();
    try { sessionStorage.removeItem(summaryKey); sessionStorage.removeItem(requestKey); } catch {}
    resetConsent(); updatePreview(); updateControls();
  }
  q('#coach-clear').onclick = () => { clearData(); status('Your working summary and plan were cleared. The budget in Actual is unchanged.'); };
  window.addEventListener('finance-clear', () => { clearData(); providers = []; readyAccount = ''; q('#coach-settings-form').reset(); q('#coach-provider-details').open = false; updateControls(); status('Sign in again to use Money coach.'); });
  async function initialize() {
    if (!session?.user?.id) { clearData(); return; }
    if (initializing || readyAccount === session.user.id) return;
    initializing = true;
    readyAccount = session.user.id;
    try { await verifyAccount(); consumeHandoff(); await loadSettings(); } catch (error) { status(error.message); }
    finally { initializing = false; }
  }
  window.addEventListener('finance-ready', initialize);
  window.addEventListener('focus', async () => {
    if (!summary || !session) return;
    try { await verifyAccount(); } catch (error) { status(error.message); }
  });
  window.addEventListener('pageshow', event => { if (event.persisted) location.reload(); });
  renderManualMonths(); updateControls(); if (session?.user?.id) initialize();
})();
