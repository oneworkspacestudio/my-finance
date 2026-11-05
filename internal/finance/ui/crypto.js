(() => {
  'use strict';
  const q = s => document.querySelector(s), base = 'api/finance/crypto';
  const form = q('#crypto-settings-form');
  let account = '', epoch = 0, data = null, busy = false, loading = false;
  let dirty = false, timer = 0, controller = null, wallet = null, reviewed = '', reviewTicket = 0, reviewBusy = false;
  let push = null, pushBusy = false, pushBound = false, localSubscription = null;
  let email = null, emailBusy = false;
  const emailForm = q('#crypto-email-form');
  let draftRevision = 0, draftConfig = '', settingsConflict = false;
  let tab = 'monitor';
  const active = () => location.hash === '#crypto' && !document.hidden && !!account;
  const state = () => data?.state;
  const revision = () => state()?.revision || 0;
  const current = (version, owner) => version === epoch && owner === account && owner === session?.user?.id;
  const text = (tag, value, className) => { const el = document.createElement(tag); el.textContent = value; if (className) el.className = className; return el; };
  const money = value => typeof value === 'string' && /^\d+(\.\d+)?$/.test(value) ? new Intl.NumberFormat('en-GB', { style: 'currency', currency: 'USD', maximumFractionDigits: 2 }).format(Number(value)) : 'Unavailable';
  const date = value => value && Number.isFinite(Date.parse(value)) ? new Date(value).toLocaleString('en-GB', { dateStyle: 'medium', timeStyle: 'short' }) : 'Not yet';
  const expired = item => !item.expiresAt || Date.parse(item.expiresAt) <= Date.now();
  const units = (value, decimals) => {
    if (!/^\d+$/.test(String(value))) return 'Unavailable';
    const n = String(value).padStart(decimals + 1, '0'), fraction = n.slice(-decimals).replace(/0+$/, '');
    return n.slice(0, -decimals) + (fraction ? '.' + fraction : '');
  };
  function message(value, error = false) { q('#crypto-message').textContent = value; q('#crypto-message').classList.toggle('is-error', error); }
  function resetReview() { reviewTicket++; reviewBusy = false; reviewed = ''; wallet?.reset(); }
  function stopPolling() { clearTimeout(timer); timer = 0; controller?.abort(); controller = null; }
  function schedule() { clearTimeout(timer); if (active()) timer = setTimeout(() => refresh(false), state()?.status === 'checking' ? 5000 : 20000); }
  function switchTab(value, focus = false) {
    tab = value;
    for (const name of ['monitor', 'proposals', 'settings']) {
      const button = q('#crypto-' + name + '-tab'), selected = name === value;
      button.setAttribute('aria-selected', String(selected)); button.tabIndex = selected ? 0 : -1;
      q('#crypto-' + name + '-panel').hidden = !selected;
      if (selected && focus) button.focus();
    }
    if (value === 'proposals' && account) wallet?.refresh();
  }
  function controls() {
    const s = state(), checking = s?.status === 'checking';
    q('#crypto-refresh').disabled = loading || busy || !account;
    q('#crypto-check').disabled = !s?.config.enabled || !data?.deepseekConfigured || busy || checking || s?.status === 'receipt' || dirty;
    q('#crypto-pause').disabled = !s?.config.enabled || busy;
    q('#crypto-save').disabled = busy || !account || settingsConflict || (q('#crypto-enabled').checked && (!data?.deepseekConfigured || !q('#crypto-consent').checked));
    q('#crypto-save').textContent = busy ? 'Saving…' : q('#crypto-enabled').checked ? 'Save & enable monitoring' : 'Save paused settings';
    for (const element of form.elements) if (element.id !== 'crypto-save') element.disabled = busy;
    q('#crypto-discard').disabled = busy || !dirty;
    q('#crypto-settings-note').textContent = settingsConflict ? 'Saved settings changed while you were editing. Discard your edits to review the current plan before saving again.' : 'Changing any setting clears this consent. Save with monitoring disabled to keep a paused plan.';
    q('#crypto-key-status').textContent = data?.deepseekConfigured ? 'DeepSeek key is saved. Monitoring uses deepseek-flash for its cost estimate.' : 'Save a DeepSeek API key with the deepseek-flash model to enable monitoring.';
    q('#crypto-key-status').classList.toggle('needs-action', !data?.deepseekConfigured);
    const supported = 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window && window.isSecureContext;
    q('#crypto-push-enable').disabled = pushBusy || !push?.available || !supported || Notification.permission === 'denied';
    q('#crypto-push-remove').disabled = pushBusy || !localSubscription;
    q('#crypto-push-test').disabled = pushBusy || !push?.available || !push?.subscriptions;
    for (const element of emailForm.elements) element.disabled = emailBusy || !account;
    q('#crypto-email-save').disabled = emailBusy || !account || (q('#crypto-email-enabled').checked && !q('#crypto-email-consent').checked);
    q('#crypto-email-remove').disabled = emailBusy || !account || !email?.configured;
    q('#crypto-email-test').disabled = emailBusy || !account || !email?.configured || !email?.enabled;
  }
  function configFromForm() {
    const decimal = (name, label, places) => {
      const value = form.elements[name].value.trim();
      if (!(new RegExp('^(?:0|[1-9]\\d*)(?:\\.\\d{1,' + places + '})?$')).test(value)) throw Error(label + ' must be a positive number with up to ' + places + ' decimal places.');
      return value;
    };
    const walletAddress = form.elements.cryptoWallet.value.trim();
    if (!/^0x[0-9a-fA-F]{40}$/.test(walletAddress) || /^0x0{40}$/.test(walletAddress)) throw Error('Enter a valid public wallet address starting with 0x. Never enter a private key.');
    const config = {
      enabled: q('#crypto-enabled').checked, walletAddress, chainId: 8453,
      budgetUsd: decimal('cryptoBudget', 'ETH exposure budget', 2), maxTradeUsd: decimal('cryptoMaxTrade', 'Trade limit', 2),
      reserveEth: decimal('cryptoReserve', 'ETH reserve', 18), slippageBps: Number(form.elements.cryptoSlippage.value),
      intervalMinutes: Number(form.elements.cryptoInterval.value), maxAnalysesPerDay: Number(form.elements.cryptoMaxAnalyses.value),
      maxSpendUsd: decimal('cryptoMaxSpend', 'Daily API cost limit', 2), objective: form.elements.cryptoObjective.value.trim()
    };
    if (Number(config.budgetUsd) < 1 || Number(config.budgetUsd) > 1000000) throw Error('Choose an ETH exposure budget from $1 to $1,000,000.');
    if (Number(config.maxTradeUsd) < 1 || Number(config.maxTradeUsd) > Number(config.budgetUsd)) throw Error('The trade limit must be at least $1 and no more than the ETH exposure budget.');
    if (Number(config.reserveEth) > 1000) throw Error('The ETH reserve must be between 0 and 1,000 ETH.');
    if (!Number.isInteger(config.maxAnalysesPerDay) || config.maxAnalysesPerDay < 1 || config.maxAnalysesPerDay > 96) throw Error('Choose between 1 and 96 analyses per UTC day.');
    if (Number(config.maxSpendUsd) < .01 || Number(config.maxSpendUsd) > 10) throw Error('Choose a daily estimated API cost limit from $0.01 to $10.');
    if (!config.objective || new TextEncoder().encode(config.objective).length > 1000) throw Error('Describe your objective in up to 1,000 UTF-8 bytes.');
    return config;
  }
  function preview() {
    const values = {};
    for (const [label, name] of [['ETH exposure budget (USD)', 'cryptoBudget'], ['Trade limit (USD)', 'cryptoMaxTrade'], ['Fee reserve (ETH)', 'cryptoReserve'], ['Interval (minutes)', 'cryptoInterval'], ['Analyses per UTC day', 'cryptoMaxAnalyses'], ['Estimated API cap (USD/day)', 'cryptoMaxSpend'], ['Objective', 'cryptoObjective']]) values[label] = form.elements[name].value;
    values['Maximum slippage'] = (Number(form.elements.cryptoSlippage.value) / 100) + '%';
    values['Additional analysis input'] = 'Public ETH/USDC balances, live market data and bounded public evidence. No wallet address or private ledger is sent to the model.';
    q('#crypto-consent-preview').textContent = JSON.stringify(values, null, 2);
  }
  function fillSettings(config) {
    if (!config) return;
    draftRevision = revision(); draftConfig = JSON.stringify(config); settingsConflict = false;
    const mapping = { cryptoWallet: 'walletAddress', cryptoBudget: 'budgetUsd', cryptoMaxTrade: 'maxTradeUsd', cryptoReserve: 'reserveEth', cryptoSlippage: 'slippageBps', cryptoInterval: 'intervalMinutes', cryptoMaxAnalyses: 'maxAnalysesPerDay', cryptoMaxSpend: 'maxSpendUsd', cryptoObjective: 'objective' };
    for (const [name, property] of Object.entries(mapping)) {
      const input = form.elements[name], value = String(config[property] ?? '');
      if (input.tagName === 'SELECT' && ![...input.options].some(option => option.value === value)) input.add(new Option(name === 'cryptoSlippage' ? Number(value) / 100 + '%' : value + ' minutes', value));
      input.value = value;
    }
    q('#crypto-enabled').checked = !!config.enabled; q('#crypto-consent').checked = false; preview();
  }
  function facts(container, entries) {
    container.replaceChildren();
    for (const [label, value] of entries) { const item = document.createElement('div'); item.append(text('dt', label), text('dd', value)); container.append(item); }
  }
  function sourceLink(url, label) {
    try { const parsed = new URL(url); if (parsed.protocol !== 'https:' || parsed.username || parsed.password) return null; const link = text('a', label); link.href = parsed.href; link.target = '_blank'; link.rel = 'noopener noreferrer'; return link; } catch { return null; }
  }
  function evidenceCard(item, compact = false) {
    const card = document.createElement('article'); card.className = 'crypto-evidence-item';
    const kind = { market: 'Market data', news: 'News result', 'indexed-social': 'Indexed social result' }[item.kind] || 'Public source';
    card.append(text('span', kind, 'tag'), text('h3', item.title || 'Untitled source'));
    if (item.snippet && !compact) card.append(text('p', item.snippet));
    card.append(text('p', (item.publishedAt ? 'Published ' + date(item.publishedAt) + ' · ' : '') + 'Retrieved ' + date(item.fetchedAt), 'small'));
    const link = sourceLink(item.url, 'Open source ↗'); if (link) card.append(link);
    return card;
  }
  function render() {
    const s = state(), enabled = !!s?.config.enabled;
    const statuses = { idle: 'Ready', scheduled: 'Monitoring enabled', checking: 'Checking public sources', paused: 'Monitoring paused', hold: 'Hold — no action proposed', proposal: 'A proposal is ready', receipt: 'Check transaction receipt', limit: 'Daily limit reached', error: 'Check needs attention' };
    q('#crypto-state-title').textContent = s ? statuses[s.status] || 'Monitoring status' : 'Set your boundaries first';
    q('#crypto-state-badge').textContent = enabled ? (s?.status === 'checking' ? 'Checking' : 'Enabled') : s ? 'Paused' : 'Not configured';
    q('#crypto-state-badge').classList.toggle('is-on', enabled);
    q('#crypto-state-copy').textContent = s ? enabled ? 'Checks continue while this Workspace host is online, including when your browser is closed. Each real transaction still needs your approval.' : 'Scheduled analysis is off. Your saved plan remains available; open Settings to enable monitoring.' : 'Set your ETH exposure and trade limits, choose a public Base wallet address, and save explicit consent for DeepSeek checks.';
    facts(q('#crypto-timing'), s ? [['Last check', date(s.lastRun)], ['Next scheduled check', enabled ? date(s.nextRun) : 'Paused']] : []);
    q('#crypto-monitor-error').hidden = !s?.lastError; q('#crypto-monitor-error').textContent = s?.lastError || '';
    if (s?.status === 'receipt') q('#crypto-state-copy').textContent = 'A submitted transaction is unresolved. Open Proposals and check its receipt before more analysis can run. Do not assume it failed or send it again.';
    q('#crypto-open-settings').textContent = s?.status === 'receipt' ? 'Review transaction status' : s ? 'Adjust settings' : 'Set up monitoring';
    const market = q('#crypto-market'); market.replaceChildren();
    if (s?.market) {
      for (const [token, value, subtitle] of [['ETH', s.market.ethUsd, 'Ethereum on Base'], ['USDC', s.market.usdcUsd, 'USD Coin on Base']]) {
        const card = document.createElement('div'); card.className = 'crypto-token'; card.append(text('span', token, 'crypto-token-symbol'), text('strong', money(value)), text('small', subtitle)); market.append(card);
      }
      q('#crypto-market-asof').replaceChildren(text('span', 'Retrieved ' + date(s.market.fetchedAt) + ' · Pool liquidity ' + money(s.market.liquidityUsd) + '. '));
      const link = sourceLink(s.market.sourceUrl, 'Market source ↗'); if (link) q('#crypto-market-asof').append(link);
    } else { market.append(text('p', 'No market data yet. A successful check retrieves live sources; there are no sample prices.')); q('#crypto-market-asof').textContent = ''; }
    q('#crypto-balances-details').hidden = !s?.wallet;
    facts(q('#crypto-balances'), s?.wallet ? [['ETH', units(s.wallet.eth, 18)], ['USDC', units(s.wallet.usdc, 6)], ['Read at', date(s.wallet.fetchedAt)], ['Network', 'Base · 8453']] : []);
    q('#crypto-analysis-card').hidden = !s?.lastAnalysis;
    q('#crypto-analysis').textContent = s?.lastAnalysis || '';
    q('#crypto-analysis-risks').replaceChildren(); for (const risk of s?.lastRisks || []) q('#crypto-analysis-risks').append(text('li', risk));
    const evidence = q('#crypto-evidence'); evidence.replaceChildren();
    for (const item of s?.evidence || []) evidence.append(evidenceCard(item));
    if (!evidence.children.length) evidence.append(text('p', 'No evidence retrieved yet.'));
    facts(q('#crypto-usage'), s?.usage ? [['UTC day', s.usage.day], ['Analyses used', String(s.usage.analyses)], ['Analyses remaining', String(s.usage.remainingAnalyses)], ['Reserved cost estimate', money(s.usage.reservedSpendUsd) + ' / ' + money(s.usage.maxSpendUsd)]] : []);
    if (!s?.usage) q('#crypto-usage').append(text('p', 'No paid requests recorded.'));
    renderProposals(); controls();
  }
  function renderProposals() {
    const s = state(), list = q('#crypto-proposals'), history = q('#crypto-history'); list.replaceChildren(); history.replaceChildren();
    const pending = (s?.proposals || []).filter(item => item.status === 'pending' && !expired(item));
    q('#crypto-proposal-count').textContent = String(pending.length); q('#crypto-proposal-count').hidden = !pending.length;
    if (!pending.length) { const empty = document.createElement('article'); empty.className = 'investment-card crypto-empty'; empty.append(text('h3', 'No proposal waiting for review'), text('p', 'A check can recommend holding. Previous proposals may also expire; refresh status for the latest result.')); list.append(empty); }
    for (const proposal of pending) {
      const card = document.createElement('article'); card.className = 'investment-card crypto-proposal';
      card.append(text('span', 'FOR YOUR REVIEW · BASE', 'tag'), text('h2', proposal.sellToken + ' → ' + proposal.buyToken));
      const amount = units(proposal.sellAmount, proposal.sellToken === 'ETH' ? 18 : 6);
      card.append(text('p', amount + ' ' + proposal.sellToken + ' · estimated ' + money(proposal.estimatedSellUsd), 'crypto-proposal-amount'), text('p', proposal.rationale));
      card.append(text('p', 'Created ' + date(proposal.createdAt) + ' · Expires ' + date(proposal.expiresAt), 'small'));
      if (proposal.risks?.length) { card.append(text('h3', 'Risks to consider')); const risks = document.createElement('ul'); for (const risk of proposal.risks) risks.append(text('li', risk)); card.append(risks); }
      const refs = (s.evidence || []).filter(item => proposal.evidenceIds?.includes(item.id));
      if (refs.length) { const details = document.createElement('details'); details.append(text('summary', 'Evidence behind this proposal')); for (const item of refs) details.append(evidenceCard(item, true)); card.append(details); }
      const actions = document.createElement('div'); actions.className = 'crypto-actions';
      const review = text('button', reviewed === proposal.id ? 'Refresh transaction review' : 'Review with my wallet'); review.type = 'button'; review.disabled = busy || reviewBusy || dirty || !s.config.enabled || !wallet;
      review.onclick = async () => {
        if (reviewBusy) return;
        const version = epoch, owner = account, intendedRevision = revision(), ticket = ++reviewTicket;
        reviewBusy = true; review.disabled = true;
        try {
          await verifyAccount(); if (!current(version, owner) || ticket !== reviewTicket || dirty || !state()?.config.enabled || revision() !== intendedRevision) return;
          reviewed = proposal.id; await wallet.review(proposal.id);
          if (current(version, owner) && ticket === reviewTicket) q('#crypto-wallet-panel').scrollIntoView({ behavior: 'smooth', block: 'start' });
        } catch (error) { if (current(version, owner) && ticket === reviewTicket) message(error.message, true); }
        finally { if (current(version, owner) && ticket === reviewTicket) { reviewBusy = false; renderProposals(); } }
      };
      const dismiss = text('button', 'Dismiss'); dismiss.type = 'button'; dismiss.className = 'secondary'; dismiss.disabled = busy; dismiss.onclick = () => mutate('/dismiss', { revision: revision(), proposalId: proposal.id }, 'Proposal dismissed.');
      actions.append(review, dismiss); card.append(actions);
      if (!s.config.enabled || dirty) card.append(text('p', dirty ? 'Save or discard your changed settings before reviewing a transaction.' : 'Monitoring is paused. Enable a fresh check before reviewing a transaction.', 'small'));
      card.append(text('p', 'This is a proposal, not an order. Fees and minimum output are shown in the fresh wallet review.', 'small')); list.append(card);
    }
    for (const proposal of s?.proposals || []) {
      if (pending.includes(proposal)) continue;
      const item = document.createElement('article'); item.className = 'crypto-history-item';
      item.append(text('strong', proposal.sellToken + ' → ' + proposal.buyToken), text('span', proposal.status === 'pending' ? 'Expired' : proposal.status, 'crypto-state'), text('p', proposal.rationale), text('p', date(proposal.createdAt), 'small'));
      if (proposal.transactionHash) item.append(text('p', 'Transaction: ' + proposal.transactionHash, 'crypto-hash'));
      if (['submitted', 'unknown'].includes(proposal.status)) item.append(text('p', 'Submitted or unresolved is not confirmation of a completed swap.', 'small'));
      history.append(item);
    }
    q('#crypto-history-details').hidden = !history.children.length;
  }
  async function verifyAccount() {
    const expected = account;
    if (!expected || expected !== session?.user?.id) throw Error('Sign in again before continuing.');
    const fresh = await (await api('api/me')).json();
    if (fresh.user?.id !== expected) {
      window.dispatchEvent(new Event('finance-clear')); session = undefined;
      q('#content').hidden = true; q('#signin').hidden = false; q('#signout').hidden = true;
      q('#local-login').hidden = !!bootstrap?.oidc; q('#signin .button').hidden = !bootstrap?.oidc;
      throw Error('Your account changed. Reload and sign in before continuing.');
    }
  }
  function apply(value, fill = false) {
    if (value?.state && (!Number.isSafeInteger(value.state.revision) || !value.state.config)) throw Error('The monitoring response was incomplete. Refresh to try again.');
    const old = state();
    if (old && (JSON.stringify(old.config) !== JSON.stringify(value.state?.config) || (reviewed && !value.state?.proposals?.some(item => item.id === reviewed && item.status === 'pending' && !expired(item))))) resetReview();
    data = value;
    if (dirty && state()) {
      if (draftConfig && draftConfig !== JSON.stringify(state().config)) settingsConflict = true;
      else draftRevision = revision();
    }
    if (fill && !dirty) fillSettings(state()?.config); render();
  }
  async function refresh(notify = true) {
    if (!account || loading || busy) return;
    const version = epoch, owner = account, request = new AbortController(); loading = true; controller = request; controls();
    try { const value = await (await api(base, undefined, { signal: request.signal })).json(); if (!current(version, owner) || controller !== request) return; apply(value, true); if (notify) message('Monitoring status refreshed. No new analysis was requested.'); }
    catch (error) { if (current(version, owner) && error.name !== 'AbortError') message(error.message, true); }
    finally { if (current(version, owner) && controller === request) { loading = false; controller = null; controls(); schedule(); } }
  }
  async function mutate(path, body, success) {
    if (busy || !account) return;
    const version = epoch, owner = account; busy = true; stopPolling(); loading = false; resetReview(); controls();
    try {
      await verifyAccount(); if (!current(version, owner)) return;
      const value = await (await api(base + path, body)).json();
      if (!current(version, owner)) return;
      if (path === '/settings') { dirty = false; q('#crypto-consent').checked = false; }
      apply(value, true); message(success);
    } catch (error) { if (current(version, owner)) { message(error.message + ' Refresh status before retrying; a failed request is not retried automatically.', true); q('#crypto-consent').checked = false; } }
    finally { if (current(version, owner)) { busy = false; controls(); schedule(); } }
  }
  function pushScope() { return new URL('./', endpoint('')).href; }
  function keyBytes(value) { const encoded = value.replace(/-/g, '+').replace(/_/g, '/'); return Uint8Array.from(atob(encoded.padEnd(Math.ceil(encoded.length / 4) * 4, '=')), c => c.charCodeAt(0)); }
  function matchingKey(subscription) {
    if (!subscription?.options?.applicationServerKey || !push?.publicKey) return false;
    const actual = new Uint8Array(subscription.options.applicationServerKey), expected = keyBytes(push.publicKey);
    return actual.length === expected.length && actual.every((v, i) => v === expected[i]);
  }
  async function activeRegistration(registration) {
    const start = Date.now();
    while (!registration.active) {
      if (Date.now() - start > 10000) throw Error('The Finance notification worker did not activate. Reload and try again.');
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    return registration;
  }
  function pushStatus(extra = '') {
    const supported = 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window && window.isSecureContext;
    let value = !supported ? 'Web notifications are not available in this browser. Use a supported HTTPS browser.' : !push?.available ? 'Notifications are unavailable on this Finance address.' : Notification.permission === 'denied' ? 'Notifications are blocked. Change this site’s notification permission in your browser settings.' : pushBound ? 'Notifications are enabled for this browser.' : 'Notifications need your permission and explicit registration on this device.';
    if (push?.subscriptions) value += ' ' + push.subscriptions + ' registered device' + (push.subscriptions === 1 ? '' : 's') + ' for this OneAccount.';
    q('#crypto-push-status').textContent = extra || value;
    const ios = /iPad|iPhone|iPod/.test(navigator.userAgent) || navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1;
    q('#crypto-push-help').textContent = ios && !matchMedia('(display-mode: standalone)').matches ? 'On iPhone or iPad, add My Finance to your Home Screen, open it there, then enable notifications. Alerts never approve transactions.' : 'Delivery depends on your browser, network and device settings. A test being accepted by the server does not prove it was displayed. Alerts never approve transactions.';
    controls();
  }
  async function loadPush() {
    const version = epoch, owner = account;
    try {
      const value = await (await api(base + '/push')).json(); if (!current(version, owner)) return; push = value;
      if ('serviceWorker' in navigator) { const reg = await navigator.serviceWorker.getRegistration(pushScope()); if (!current(version, owner)) return; const subscription = reg?.scope === pushScope() ? await reg.pushManager.getSubscription() : null; if (!current(version, owner)) return; localSubscription = subscription; }
      if (current(version, owner)) pushStatus();
    } catch (error) { if (current(version, owner)) pushStatus('Notification settings could not load: ' + error.message); }
  }
  async function enablePush() {
    if (pushBusy || !push?.publicKey) return;
    const version = epoch, owner = account; pushBusy = true; controls();
    let created = null;
    try {
      // The permission prompt must originate directly from the user's click.
      const permission = await Notification.requestPermission(); if (!current(version, owner)) return;
      if (permission !== 'granted') { pushStatus('Notification permission was not granted. Monitoring settings are unchanged.'); return; }
      await verifyAccount(); if (!current(version, owner)) return;
      const registration = await navigator.serviceWorker.register(endpoint('finance-push-sw.js').href, { scope: pushScope(), updateViaCache: 'none' });
      if (registration.scope !== pushScope()) throw Error('The notification worker has an unexpected scope.');
      const ready = await activeRegistration(registration);
      if (!current(version, owner) || ready.scope !== pushScope()) return;
      let subscription = await ready.pushManager.getSubscription();
      if (!current(version, owner)) return;
      if (subscription && !matchingKey(subscription)) { await subscription.unsubscribe(); subscription = null; }
      if (!current(version, owner)) return;
      if (!subscription) { subscription = await ready.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: keyBytes(push.publicKey) }); created = subscription; }
      if (!current(version, owner)) { if (created) await created.unsubscribe(); return; }
      await verifyAccount(); if (!current(version, owner)) { if (created) await created.unsubscribe(); return; }
      const payload = subscription.toJSON(); await api(base + '/push/subscribe', { endpoint: payload.endpoint, keys: payload.keys });
      if (!current(version, owner)) { if (created) await created.unsubscribe(); return; }
      localSubscription = subscription; pushBound = true; await loadPush(); pushStatus();
    } catch (error) { if (created && !current(version, owner)) await created.unsubscribe().catch(() => {}); if (current(version, owner)) pushStatus('Notifications could not be enabled: ' + error.message); }
    finally { if (current(version, owner)) { pushBusy = false; controls(); } }
  }
  async function removePush() {
    if (pushBusy || !localSubscription) return;
    const version = epoch, owner = account, subscription = localSubscription; pushBusy = true; controls();
    try {
      await verifyAccount(); if (!current(version, owner)) return;
      if (matchingKey(subscription)) await api(base + '/push/remove', { endpoint: subscription.endpoint });
      if (!current(version, owner)) return;
      await subscription.unsubscribe(); if (!current(version, owner)) return;
      localSubscription = null; pushBound = false; await loadPush(); pushStatus('Notifications are disabled for this browser. Monitoring can continue; other registered devices are unchanged.');
    } catch (error) { if (current(version, owner)) pushStatus(error.message); }
    finally { if (current(version, owner)) { pushBusy = false; controls(); } }
  }
  async function testPush() {
    if (pushBusy) return;
    const version = epoch, owner = account; pushBusy = true; controls();
    try { await verifyAccount(); if (!current(version, owner)) return; await api(base + '/push/test', {}); if (current(version, owner)) pushStatus('Test alert requested for your registered devices. Check the device; this is not a delivery confirmation.'); }
    catch (error) { if (current(version, owner)) pushStatus(error.message); }
    finally { if (current(version, owner)) { pushBusy = false; controls(); } }
  }
  function applyEmail(value, note = '') {
    email = value;
    q('#crypto-email-from').value = value.from || ''; q('#crypto-email-to').value = value.to || '';
    q('#crypto-email-key').value = ''; q('#crypto-email-enabled').checked = !!value.enabled; q('#crypto-email-consent').checked = false;
    const status = !value.configured ? 'No Resend API key saved.' : value.enabled ? 'Email alerts enabled for this OneAccount.' : 'Resend key saved. Email alerts are paused.';
    q('#crypto-email-status').textContent = note || status + (value.lastAcceptedAt ? ' Last accepted by the provider: ' + date(value.lastAcceptedAt) + '. Acceptance does not confirm delivery.' : '') + (value.lastError ? ' ' + value.lastError : '');
    controls();
  }
  async function loadEmail() {
    const version = epoch, owner = account;
    try { const value = await (await api(base + '/email')).json(); if (current(version, owner)) applyEmail(value); }
    catch (error) { if (current(version, owner)) { q('#crypto-email-status').textContent = 'Email settings could not load: ' + error.message; controls(); } }
  }
  async function emailAction(path, payload, note) {
    if (emailBusy || !account) return;
    const version = epoch, owner = account; emailBusy = true; controls();
    try {
      await verifyAccount(); if (!current(version, owner)) return;
      const result = await (await api(base + '/email' + path, payload)).json(); if (!current(version, owner)) return;
      q('#crypto-email-key').value = ''; q('#crypto-email-consent').checked = false;
      if (!path) applyEmail(result, note); else q('#crypto-email-status').textContent = note;
    } catch (error) { if (current(version, owner)) { q('#crypto-email-key').value = ''; q('#crypto-email-consent').checked = false; q('#crypto-email-status').textContent = error.message + ' No automatic retry was made.'; } }
    finally { if (current(version, owner)) { emailBusy = false; controls(); } }
  }
  function clear() {
    epoch++; account = ''; stopPolling(); loading = false; busy = false; dirty = false; data = null; push = null; pushBusy = false; pushBound = false;
    email = null; emailBusy = false; emailForm.reset(); for (const name of ['from', 'to', 'apiKey']) emailForm.elements[name].value = ''; q('#crypto-email-status').textContent = 'Sign in to manage email alerts.';
    draftRevision = 0; draftConfig = ''; settingsConflict = false;
    const subscription = localSubscription; localSubscription = null; if (subscription) subscription.unsubscribe().catch(() => {});
    wallet?.destroy(); wallet = null; reviewed = ''; form.reset(); q('#crypto-wallet-address').value = ''; form.elements.cryptoObjective.value = '';
    q('#crypto-consent-preview').textContent = ''; q('#crypto-wallet-panel').replaceChildren();
    render(); pushStatus('Sign in to manage notifications.'); message('Sign in again to use Crypto monitor.');
  }
  async function activate() {
    if (!session?.user?.id || location.hash !== '#crypto') return;
    if (account && account !== session.user.id) clear();
    if (!account) {
      account = session.user.id; epoch++;
      if (window.FinanceCryptoWallet) { wallet = window.FinanceCryptoWallet.create({ api, accountId: () => account, container: q('#crypto-wallet-panel'), onChange: () => refresh(false) }); wallet.refresh(); }
      else message('Wallet review is currently unavailable. Monitoring can still be configured.', true);
      preview(); await Promise.allSettled([refresh(false), loadPush(), loadEmail()]);
    } else await refresh(false);
    schedule();
  }
  for (const name of ['monitor', 'proposals', 'settings']) q('#crypto-' + name + '-tab').onclick = () => switchTab(name);
  q('.crypto-tabs').addEventListener('keydown', event => {
    const names = ['monitor', 'proposals', 'settings']; let next;
    if (event.key === 'ArrowRight') next = (names.indexOf(tab) + 1) % 3;
    else if (event.key === 'ArrowLeft') next = (names.indexOf(tab) + 2) % 3;
    else if (event.key === 'Home') next = 0; else if (event.key === 'End') next = 2; else return;
    event.preventDefault(); switchTab(names[next], true);
  });
  form.addEventListener('input', event => {
    if (event.target.id === 'crypto-consent') { controls(); return; }
    dirty = true; q('#crypto-consent').checked = false; resetReview(); preview(); controls(); renderProposals();
  });
  form.onsubmit = event => {
    event.preventDefault();
    try { if (settingsConflict) throw Error('Review the current saved settings before replacing them.'); const config = configFromForm(); if (config.enabled && !q('#crypto-consent').checked) throw Error('Review and approve background sharing and provider charges first.'); mutate('/settings', { revision: draftRevision, consent: q('#crypto-consent').checked, config }, config.enabled ? 'Monitoring is enabled within your saved limits. The server handles scheduled checks.' : 'Paused settings saved. No scheduled analysis is enabled.'); }
    catch (error) { message(error.message, true); }
  };
  q('#crypto-refresh').onclick = () => refresh();
  q('#crypto-pause').onclick = () => mutate('/pause', { revision: revision() }, 'Monitoring paused and new reviews invalidated. A request already opened in your wallet or submitted cannot be retracted; reject any pending wallet request you no longer want.');
  q('#crypto-check').onclick = () => mutate('/check', { revision: revision() }, 'One check was requested within your existing limits. Refresh status to see its outcome.');
  q('#crypto-open-settings').onclick = () => { const destination = state()?.status === 'receipt' ? 'proposals' : 'settings'; switchTab(destination); q('#crypto-' + destination + '-tab').focus(); };
  q('#crypto-discard').onclick = () => { dirty = false; settingsConflict = false; resetReview(); if (state()) fillSettings(state().config); else { form.reset(); draftRevision = 0; draftConfig = ''; preview(); } controls(); renderProposals(); message('Edits discarded. Review the saved settings before enabling monitoring.'); };
  q('#crypto-connect-wallet').onclick = async () => {
    if (!wallet || busy) return;
    const version = epoch, owner = account; switchTab('proposals');
    try {
      const address = await wallet.connect(); if (!current(version, owner) || !address) return;
      form.elements.cryptoWallet.value = address; dirty = true; q('#crypto-consent').checked = false;
      preview(); controls(); switchTab('settings'); q('#crypto-wallet-address').focus();
      message('Public wallet address selected. Review the Base network and your limits before saving.');
    } catch (error) { if (current(version, owner)) message(error.message, true); }
  };
  q('#crypto-push-enable').onclick = enablePush; q('#crypto-push-remove').onclick = removePush; q('#crypto-push-test').onclick = testPush;
  emailForm.addEventListener('input', event => { if (event.target.id !== 'crypto-email-consent') q('#crypto-email-consent').checked = false; controls(); });
  emailForm.onsubmit = event => {
    event.preventDefault();
    const payload = { revision: email?.revision || 0, enabled: q('#crypto-email-enabled').checked, from: q('#crypto-email-from').value.trim(), to: q('#crypto-email-to').value.trim(), apiKey: q('#crypto-email-key').value.trim(), consent: q('#crypto-email-consent').checked };
    if (payload.enabled && !payload.consent) { q('#crypto-email-status').textContent = 'Approve the email sharing and provider charges before enabling alerts.'; return; }
    if (!email?.configured && !payload.apiKey) { q('#crypto-email-status').textContent = 'Enter your Resend API key before saving this connection.'; return; }
    emailAction('', payload, payload.enabled ? 'Email alerts enabled. Use Send test email to request a test.' : 'Email settings saved with alerts paused.');
  };
  q('#crypto-email-remove').onclick = () => emailAction('', { revision: email?.revision || 0, remove: true }, 'Email alerts disabled and the saved API key removed. Revoke the key at Resend if you want to end its provider access.');
  q('#crypto-email-test').onclick = () => emailAction('/test', {}, 'Test email queued for your saved recipient. Check your inbox; this does not confirm delivery.');
  window.addEventListener('finance-ready', activate); window.addEventListener('finance-clear', clear);
  window.addEventListener('hashchange', () => { if (location.hash === '#crypto') activate(); else { stopPolling(); loading = false; resetReview(); } });
  document.addEventListener('visibilitychange', () => { if (document.hidden) { stopPolling(); loading = false; resetReview(); } else if (location.hash === '#crypto') activate(); });
  window.addEventListener('pagehide', () => { stopPolling(); resetReview(); });
  window.addEventListener('focus', async () => { if (active()) try { await verifyAccount(); } catch (error) { message(error.message, true); } });
  controls(); if (session?.user?.id && location.hash === '#crypto') activate();
})();
