/* Deterministic Base ETH/USDC wallet requests. No private keys or autonomous sends. */
const CW_ROUTER = '0x2626664c2603336e57b271c5c0b26f421741e481';
const CW_USDC = '0x833589fcd6edb6e08f4c7c32d4f71b54bda02913';
const CW_WETH = '0x4200000000000000000000000000000000000006';
const cwAddress = value => /^0x[0-9a-f]{40}$/i.test(value) && !/^0x0{40}$/i.test(value);
const cwWord = value => { const n = BigInt(value); if (n < 0n || n >= 2n ** 256n) throw Error('Invalid amount'); return n.toString(16).padStart(64, '0'); };
const cwAddressWord = value => { if (!cwAddress(value)) throw Error('Invalid wallet address'); return value.toLowerCase().slice(2).padStart(64, '0'); };
function cwBytesArray(calls) {
  let offset = calls.length * 32, head = '', tail = '';
  for (const data of calls) { const entry = cwWord(data.length / 2) + data.padEnd(Math.ceil(data.length / 64) * 64, '0'); head += cwWord(offset); tail += entry; offset += entry.length / 2; }
  return cwWord(calls.length) + head + tail;
}
function cwTransaction(quote, kind) {
  if (quote.chainId !== 8453 || !cwAddress(quote.walletAddress) || !/^[1-9][0-9]{0,77}$/.test(quote.sellAmount) || !/^[1-9][0-9]{0,77}$/.test(quote.minBuyAmount)) throw Error('Invalid quote');
  if (!((quote.sellToken === 'ETH' && quote.buyToken === 'USDC') || (quote.sellToken === 'USDC' && quote.buyToken === 'ETH'))) throw Error('Unsupported token pair');
  const tx = {from:quote.walletAddress.toLowerCase(),to:CW_ROUTER,chainId:'0x2105',value:'0x0',data:''};
  if (kind === 'approve' || kind === 'reset') {
    if (quote.sellToken !== 'USDC') throw Error('ETH needs no token approval');
    tx.to = CW_USDC; tx.data = '0x095ea7b3' + cwAddressWord(CW_ROUTER) + cwWord(kind === 'reset' ? 0 : quote.sellAmount); return tx;
  }
  if (kind !== 'swap' || !Number.isSafeInteger(quote.deadline) || quote.deadline <= 0) throw Error('Invalid transaction');
  const eth = quote.sellToken === 'ETH';
  const swap = '04e45aaf' + cwAddressWord(eth ? CW_WETH : CW_USDC) + cwAddressWord(eth ? CW_USDC : CW_WETH) + cwWord(500) + cwAddressWord(eth ? quote.walletAddress : CW_ROUTER) + cwWord(quote.sellAmount) + cwWord(quote.minBuyAmount) + cwWord(0);
  const tail = eth ? '12210e8a' : '49404b7c' + cwWord(quote.minBuyAmount) + cwAddressWord(quote.walletAddress);
  tx.data = '0x5ae401dc' + cwWord(quote.deadline) + cwWord(64) + cwBytesArray([swap,tail]);
  if (eth) tx.value = '0x' + BigInt(quote.sellAmount).toString(16);
  return tx;
}
function cwVerifyTransaction(quote, kind, tx) {
  const expected = cwTransaction(quote, kind);
  if (!tx || Object.keys(tx).some(key => !['from','to','chainId','value','data','gas'].includes(key))) throw Error('Unexpected wallet request fields');
  for (const key of Object.keys(expected)) if (typeof tx[key] !== 'string' || tx[key].toLowerCase() !== expected[key].toLowerCase()) throw Error('Wallet request differs from the reviewed swap');
  if (!/^0x[0-9a-f]+$/i.test(tx.gas || '') || BigInt(tx.gas) < 21000n || BigInt(tx.gas) > 1200000n) throw Error('Unexpected gas limit');
  return tx;
}
function cwFormat(amount, token) { const decimals = token === 'USDC' ? 6 : 18; const s = String(amount).padStart(decimals+1,'0'); return s.slice(0,-decimals)+'.'+s.slice(-decimals).replace(/0+$/,'').padEnd(1,'0'); }

