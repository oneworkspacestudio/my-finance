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
  let managerScope = '', savedRules = false, rulesSaving = false, planTimer = null, managerSuggestion = null;

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
    q('#coach-plan-button').textContent = planning ? 'Updating…' : 'Update my plan';
    q('#manager-save-rules').disabled = !summary || !managerScope || rulesSaving || saving || analysing;
    q('#manager-remove-rules').disabled = !managerScope || !savedRules || rulesSaving || saving || analysing;
    q('#manager-download').disabled = !plan;
    q('#coach-analyse').textContent = analysing ? 'Preparing your guidance…' : 'Get AI guidance';
    q('#coach-consent-provider').textContent = providerName();
    q('#coach-provider-badge').textContent = selectedProvider()?.configured ? '· ' + providerName() + ' key saved' : '· No key saved';
    q('#coach-ai-hint').textContent = analysing ? 'This can take up to 90 seconds. Your budget stays unchanged.' : !plan ? 'Load a budget and let your plan update before asking for an AI review.' : !selectedProvider()?.configured ? 'Save an API key for this provider to continue.' : 'Each request shares only the reviewed information. The agreement resets after every request.';
  }
  function profile() {
    const fields = q('#coach-profile-form').elements;
    return {
      essentialMonthlyMinor: minor(fields.essential.value), emergencySavingsMinor: minor(fields.emergency.value),
      highCostDebt: fields.debt.value, horizonYears: Number(fields.horizon.value), risk: fields.risk.value,
      manager: {
        enabled: true, savingsMonthlyMinor: minor(fields.managerSavings.value), investmentMonthlyMinor: minor(fields.managerInvestment.value), cashBufferMinor: minor(fields.managerBuffer.value),
        categories: [...q('#manager-rule-list').querySelectorAll('[data-category-rule]')].map(card => ({ name: card.dataset.categoryRule, essential: card.querySelector('[data-rule=essential]').checked, targetMonthlyMinor: card.querySelector('[data-rule=target]').value === '' ? null : minor(card.querySelector('[data-rule=target]').value) }))
      },
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
    revision++; resetConsent(); advice = null; managerSuggestion = null; q('#coach-advice').hidden = true; q('#manager-proposal').hidden = true;
    if (clearPlan) { plan = null; q('#coach-plan').hidden = true; q('#manager-dashboard').hidden = true; q('#manager-uk-next').textContent = 'Refresh your plan to check the next step. Keep short-term needs and emergency savings accessible before considering investments.'; }
    updatePreview(); updateControls();
  }
  function schedulePlan() {
    clearTimeout(planTimer);
    if (summary) planTimer = setTimeout(() => calculatePlan(false), 400);
  }
  function profileChanged() { invalidate(); q('#manager-rules-status').textContent = managerScope ? 'Rules changed. Your plan updates automatically; save to remember these changes.' : 'Load a budget to save rules for it.'; schedulePlan(); }
  function validSummary(value) {
    const safeAmount = number => Number.isSafeInteger(number) && number >= 0 && number <= 100000000000;
    const safeCategories = categories => Array.isArray(categories) && categories.length <= 200 && categories.every(item => item && typeof item.name === 'string' && item.name.trim().length > 0 && item.name.length <= 120 && safeAmount(item.incomeMinor) && safeAmount(item.outgoingMinor));
    return value && ['actual','manual'].includes(value.source) && value.currency === 'GBP' && Array.isArray(value.months) && value.months.length > 0 && value.months.length <= 12 &&
      value.months.every(item => /^\d{4}-\d{2}$/.test(item.month) && safeAmount(item.incomeMinor) && safeAmount(item.outgoingMinor)) &&
      safeCategories(value.categories) && (!value.current || typeof value.current === 'object' && /^\d{4}-\d{2}$/.test(value.current.month) && /^\d{4}-\d{2}-\d{2}$/.test(value.current.asOf) && safeAmount(value.current.incomeMinor) && safeAmount(value.current.outgoingMinor) && safeCategories(value.current.categories)) &&
      safeAmount(value.transactionCount) && safeAmount(value.excludedTransfers);
  }
  function row(label, income, outgoing) {
    const element = node('div', undefined, 'coach-row');
    element.append(node('strong', label), node('span', 'In ' + money(income)), node('span', 'Out ' + money(outgoing)));
    return element;
  }
  function spendingCategories() {
    const names = new Set();
    for (const category of [...(summary?.categories || []), ...(summary?.current?.categories || [])]) if (category.outgoingMinor > 0) names.add(category.name);
    return [...names].sort((a,b) => a.localeCompare(b, 'en-GB'));
  }
  function renderCategoryRules(rules = []) {
    const list = q('#manager-rule-list'); list.replaceChildren();
    const byName = new Map(rules.map(rule => [rule.name.trim().toLowerCase(), rule])), names = spendingCategories();
    q('#manager-category-count').textContent = names.length ? '· ' + names.length : '';
    for (const name of names) {
      const saved = byName.get(name.trim().toLowerCase()), card = node('div', undefined, 'manager-rule'); card.dataset.categoryRule = name;
      card.append(node('h4', name));
      const essentialLabel = node('label', undefined, 'check'), essential = node('input'); essential.type = 'checkbox'; essential.dataset.rule = 'essential'; essential.checked = saved ? saved.essential : true; essentialLabel.append(essential,node('span','Essential spending'));
      const targetLabel = node('label','Monthly cap (£)'), target = node('input'); target.type = 'number'; target.min = '0'; target.max = '10000000'; target.step = '0.01'; target.inputMode = 'decimal'; target.dataset.rule = 'target'; target.placeholder = 'Use recent average'; target.value = saved?.targetMonthlyMinor == null ? '' : (saved.targetMonthlyMinor / 100).toFixed(2); targetLabel.append(target);
      card.append(essentialLabel,targetLabel); list.append(card);
    }
    if (!names.length) list.append(node('p','No spending categories are available in this summary.'));
  }
  function setProfile(value) {
    const fields = q('#coach-profile-form').elements;
    fields.essential.value = value.essentialMonthlyMinor ? (value.essentialMonthlyMinor/100).toFixed(2) : '';
    fields.emergency.value = (value.emergencySavingsMinor/100).toFixed(2);
    fields.debt.value = value.highCostDebt; fields.horizon.value = String(value.horizonYears); fields.risk.value = value.risk;
    // Preserve supported custom horizons loaded from earlier rules.
    if (!fields.horizon.value) { const option=node('option',value.horizonYears+' years');option.value=String(value.horizonYears);fields.horizon.append(option);fields.horizon.value=String(value.horizonYears); }
    fields.managerSavings.value = ((value.manager?.savingsMonthlyMinor || 0)/100).toFixed(2);
    fields.managerInvestment.value = ((value.manager?.investmentMonthlyMinor || 0)/100).toFixed(2);
    fields.managerBuffer.value = ((value.manager?.cashBufferMinor || 0)/100).toFixed(2);
    renderCategoryRules(value.manager?.categories || []);
    q('#coach-goals').replaceChildren(); for (const goal of value.goals || []) addGoal(goal,false); q('#coach-add-goal').disabled = q('#coach-goals').children.length >= 8;
  }
  async function loadManagerRules() {
    const scope = managerScope, version = revision, account = readyAccount;
    q('#manager-rules-status').textContent = 'Checking saved rules for this budget…';
    try {
      const response = await (await api('api/finance/ai/manager-settings?scope='+encodeURIComponent(scope))).json();
      if (version !== revision || scope !== managerScope || account !== readyAccount || !session) return;
      savedRules = !!response.configured;
      if (savedRules && response.profile) { setProfile(response.profile); invalidate(); }
      q('#manager-rules-status').textContent = savedRules ? 'Your saved rules were loaded for this budget.' : 'These rules are not saved yet. Choose Save my rules to remember them.';
      schedulePlan(); updateControls();
    } catch (error) { if (version === revision && scope === managerScope) { savedRules = true; q('#manager-rules-status').textContent = error.message+' You can remove the saved rules and enter them again.'; updateControls(); schedulePlan(); } }
  }
  q('#manager-save-rules').onclick = async () => {
    if (!summary || !managerScope || rulesSaving || !q('#coach-profile-form').reportValidity()) return;
    const scope = managerScope, version = revision;
    let request;
    try { request = {scope,profile:profile(),remove:false}; }
    catch(error) { q('#manager-rules-status').textContent = error.message; return; }
    rulesSaving = true; updateControls();
    try {
      await verifyAccount(); if (version !== revision || scope !== managerScope) return;
      await api('api/finance/ai/manager-settings', request); if (version !== revision || scope !== managerScope) return;
      savedRules = true; q('#manager-rules-status').textContent = 'Rules saved for your OneAccount and this budget. Your transaction summary was not saved.';
    } catch (error) { if (version === revision) q('#manager-rules-status').textContent = error.message; }
    finally { rulesSaving = false; updateControls(); }
  };
  q('#manager-remove-rules').onclick = async () => {
    if (!managerScope || !savedRules || rulesSaving) return;
    const scope = managerScope, version = revision; rulesSaving = true; updateControls();
    try {
      await verifyAccount(); if (version !== revision || scope !== managerScope) return;
      await api('api/finance/ai/manager-settings',{scope,remove:true}); if (version !== revision || scope !== managerScope) return;
      savedRules = false; q('#manager-rules-status').textContent = 'Saved rules removed. The unsaved rules on this page still apply until you clear or leave it.';
    } catch (error) { if (version === revision) q('#manager-rules-status').textContent = error.message; }
    finally { rulesSaving = false; updateControls(); }
  };
  function useSummary(value, description, budgetRef = '') {
    if (!validSummary(value)) throw Error('This budget summary could not be read. Open your budget and try again.');
    const scope = value.source === 'manual' ? 'manual' : /^[a-f0-9]{64}$/.test(budgetRef) ? budgetRef : '';
    const sameScope = scope && scope === managerScope, previousRules = sameScope ? profile().manager.categories : [];
    summary = value; source = description; managerScope = scope;
    if (!sameScope) { q('#coach-profile-form').reset(); q('#coach-goals').replaceChildren(); q('#coach-add-goal').disabled = false; savedRules = false; }
    renderCategoryRules(previousRules); invalidate();
    q('#coach-data').hidden = false; q('#coach-source').textContent = source;
    const income = value.months.reduce((total, item) => total + item.incomeMinor, 0), outgoing = value.months.reduce((total, item) => total + item.outgoingMinor, 0);
    const metrics = q('#coach-metrics'); metrics.replaceChildren();
    for (const [label, amount] of [['Average in / month', Math.floor(income / value.months.length)], ['Average out / month', Math.ceil(outgoing / value.months.length)]]) {
      const metric = node('div'); metric.append(node('span', label), node('strong', money(amount))); metrics.append(metric);
    }
    q('#coach-months').replaceChildren(...value.months.map(item => row(monthName(item.month), item.incomeMinor, item.outgoingMinor)));
    q('#coach-categories').replaceChildren(...value.categories.map(item => row(item.name, item.incomeMinor, item.outgoingMinor)));
    status('Budget totals refreshed. Your allowances are being updated without an AI provider call.');
    q('#manager-source-details').open = false;
    q('#manager-rules-status').textContent = managerScope ? 'Rules apply to ' + (managerScope === 'manual' ? 'your manual estimates.' : 'this selected budget.') : 'Refresh this budget to enable saving its rules.';
    if (!sameScope && managerScope) loadManagerRules(); else schedulePlan();
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
      useSummary(envelope.summary, 'From your selected Actual budget · ' + envelope.summary.transactionCount + ' historical transaction lines · ' + envelope.summary.excludedTransfers + ' historical transfer entries excluded', envelope.budgetRef);
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
  q('#manager-refresh').onclick = () => q('#coach-open-budget').click();
  q('#manager-edit-rules').onclick = () => { q('#coach-profile-card').open = true; q('#coach-profile-card').scrollIntoView({behavior:'smooth',block:'start'}); };
  function dropSummary() { summary = null; source = ''; managerScope = ''; savedRules = false; q('#coach-data').hidden = true; clearTimeout(planTimer); invalidate(); }
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
  function addGoal(value = {}, focus = true) {
    if (q('#coach-goals').children.length >= 8) return;
    const card = node('fieldset', undefined, 'coach-goal'); card.append(node('legend', 'Savings goal'));
    for (const [field, title, type, value] of [['name','Goal name','text',''],['target','Target (£)','number',''],['saved','Already saved (£)','number','0'],['months','Months to reach it','number','12']]) {
      const label = node('label', title), input = node('input'); input.id = 'coach-goal-' + ++goalId; input.type = type; input.dataset.goal = field; input.required = true; input.value = value;
      if (type === 'number') { input.min = field === 'months' ? '1' : '0'; input.max = field === 'months' ? '600' : '10000000'; input.step = field === 'months' ? '1' : '0.01'; input.inputMode = field === 'months' ? 'numeric' : 'decimal'; }
      else { input.maxLength = 80; input.placeholder = 'For example, holiday'; }
      label.append(input); card.append(label);
    }
    if (value.name !== undefined) { card.querySelector('[data-goal=name]').value = value.name; card.querySelector('[data-goal=target]').value = (value.targetMinor/100).toFixed(2); card.querySelector('[data-goal=saved]').value = (value.savedMinor/100).toFixed(2); card.querySelector('[data-goal=months]').value = value.months; }
    const remove = node('button', 'Remove goal', 'secondary'); remove.type = 'button'; remove.onclick = () => { card.remove(); q('#coach-add-goal').disabled = false; profileChanged(); q('#coach-add-goal').focus(); };
    card.append(remove); q('#coach-goals').append(card); q('#coach-add-goal').disabled = q('#coach-goals').children.length >= 8;
    if (focus) card.querySelector('input').focus();
  }
  q('#coach-add-goal').onclick = () => { addGoal(); profileChanged(); };
  q('#coach-profile-form').addEventListener('input', profileChanged);
  function renderManager(manager) {
    if (!manager) return;
    q('#manager-dashboard').hidden = false;
    const metrics = q('#manager-metrics'); metrics.replaceChildren();
    const savings = manager.emergencyReservedMinor + manager.savingsReservedMinor + manager.goalReservedMinor;
    const spending = manager.categoryAllocatedMinor + manager.unassignedEssentialsAllocatedMinor;
    const remaining = manager.categories.reduce((n,c) => n + c.remainingMinor, 0);
    for (const [label, value, type] of [
      ['Planning income / month',money(manager.planningIncomeMinor),''],['Allocated to spending',money(spending),'spending'],
      ['Reserved for saving',money(savings),'reserved'],['Reserved for investing',money(manager.investmentReservedMinor),'reserved'],
      ['Cash buffer reserved',money(manager.cashBufferReservedMinor),'reserved'],['Left in category allowances',manager.currentAvailable ? money(remaining) : 'Refresh to track','remaining']
    ]) { const card=node('div',undefined,'manager-metric '+type);card.append(node('span',label),node('strong',value));metrics.append(card); }
    q('#manager-asof').textContent = manager.currentAvailable ? 'Spending snapshot: '+monthName(manager.currentMonth)+' · Through '+manager.asOf+'. Refresh from Actual for newer transactions.' : 'No current-month spending snapshot. These allowances are a forecast until you refresh a budget.';
    const problems = [];
    if (manager.essentialShortfallMinor > 0) problems.push('Essential spending is underfunded by '+money(manager.essentialShortfallMinor)+'.');
    if (manager.spendingFundingGapMinor > 0) problems.push('Recorded spending exceeds the funding available by '+money(manager.spendingFundingGapMinor)+'.');
    if (manager.unfundedTargetsMinor > 0) problems.push('Your requested reservations and category targets exceed the plan by '+money(manager.unfundedTargetsMinor)+'. Reduce flexible targets or adjust your priorities.');
    q('#manager-shortfall').hidden = problems.length === 0; q('#manager-shortfall').textContent = problems.join(' ');
    q('#manager-warnings').replaceChildren(...(manager.warnings || []).map(warning=>node('li',warning)));
    q('#manager-category-plan').replaceChildren(...manager.categories.filter(category=>category.wantedMinor>0 || category.limitMinor>0 || category.spentMinor>0).map(category => {
      const card = node('div',undefined,'manager-category'+(category.overspentMinor>0?' is-over':'')+'');
      const head=node('div',undefined,'manager-category-heading');head.append(node('h4',category.name),node('span',category.essential?'Essential':'Flexible','manager-category-kind'));card.append(head);
      const figures=node('dl',undefined,'manager-category-figures');
      for(const [label,value] of [['Allowance',money(category.limitMinor)],['Spent',manager.currentAvailable?money(category.spentMinor):'Not loaded'],['Left',manager.currentAvailable?money(category.remainingMinor):'Not loaded']]) { const pair=node('div');pair.append(node('dt',label),node('dd',value));figures.append(pair); } card.append(figures);
      const meter=node('progress');meter.max=Math.max(1,category.limitMinor);meter.value=manager.currentAvailable?Math.min(meter.max,category.spentMinor):0;meter.setAttribute('aria-label',category.name+' allowance used');card.append(meter);
      if(category.overspentMinor>0)card.append(node('p',money(category.overspentMinor)+' over the category target.','manager-over-text'));
      else if(category.reductionMinor>0)card.append(node('p',money(category.reductionMinor)+' below your requested target to keep the plan funded.','small'));
      return card;
    }));
    q('#manager-reserve-note').textContent = 'Savings reservations: emergency buffer '+money(manager.emergencyReservedMinor)+', named goals '+money(manager.goalReservedMinor)+', extra savings '+money(manager.savingsReservedMinor)+'. Debt reservation: '+money(manager.debtReservedMinor)+'. Unassigned essential spending: '+money(manager.unassignedEssentialsAllocatedMinor)+'. Not yet allocated: '+money(manager.unallocatedMinor)+'. '+(!manager.investmentEligible && manager.investmentReservedMinor>0?'Your investment reservation stays in cash while the readiness checks are incomplete.':'');
    q('#manager-uk-next').textContent = manager.investmentEligible ? (manager.investmentReservedMinor > 0 ? 'Your plan reserves '+money(manager.investmentReservedMinor)+' a month for longer-term investing. ' : 'The readiness checks are complete, but no money is allocated to investing yet. ')+'Compare suitable UK account wrappers, diversified fund types and total fees; use regulated advice for personal recommendations.' : (manager.investmentReservedMinor > 0 ? 'Keep the '+money(manager.investmentReservedMinor)+' investment reservation in cash for now. ' : 'No investment amount is allocated yet. ')+'First cover essential spending, review expensive debts and build accessible emergency savings.';
  }
  function renderPlan(value) {
    plan = value; q('#coach-plan').hidden = false;
    renderManager(value.manager);
    q('#coach-plan-basis').textContent = value.manager ? 'The plan starts with '+money(value.manager.planningIncomeMinor)+' monthly income, based on the lowest historical month. It protects essential commitments and spending already recorded, then funds your savings, goals, investment and buffer rules before flexible allowances. '+money(value.manager.unallocatedMinor)+' remains unallocated. These reservations do not move money.' : 'Planning income: ' + money(value.planningIncomeMinor) + ' a month. Spending reserve: ' + money(value.spendingReserveMinor) + '.';
    const warnings = [...new Set([...value.warnings,...(value.manager?.warnings || [])])];
    q('#coach-warnings').replaceChildren(...warnings.map(text => node('li', text)));
    q('#coach-warnings').hidden = warnings.length === 0;
    q('#coach-buckets').replaceChildren(...value.buckets.map(bucket => {
      const card = node('div', undefined, 'coach-bucket'); card.append(node('h3', bucket.name), node('strong', money(bucket.monthlyMinor) + ' / month'), node('p', bucket.purpose)); return card;
    }));
    q('#coach-emergency').textContent = 'A starting target of ' + money(value.emergencyTargetMinor) + ' covers three months of essential spending. ' + money(value.emergencyGapMinor) + ' remains to reach it. Adjust the target for your circumstances.';
    q('#coach-emergency-progress').value = value.emergencyTargetMinor ? Math.min(100,Math.max(0,(value.emergencyTargetMinor-value.emergencyGapMinor)/value.emergencyTargetMinor*100)) : 0;
    const goals = q('#coach-goal-plan'); goals.replaceChildren();
    const goalPlan=value.manager?.goals || value.goals;
    if (goalPlan.length) goals.append(node('h3', 'Your savings goals'));
    for (const goal of goalPlan) {
      const card = node('div', undefined, 'coach-goal-result');
      card.append(node('h4', goal.name), node('p', money(goal.remainingMinor) + ' still to save · ' + money(goal.requiredMonthlyMinor) + ' a month needed for your target date.'), node('p', 'This plan allows ' + money(goal.allocatedMonthlyMinor) + ' a month. ' + (goal.remainingMinor === 0 ? 'Your goal is already covered.' : goal.monthsToGoal === null ? 'There is no spare contribution for this goal yet; review the amount, date or priorities.' : 'At that rate: about ' + goal.monthsToGoal + ' months, before interest or changes.')));
      goals.append(card);
    }
    q('#coach-investment').textContent = value.investmentReason;
    updatePreview(); updateControls();
  }
  async function calculatePlan(userInitiated = false) {
    if (!summary || planning || analysing) return;
    if (!q('#coach-profile-form').checkValidity()) { if(userInitiated)q('#coach-profile-form').reportValidity(); return; }
    let request; try { request={summary,profile:profile()}; } catch(error) { status(error.message); return; }
    const version = revision; planning = true; updateControls(); status('Calculating your plan on your Finance server…');
    try {
      await verifyAccount(); if (version !== revision) return;
      const response = await (await api('api/finance/ai/plan', request)).json();
      if (version !== revision) return;
      renderPlan(response); status('Your allowances are up to date. Savings and investment reservations are excluded from spending.');
      if(userInitiated)q('#manager-dashboard').scrollIntoView({ behavior: 'smooth', block: 'start' });
    } catch (error) { if (version === revision) status(error.message); }
    finally { planning = false; updateControls(); if(version !== revision && summary)schedulePlan(); }
  }
  q('#coach-profile-form').onsubmit = event => { event.preventDefault(); clearTimeout(planTimer); calculatePlan(true); };
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
    const suggestion = result.advice.managerSuggestion;
    if (suggestion && summary) {
      managerSuggestion = {rules:suggestion,revision}; q('#manager-proposal').hidden = false;
      const preview=q('#manager-proposal-preview');preview.replaceChildren();
      preview.append(node('p',suggestion.reason),node('p','Extra savings '+money(suggestion.savingsMonthlyMinor)+' · Investment reservation '+money(suggestion.investmentMonthlyMinor)+' · Cash buffer '+money(suggestion.cashBufferMinor)+' each month.'));
      const list=node('ul');for(const category of suggestion.categories)list.append(node('li',category.name+': '+(category.essential?'essential':'flexible')+', '+(category.targetMonthlyMinor===null?'recent average':money(category.targetMonthlyMinor))+' monthly target.'));preview.append(list);
    }
    q('#coach-advice').focus({ preventScroll: true }); q('#coach-advice').scrollIntoView({ behavior: 'smooth', block: 'start' });
  }
  q('#manager-apply-proposal').onclick = () => {
    if (!managerSuggestion || managerSuggestion.revision !== revision || !summary) return;
    const rules=managerSuggestion.rules, current=profile(), merged=new Map(current.manager.categories.map(category=>[category.name.trim().toLowerCase(),category]));
    for(const category of rules.categories){const key=category.name.trim().toLowerCase();if(merged.has(key))merged.set(key,{...category,name:merged.get(key).name});}
    current.manager={enabled:true,savingsMonthlyMinor:rules.savingsMonthlyMinor,investmentMonthlyMinor:rules.investmentMonthlyMinor,cashBufferMinor:rules.cashBufferMinor,categories:[...merged.values()]};
    setProfile(current); profileChanged(); status('Suggested rules applied to this plan. Review the updated allowances, then save if you want to keep them.');
  };
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
    finally { analysing = false; resetConsent(); updateControls(); if(summary && !plan)schedulePlan(); }
  };
  function downloadPlan() {
    if (!plan) return;
    const data = { created: new Date().toISOString(), source, summary, profile: profile(), plan, ...(advice ? {advice:advice.advice,provider:advice.provider,model:advice.model} : {}) };
    const url = URL.createObjectURL(new Blob([JSON.stringify(data,null,2)],{type:'application/json'})), link = node('a');
    link.href = url; link.download = 'MyFinance-money-plan.json'; link.click(); setTimeout(()=>URL.revokeObjectURL(url),10000);
    status('Plan downloaded to your device. It contains your financial summary; keep it private.');
  }
  q('#coach-download').onclick = downloadPlan; q('#manager-download').onclick = downloadPlan;
  function clearData() {
    summary = null; source = ''; plan = null; advice = null; revision++;
    clearTimeout(planTimer); managerScope = ''; savedRules = false; managerSuggestion = null;
    q('#manager-dashboard').hidden = true; q('#manager-proposal').hidden = true;
    q('#coach-data').hidden = true; q('#coach-plan').hidden = true; q('#coach-advice').hidden = true;
    q('#coach-profile-form').reset(); q('#coach-advice-form').reset(); q('#coach-key').value = ''; q('#coach-goals').replaceChildren(); q('#coach-add-goal').disabled = false;
    for (const selector of ['#coach-months','#coach-categories','#coach-metrics','#coach-buckets','#coach-goal-plan','#coach-answer-sections','#manager-metrics','#manager-category-plan','#manager-rule-list','#manager-proposal-preview','#manager-warnings']) q(selector).replaceChildren();
    for (const selector of ['#coach-answer-summary','#coach-answer-meta','#coach-source','#coach-plan-basis','#coach-emergency','#coach-investment','#coach-warnings']) q(selector).textContent = '';
    q('#coach-emergency-progress').value = 0; renderManualMonths();
    for (const selector of ['#manager-asof','#manager-shortfall','#manager-reserve-note','#manager-category-count']) q(selector).textContent='';
    q('#manager-rules-status').textContent='Load a budget to save rules for it.';q('#manager-uk-next').textContent='Build a cash buffer and review expensive debts before committing money for the long term.';
    q('#manager-source-details').open=true;
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