function createCryptoWallet({api, accountId, container, onChange = () => {}}) {
  let providers = [], provider = null, address = '', quote = null, epoch = 0, busy = false, timer = null, destroyed = false;
  const account = () => typeof accountId === 'function' ? accountId() : accountId;
  const node = (tag, text, cls) => { const el=document.createElement(tag); if(text !== undefined) el.textContent=text; if(cls) el.className=cls; return el; };
  const message = node('p', '', 'hint'); message.setAttribute('role','status');
  const body = node('div'); const history = node('div'); container.replaceChildren(message, body, history);
  const setMessage = text => { message.textContent=text; };
  const call = async (path, input) => { const response=await api('api/finance/crypto/wallet/'+path,input); const result=await response.json(); if(!response.ok) throw Error(result.error || result.message || result.detail || 'Wallet operation unavailable'); return result; };
  const button = (label, action) => { const el=node('button',label); el.type='button'; el.addEventListener('click',()=>{const current=account(),generation=epoch;action().catch(error=>{if(!destroyed&&current===account()&&generation===epoch)setMessage(error.message)})}); return el; };
  function reset() { epoch++; quote=null; busy=false; clearInterval(timer); timer=null; body.replaceChildren(); history.replaceChildren(); setMessage('Review a current proposal before requesting a wallet transaction.'); }
  function walletChanged() { if(!address&&!quote)return;address=''; reset(); setMessage('Wallet account or network changed. Connect again and review a fresh quote.'); }
  function useProvider(p) { if(provider?.removeListener){provider.removeListener('accountsChanged',walletChanged);provider.removeListener('chainChanged',walletChanged);provider.removeListener('disconnect',walletChanged);} provider=p; provider.on?.('accountsChanged',walletChanged);provider.on?.('chainChanged',walletChanged);provider.on?.('disconnect',walletChanged); }
  function announced(event) { const p=event?.detail?.provider, info=event?.detail?.info; if(p && typeof p.request==='function' && !providers.some(v=>v.provider===p)) providers.push({provider:p,name:String(info?.name||'Browser wallet').slice(0,60)}); }
  window.addEventListener('eip6963:announceProvider',announced); window.dispatchEvent(new Event('eip6963:requestProvider'));
  window.addEventListener('finance-clear',reset);
  async function connect() {
    const generation=epoch, expectedAccount=account();
    if(!provider) {
      if(!providers.length && window.ethereum?.request) providers.push({provider:window.ethereum,name:'Browser wallet'});
      if(!providers.length) { setMessage('Open this app in MetaMask or Rabby’s browser, or install their browser extension. No wallet keys are stored here.'); throw Error('No injected wallet was found'); }
      if(providers.length===1) useProvider(providers[0].provider);
      else {
        body.replaceChildren(node('p','Choose your wallet.'));
        await new Promise((resolve,reject)=>{for(const p of providers)body.append(button(p.name,async()=>{if(generation!==epoch||expectedAccount!==account()){reject(Error('Wallet connection changed'));return}useProvider(p.provider);resolve()}));});
      }
    }
    const accounts=await provider.request({method:'eth_requestAccounts'});
    if(generation!==epoch||expectedAccount!==account()||destroyed) throw Error('Connection changed; try again');
    if(!Array.isArray(accounts)||!cwAddress(accounts[0])) throw Error('Choose a valid wallet account');
    let chain=await provider.request({method:'eth_chainId'});
    if(generation!==epoch||expectedAccount!==account()||destroyed) throw Error('Connection changed; try again');
    if(chain!=='0x2105') { setMessage('Switch to Base in your wallet, then connect again.'); body.replaceChildren(button('Switch wallet to Base',async()=>{await provider.request({method:'wallet_switchEthereumChain',params:[{chainId:'0x2105'}]});setMessage('Base selected. Connect again to continue.')})); throw Error('The wallet must use Base (chain 8453)'); }
    address=accounts[0].toLowerCase();setMessage('Connected on Base: '+address);return address;
  }
  async function currentWallet(expected) { if(!provider) throw Error('Connect your wallet'); const [accounts,chain]=await Promise.all([provider.request({method:'eth_accounts'}),provider.request({method:'eth_chainId'})]);if(chain!=='0x2105'||!Array.isArray(accounts)||accounts[0]?.toLowerCase()!==expected.toLowerCase()) throw Error('Account or chain changed; request a fresh quote'); }
  function renderQuote(q) {
    clearInterval(timer); body.replaceChildren();
    body.append(node('h3','Review this live swap'),node('p',cwFormat(q.sellAmount,q.sellToken)+' '+q.sellToken+' → approximately '+cwFormat(q.buyAmount,q.buyToken)+' '+q.buyToken),node('p','Minimum received: '+cwFormat(q.minBuyAmount,q.buyToken)+' '+q.buyToken),node('p','Base · Uniswap v3 direct 0.05% pool · Slippage '+(q.slippageBps/100)+'%'),node('p','Wallet: '+q.walletAddress),node('p','Router: '+CW_ROUTER),node('p','On-chain deadline: '+new Date(q.deadline*1000).toLocaleTimeString()),node('p','Fresh independent USD prices, ETH reserve and network fees are checked again before signing. The wallet shows the final network fee.','hint'));
    const countdown=node('p'); body.append(countdown);
    const kind=q.resetRequired?'reset':q.approvalRequired?'approve':'swap';
    if(kind==='reset') body.append(node('p','Existing USDC allowance differs from this exact amount. Reset it to zero first. This requires its own wallet approval.'));
    if(kind==='approve') body.append(node('p','Allow only '+cwFormat(q.sellAmount,'USDC')+' USDC for the router. This approval does not perform the swap.'));
    const action=button(kind==='reset'?'Review allowance reset in wallet':kind==='approve'?'Review exact USDC approval in wallet':'Review swap in wallet',()=>send(kind));
    body.append(action,button('Refresh quote',()=>review(q.proposalId)));
    const tick=()=>{const seconds=Math.max(0,Math.ceil((Date.parse(q.expiresAt)-Date.now())/1000));countdown.textContent=seconds?'Quote review expires in '+seconds+' seconds.':'Quote review expired. Refresh before signing.';action.disabled=busy||seconds===0||quote!==q;};tick();timer=setInterval(tick,500);
  }
  async function review(proposalId) {
    reset();const generation=epoch,expectedAccount=account();busy=true;
    try {const wallet=await connect();const q=await call('quote',{proposalId});if(generation!==epoch||expectedAccount!==account())throw Error('Account or proposal changed');if(q.walletAddress?.toLowerCase()!==wallet)throw Error('Connected wallet differs from the configured monitor address');cwTransaction(q,'swap');quote=q;renderQuote(q);setMessage('Inspect the amounts and recipient. Every transaction requires your wallet confirmation.');}finally{if(generation===epoch)busy=false}
  }
  async function send(kind) {
    if(busy||!quote)throw Error('Review a current quote first');const q=quote,generation=epoch,expectedAccount=account();if(Date.now()>=Date.parse(q.expiresAt))throw Error('Quote expired; refresh it');busy=true;
    try {
      await currentWallet(q.walletAddress);const prepared=await call('prepare',{quoteId:q.id,kind});
      if(generation!==epoch||expectedAccount!==account()||quote!==q)throw Error('Review changed');
      cwVerifyTransaction(q,kind,prepared.transaction);await currentWallet(q.walletAddress);
      if(generation!==epoch||expectedAccount!==account()||Date.now()>=Date.parse(q.expiresAt))throw Error('Review expired or changed');
      setMessage('Inspect and confirm this single transaction in your wallet.');
      const hash=await provider.request({method:'eth_sendTransaction',params:[prepared.transaction]});
      if(!/^0x[0-9a-f]{64}$/i.test(hash))throw Error('Wallet returned no valid transaction hash');
      if(expectedAccount!==account()||destroyed)return;
      if(generation!==epoch){await call('receipt',{quoteId:q.id,kind,hash});return;}
      quote=null;clearInterval(timer);body.replaceChildren(node('p','Submitted transaction: '+hash));
      const result=await call('receipt',{quoteId:q.id,kind,hash});if(generation!==epoch||expectedAccount!==account()||destroyed)return;showReceipt(result);await loadHistory();onChange();
    } catch(error) { if(generation===epoch&&expectedAccount===account()&&!destroyed){quote=null;clearInterval(timer);setMessage(error.message+' A wallet transaction is never retried automatically. Check the wallet before requesting another quote.');await loadHistory()}throw error; }
    finally {if(generation===epoch)busy=false}
  }
  function showReceipt(result) { const status=result.status;setMessage(status==='confirmed'?(result.kind==='swap'?'Swap confirmed on a safe Base block.':'Allowance transaction confirmed. Request and review a fresh quote to continue.'):status==='reverted'?'Transaction reverted; gas may still have been charged.':status==='unknown'?'Transaction not yet found. Its outcome is unknown; check again.':'Transaction submitted. Waiting for a safe Base block.'); }
  async function loadHistory() { const expectedAccount=account(),generation=epoch;try {const data=await call('reviews');if(expectedAccount!==account()||generation!==epoch||destroyed)return;history.replaceChildren();if(!data.reviews?.length)return;history.append(node('h3','Recent wallet requests'));for(const item of data.reviews){const row=node('div');row.append(node('p',item.kind+' · '+(item.status||'prepared; outcome not recorded')));const input=node('input');input.type='text';input.placeholder='Transaction hash from your wallet';input.value=item.hash||'';input.setAttribute('aria-label','Transaction hash for '+item.kind);row.append(input,button('Check receipt',async()=>{const result=await call('receipt',{quoteId:item.quoteId,kind:item.kind,hash:input.value.trim()});if(expectedAccount!==account()||generation!==epoch||destroyed)return;showReceipt(result);await loadHistory();onChange()}));history.append(row)}}catch(error){if(expectedAccount===account()&&generation===epoch&&!destroyed)setMessage(error.message)}}
  reset();
  return {connect,review,reset,refresh:loadHistory,destroy(){destroyed=true;reset();window.removeEventListener('finance-clear',reset);window.removeEventListener('eip6963:announceProvider',announced);if(provider?.removeListener){provider.removeListener('accountsChanged',walletChanged);provider.removeListener('chainChanged',walletChanged);provider.removeListener('disconnect',walletChanged)}}};
}
if(typeof window!=='undefined') window.FinanceCryptoWallet={create:createCryptoWallet};
