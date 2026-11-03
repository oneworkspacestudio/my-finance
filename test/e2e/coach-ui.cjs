// Synthetic browser acceptance. Never connects to an installed Finance service.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'C:/Users/fahim/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');
const http = require('node:http'), fs = require('node:fs'), path = require('node:path'), assert = require('node:assert/strict');
const root = path.resolve(__dirname, '../../internal/finance/ui');
const evidence = process.env.COACH_EVIDENCE_DIR;
const account = 'a'.repeat(64);
const budgetRef = 'b'.repeat(64);
const now = new Date(), months = [3,2,1].map(offset => { const d = new Date(now.getFullYear(),now.getMonth()-offset,1); return { month:d.getFullYear()+'-'+String(d.getMonth()+1).padStart(2,'0'),incomeMinor:300000,outgoingMinor:180000 }; });
const currentMonth=now.getFullYear()+'-'+String(now.getMonth()+1).padStart(2,'0'),asOf=currentMonth+'-'+String(now.getDate()).padStart(2,'0');
const summary = { source:'actual',currency:'GBP',months,categories:[{name:'Salary',incomeMinor:900000,outgoingMinor:0},{name:'Household',incomeMinor:0,outgoingMinor:540000}],transactionCount:12,excludedTransfers:2,current:{month:currentMonth,asOf,incomeMinor:300000,outgoingMinor:90000,categories:[{name:'Salary',incomeMinor:300000,outgoingMinor:0},{name:'Household',incomeMinor:0,outgoingMinor:90000}]}};
const plan = {currency:'GBP',averageIncomeMinor:300000,averageOutgoingMinor:180000,planningIncomeMinor:300000,spendingReserveMinor:180000,surplusMinor:120000,deficitMinor:0,essentialMonthlyMinor:140000,emergencyTargetMinor:420000,emergencyGapMinor:320000,buckets:[{name:'Existing spending',monthlyMinor:180000,purpose:'Reserve for observed spending.'},{name:'Emergency buffer',monthlyMinor:120000,purpose:'Build accessible savings.'}],goals:[{name:'Holiday',remainingMinor:100000,requiredMonthlyMinor:8334,allocatedMonthlyMinor:0,monthsToGoal:null}],investmentReady:false,investmentReason:'Build an emergency buffer before considering long-term investing.',warnings:['Check that these months include all regular income and bills.']};
const answer={plan,provider:'openai',model:'gpt-4.1-mini',advice:{summary:'<img src=x onerror="window.injected=true"> A fictional review of your money.',observations:['Income is above recorded spending.'],actions:['Check recurring bills.'],buckets:['Create an emergency savings category.'],savings:'Review the holiday target after the emergency buffer.',investing:'Understand risk, fees and diversification before investing.',questions:['Are all regular bills included?']}};
plan.manager={planningIncomeMinor:300000,categoryAllocatedMinor:180000,essentialRequiredMinor:180000,essentialAllocatedMinor:180000,essentialShortfallMinor:0,unassignedEssentialsMinor:0,unassignedEssentialsAllocatedMinor:0,debtReservedMinor:0,emergencyReservedMinor:0,savingsWantedMinor:60000,savingsReservedMinor:60000,goalReservedMinor:0,investmentWantedMinor:20000,investmentReservedMinor:20000,cashBufferWantedMinor:40000,cashBufferReservedMinor:40000,unallocatedMinor:0,unfundedTargetsMinor:0,spentMinor:90000,spendingFundingGapMinor:0,currentMonth,asOf,currentAvailable:true,investmentEligible:true,categories:[{name:'Household',essential:true,historicalMonthlyMinor:180000,wantedMinor:180000,limitMinor:180000,spentMinor:90000,remainingMinor:90000,overspentMinor:0,reductionMinor:0}],goals:[],warnings:['Review current spending after every budget refresh.']};
answer.advice.managerSuggestion={categories:[{name:'Household',essential:true,targetMonthlyMinor:175000}],savingsMonthlyMinor:65000,investmentMonthlyMinor:20000,cashBufferMinor:40000,reason:'Try a slightly lower monthly household cap.'};
let configured=false, swapped=false, calls=0, sent, providerStatus=200, providerDelay=0, meDelay=0, settingsWrites=[];
const rules=new Map();let rulesWrites=[],lastPlanRequest,rulesReadError=false;
function fixturePlan(body){
  const result=structuredClone(plan),manager=result.manager,rule=body.profile.manager,categories=rule.categories;
  if(body.summary.categories.some(category=>category.name==='Rent')) {
    manager.categoryAllocatedMinor=175000;manager.essentialRequiredMinor=120000;manager.essentialAllocatedMinor=120000;manager.savingsWantedMinor=65000;manager.savingsReservedMinor=65000;manager.investmentWantedMinor=20000;manager.investmentReservedMinor=20000;manager.cashBufferWantedMinor=40000;manager.cashBufferReservedMinor=40000;manager.spentMinor=145000;manager.unfundedTargetsMinor=13000;
    manager.categories=[
      {name:'Rent',essential:true,historicalMonthlyMinor:100000,wantedMinor:100000,limitMinor:100000,spentMinor:100000,remainingMinor:0,overspentMinor:0,reductionMinor:0},
      {name:'Groceries',essential:false,historicalMonthlyMinor:40000,wantedMinor:50000,limitMinor:37000,spentMinor:22000,remainingMinor:15000,overspentMinor:0,reductionMinor:13000},
      {name:'Transport',essential:true,historicalMonthlyMinor:20000,wantedMinor:20000,limitMinor:20000,spentMinor:5000,remainingMinor:15000,overspentMinor:0,reductionMinor:0},
      {name:'Leisure',essential:false,historicalMonthlyMinor:20000,wantedMinor:15000,limitMinor:18000,spentMinor:18000,remainingMinor:0,overspentMinor:3000,reductionMinor:0}
    ];
    assert.equal(manager.categoryAllocatedMinor+manager.savingsReservedMinor+manager.investmentReservedMinor+manager.cashBufferReservedMinor,manager.planningIncomeMinor);
    assert.equal(manager.categories.reduce((n,c)=>n+c.limitMinor,0),manager.categoryAllocatedMinor);
    assert.equal(manager.categories.reduce((n,c)=>n+c.spentMinor,0),manager.spentMinor);
    return result;
  }
  const household=categories.find(category=>category.name==='Household'),limit=household?.targetMonthlyMinor??180000;
  manager.categoryAllocatedMinor=limit;manager.categories[0].limitMinor=limit;manager.categories[0].wantedMinor=limit;manager.categories[0].remainingMinor=Math.max(0,limit-manager.categories[0].spentMinor);
  manager.savingsWantedMinor=rule.savingsMonthlyMinor;manager.savingsReservedMinor=rule.savingsMonthlyMinor;manager.investmentWantedMinor=rule.investmentMonthlyMinor;manager.investmentReservedMinor=rule.investmentMonthlyMinor;manager.cashBufferWantedMinor=rule.cashBufferMinor;manager.cashBufferReservedMinor=rule.cashBufferMinor;
  manager.unallocatedMinor=Math.max(0,300000-limit-rule.savingsMonthlyMinor-rule.investmentMonthlyMinor-rule.cashBufferMinor);manager.currentAvailable=!!body.summary.current;manager.investmentEligible=body.profile.highCostDebt==='no'&&body.profile.emergencySavingsMinor>=420000&&manager.currentAvailable;
  return result;
}
const server = http.createServer((req,res)=>{
  const pathname = new URL(req.url,'http://localhost').pathname;
  if (pathname.startsWith('/api/')) {
    if (swapped && req.headers['x-workspace-expected-account']) {res.writeHead(409,{'Content-Type':'application/json'});res.end(JSON.stringify({error:'Account changed'}));return;}
    let text='';req.on('data',b=>text+=b);req.on('end',async()=>{
      let result={};const body=text?JSON.parse(text):{};
      if(pathname==='/api/bootstrap')result={oidc:true,workspace:'/'};
      else if(pathname==='/api/me'){if(meDelay)await new Promise(r=>setTimeout(r,meDelay));result={user:{id:account,role:'owner'},csrf:'fictional'};}
      else if(pathname==='/api/finance/status')result={engineReady:true,actual:'26.10.0',headlessSync:'Use native Sync.',driveConfigured:false};
      else if(pathname==='/api/finance/connections')result=[];
      else if(pathname==='/api/finance/snapshots/settings')result={daily:7,weekly:4,monthly:6};
      else if(pathname==='/api/finance/ai/settings'){
        if(req.method==='POST'){assert.equal(req.headers['x-csrf-token'],'fictional');assert.equal(req.headers['x-workspace-expected-account'],account);settingsWrites.push(body);configured=!body.remove;}
        result={providers:[{id:'openai',model:'gpt-4.1-mini',configured},{id:'deepseek',model:'deepseek-flash',configured:false}]};
      } else if(pathname==='/api/finance/ai/manager-settings') {
        const scope=new URL(req.url,'http://localhost').searchParams.get('scope');
        if(req.method==='POST'){rulesWrites.push(body);if(body.remove)rules.delete(body.scope);else rules.set(body.scope,body.profile);result={saved:true};}
        else if(rulesReadError){res.writeHead(422,{'Content-Type':'application/json'});res.end(JSON.stringify({error:'Saved rules cannot be decrypted.'}));return;}
        else result=rules.has(scope)?{configured:true,profile:rules.get(scope)}:{configured:false};
      } else if(pathname==='/api/finance/ai/plan') { assert.equal(body.summary.currency,'GBP');assert.equal(body.profile.manager.enabled,true);lastPlanRequest=body;result=fixturePlan(body); }
      else if(pathname==='/api/finance/ai/analyse') {assert.equal(body.consent,true);assert.equal(req.headers['x-workspace-expected-account'],account);calls++;sent=body;if(providerDelay)await new Promise(r=>setTimeout(r,providerDelay));if(providerStatus!==200){res.writeHead(providerStatus,{'Content-Type':'application/json'});res.end(JSON.stringify({error:'The provider is busy. Please try again later.'}));return;}result={...answer,plan:fixturePlan(body)};}
      res.writeHead(200,{'Content-Type':'application/json','Cache-Control':'no-store'});res.end(JSON.stringify(result));
    });return;
  }
  const relative=pathname==='/'?'index.html':pathname.slice(1), target=path.join(root,relative);
  if(!target.startsWith(root+path.sep)||!fs.existsSync(target)){res.writeHead(404);res.end();return;}
  res.writeHead(200,{'Content-Type':target.endsWith('.js')?'text/javascript':target.endsWith('.css')?'text/css':target.endsWith('.svg')?'image/svg+xml':'text/html'});res.end(fs.readFileSync(target));
});
(async()=>{
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
  const url='http://127.0.0.1:'+server.address().port;
  const browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_PATH||'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  try{
    const page=await browser.newPage({viewport:{width:390,height:844}}),errors=[];page.on('pageerror',e=>{errors.push(e.message);console.error(e.message);});
    await page.goto(url+'/#coach');await page.locator('#content').waitFor({state:'visible'});
    assert.equal(await page.locator('#coach-analyse').isDisabled(),true);assert.equal(calls,0);
    await page.evaluate(({account,summary,budgetRef})=>sessionStorage.setItem('myfinance.coach.summary',JSON.stringify({nonce:crypto.randomUUID(),account,created:Date.now(),summary,budgetRef})),{account,summary,budgetRef});
    await page.reload();await page.locator('#manager-dashboard').waitFor({state:'visible'});
    assert.equal(await page.evaluate(()=>sessionStorage.getItem('myfinance.coach.summary')),null);
    await page.locator('#coach-profile-form [name=managerSavings]').fill('600');await page.locator('#coach-profile-form [name=managerInvestment]').fill('200');await page.locator('#coach-profile-form [name=managerBuffer]').fill('400');
    await page.locator('#manager-background summary').click();await page.locator('#coach-profile-form [name=essential]').fill('1400');await page.locator('#coach-profile-form [name=emergency]').fill('6000');
    await page.locator('[name=debt]').selectOption('no');await page.locator('[name=horizon]').selectOption('6');await page.locator('[name=risk]').selectOption('balanced');
    await page.locator('#manager-goals-details summary').click();await page.locator('#coach-add-goal').click();
    await page.locator('[data-goal=name]').fill('Holiday');await page.locator('[data-goal=target]').fill('1200');await page.locator('[data-goal=saved]').fill('200');
    await page.locator('#manager-dashboard').waitFor({state:'visible'});assert.equal(calls,0);assert.equal(lastPlanRequest.profile.manager.savingsMonthlyMinor,60000);
    assert.match(await page.locator('#manager-metrics').textContent(),/Allocated to spending£1,800.00/);assert.match(await page.locator('#manager-metrics').textContent(),/Reserved for saving£600.00/);assert.match(await page.locator('#manager-category-plan').textContent(),/Spent£900.00Left£900.00/);
    const download=page.waitForEvent('download');await page.locator('#manager-download').click();assert.equal((await download).suggestedFilename(),'MyFinance-money-plan.json');
    // Browsers accept exponent notation for number inputs; money rules require plain decimals.
    const writesBeforeExponent=rulesWrites.length;
    await page.locator('[name=managerSavings]').fill('1e3');
    assert.equal(await page.locator('[name=managerSavings]').evaluate(e=>e.checkValidity()),true);
    await page.locator('#manager-save-rules').click();
    await page.locator('#manager-rules-status').filter({hasText:'Enter money as a positive amount with up to two decimal places.'}).waitFor();
    assert.equal(rulesWrites.length,writesBeforeExponent);assert.deepEqual(errors,[]);
    await page.locator('[name=managerSavings]').fill('600');await page.locator('#manager-dashboard').waitFor({state:'visible'});
    await page.locator('#manager-save-rules').click();await page.locator('#manager-rules-status').filter({hasText:'Rules saved'}).waitFor();assert.equal(rulesWrites.at(-1).scope,budgetRef);assert.equal(rules.get(budgetRef).manager.savingsMonthlyMinor,60000);
    await page.evaluate(({account,summary,budgetRef})=>sessionStorage.setItem('myfinance.coach.summary',JSON.stringify({nonce:crypto.randomUUID(),account,created:Date.now(),summary,budgetRef})),{account,summary,budgetRef});await page.reload();await page.locator('#manager-dashboard').waitFor({state:'visible'});assert.equal(await page.locator('[name=managerSavings]').inputValue(),'600.00');assert.equal(await page.locator('[data-goal=name]').inputValue(),'Holiday');
    await page.locator('#manager-ai-details > summary').click();
    await page.locator('#coach-provider-details summary').click();await page.locator('#coach-key').fill('fictional-key-only');
    // Settings capture the reviewed provider/model/key before checking identity.
    meDelay=250;const saveCheck=page.waitForResponse(r=>r.url().endsWith('/api/me'));
    await page.locator('#coach-settings-form button[type=submit]').click();
    for(const selector of ['#coach-provider','#coach-model','#coach-key'])assert.equal(await page.locator(selector).isDisabled(),true);
    await page.evaluate(()=>{document.querySelector('#coach-provider').value='deepseek';document.querySelector('#coach-model').value='changed-model';document.querySelector('#coach-key').value='changed-fictional-key';});
    await saveCheck;await page.locator('#coach-status').filter({hasText:'Provider settings saved for OpenAI'}).waitFor();
    assert.deepEqual(settingsWrites.at(-1),{provider:'openai',model:'gpt-4.1-mini',apiKey:'fictional-key-only',remove:false});meDelay=0;
    await page.locator('#coach-provider-details summary').click();await page.locator('#coach-provider').selectOption('openai');await page.locator('#coach-provider-details summary').click();
    assert.equal(await page.locator('#coach-key').inputValue(),'');assert.equal(await page.locator('#coach-analyse').isDisabled(),true);
    await page.locator('#coach-consent').check();assert.equal(await page.locator('#coach-analyse').isEnabled(),true);
    await page.locator('#coach-question').fill('How could I save?');assert.equal(await page.locator('#coach-consent').isChecked(),false);
    await page.locator('#coach-consent').check();await page.locator('#coach-analyse').click();await page.locator('#coach-advice').waitFor({state:'visible'});
    assert.equal(calls,1);assert.equal(sent.summary.transactionCount,12);assert.equal(sent.profile.goals[0].name,'Holiday');assert.equal(await page.evaluate(()=>window.injected),undefined);assert.equal(await page.locator('#coach-consent').isChecked(),false);
    assert.equal(await page.locator('[name=managerSavings]').inputValue(),'600.00');assert.equal(await page.locator('#manager-proposal').isVisible(),true);
    const savedBeforeProposal=rulesWrites.length;await page.locator('#manager-apply-proposal').click();await page.locator('#manager-dashboard').waitFor({state:'visible'});assert.equal(await page.locator('[name=managerSavings]').inputValue(),'650.00');assert.equal(lastPlanRequest.profile.manager.savingsMonthlyMinor,65000);assert.equal(lastPlanRequest.profile.manager.categories[0].targetMonthlyMinor,175000);assert.equal(rulesWrites.length,savedBeforeProposal,'Applying suggestions must not silently persist rules');
    // A provider error never destroys the local plan or leaves an old answer visible.
    providerStatus=429;await page.locator('#coach-consent').check();await page.locator('#coach-analyse').click();await page.locator('#coach-status').filter({hasText:'The provider is busy'}).waitFor();assert.equal(await page.locator('#manager-dashboard').isVisible(),true);assert.equal(await page.locator('#coach-advice').isHidden(),true);assert.equal(await page.locator('#coach-consent').isChecked(),false);providerStatus=200;
    // Editing while fresh identity is being checked cancels before any provider call.
    const before=calls;meDelay=250;const identityCheck=page.waitForResponse(r=>r.url().endsWith('/api/me'));
    await page.locator('#coach-consent').check();await page.locator('#coach-analyse').click();await page.locator('#coach-question').fill('A changed question');await identityCheck;await page.waitForFunction(()=>document.querySelector('#coach-analyse').textContent==='Get AI guidance');assert.equal(calls,before);meDelay=0;
    // Editing while the provider responds discards that now-stale answer.
    providerDelay=250;const inflight=page.waitForResponse(r=>r.url().endsWith('/api/finance/ai/analyse')),requested=page.waitForRequest(r=>r.url().endsWith('/api/finance/ai/analyse'));
    await page.locator('#coach-consent').check();await page.locator('#coach-analyse').click();await requested;await page.locator('#coach-question').fill('Help me make a savings plan.');await inflight;await page.waitForFunction(()=>document.querySelector('#coach-analyse').textContent==='Get AI guidance');assert.equal(await page.locator('#coach-advice').isHidden(),true);providerDelay=0;
    answer.advice.summary='Your recorded income is above spending. Build a cash buffer first, then revisit your holiday goal.';
    await page.locator('#coach-consent').check();await page.locator('#coach-analyse').click();await page.locator('#coach-advice').waitFor({state:'visible'});
    await page.locator('#coach-preview summary').click();
    assert.equal(JSON.parse(await page.locator('#coach-payload').textContent()).plan.spendingReserveMinor,180000);
    for(const width of [320,390,768,1280]){
      await page.setViewportSize({width,height:850});
      await page.evaluate(()=>window.scrollTo({top:0,behavior:'instant'}));await page.waitForTimeout(250);
      assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,'horizontal overflow at '+width);
      if(evidence){fs.mkdirSync(evidence,{recursive:true});await page.screenshot({path:path.join(evidence,`${width}-coach.png`),fullPage:true});}
    }
    if(evidence){await page.setViewportSize({width:390,height:844});await page.evaluate(()=>window.scrollTo({top:0,behavior:'instant'}));await page.waitForTimeout(100);await page.screenshot({path:path.join(evidence,'390-coach-top.png')});await page.locator('#manager-dashboard').evaluate(e=>{window.scrollTo({top:e.getBoundingClientRect().top+window.scrollY-90,behavior:'instant'});});await page.waitForTimeout(100);await page.screenshot({path:path.join(evidence,'390-coach-plan.png')});}
    await page.locator('#coach-provider-details summary').click();await page.locator('#coach-provider').selectOption('deepseek');assert.equal(await page.locator('#coach-model').inputValue(),'deepseek-flash');assert.equal(await page.locator('#coach-analyse').isDisabled(),true);
    await page.locator('#coach-provider').selectOption('openai');
    // A changed selection during the identity check cancels removal entirely.
    const beforeRemoval=settingsWrites.length;meDelay=250;const removeCheck=page.waitForResponse(r=>r.url().endsWith('/api/me'));
    await page.locator('#coach-remove-key').click();assert.equal(await page.locator('#coach-provider').isDisabled(),true);
    await page.evaluate(()=>{const provider=document.querySelector('#coach-provider');provider.value='deepseek';provider.dispatchEvent(new Event('change'));});
    await removeCheck;await page.waitForFunction(()=>!document.querySelector('#coach-provider').disabled);assert.equal(settingsWrites.length,beforeRemoval);meDelay=0;
    await page.locator('#coach-provider').selectOption('openai');await page.locator('#coach-remove-key').click();await page.locator('#coach-provider-badge').filter({hasText:'No key saved'}).waitFor();assert.equal(settingsWrites.at(-1).provider,'openai');assert.equal(settingsWrites.at(-1).remove,true);assert.equal(await page.locator('#coach-analyse').isDisabled(),true);
    await page.locator('#manager-source-details > summary').click();await page.locator('#coach-period').selectOption('6');assert.equal(await page.locator('#coach-data').isHidden(),true);assert.equal(await page.locator('#manager-dashboard').isHidden(),true);
    await page.locator('#coach-manual summary').click();
    for(const input of await page.locator('#coach-manual-months [data-field=income]').all())await input.fill('3000');
    for(const input of await page.locator('#coach-manual-months [data-field=outgoing]').all())await input.fill('1800');
    await page.locator('#coach-manual-form button').click();assert.match(await page.locator('#coach-source').textContent(),/Manual estimates/);
    assert.equal(JSON.parse(await page.locator('#coach-payload').textContent()).summary.source,'manual');
    await page.locator('#coach-plan-button').click();await page.locator('#manager-dashboard').waitFor({state:'visible'});
    await page.locator('#manager-source-details > summary').click();await page.locator('#coach-manual summary').click();await page.locator('#coach-manual-months [data-field=income]').first().fill('3100');assert.equal(await page.locator('#manager-dashboard').isHidden(),true);assert.equal(await page.locator('#coach-data').isHidden(),true);
    await page.locator('#coach-manual-form button').click();
    await page.locator('#manager-dashboard').waitFor({state:'visible'});await page.locator('#manager-save-rules').click();await page.locator('#manager-rules-status').filter({hasText:'Rules saved'}).waitFor();assert.equal(rulesWrites.at(-1).scope,'manual');assert.equal(rules.get(budgetRef).manager.savingsMonthlyMinor,60000,'Manual settings must not overwrite the native budget rules');
    // Unreadable saved rules remain removable, and loading another budget is isolated.
    rulesReadError=true;const otherBudget='c'.repeat(64);
    await page.evaluate(({account,summary,budgetRef})=>sessionStorage.setItem('myfinance.coach.summary',JSON.stringify({nonce:crypto.randomUUID(),account,created:Date.now(),summary,budgetRef})),{account,summary,budgetRef:otherBudget});await page.reload();await page.locator('#manager-rules-status').filter({hasText:'cannot be decrypted'}).waitFor();assert.equal(await page.locator('#manager-remove-rules').isEnabled(),true);assert.equal(await page.locator('[name=managerSavings]').inputValue(),'0');await page.locator('#manager-remove-rules').click();await page.locator('#manager-rules-status').filter({hasText:'Saved rules removed'}).waitFor();assert.equal(rulesWrites.at(-1).scope,otherBudget);rulesReadError=false;
    await page.locator('#manager-dashboard').waitFor({state:'visible'});
    swapped=true;await page.locator('#coach-plan-button').click();await page.locator('#content').waitFor({state:'hidden'});assert.equal(await page.locator('#coach-payload').textContent(),'Choose a budget or enter totals, then calculate your plan.');assert.equal(await page.locator('#coach-key').inputValue(),'');
    assert.equal(await page.locator('#coach-plan-basis').textContent(),'');assert.equal(await page.locator('#coach-emergency').textContent(),'');
    if(evidence){
      swapped=false;const showcaseRef='d'.repeat(64),showcaseSummary=structuredClone(summary);
      showcaseSummary.categories=[{name:'Salary',incomeMinor:900000,outgoingMinor:0},{name:'Rent',incomeMinor:0,outgoingMinor:300000},{name:'Groceries',incomeMinor:0,outgoingMinor:120000},{name:'Transport',incomeMinor:0,outgoingMinor:60000},{name:'Leisure',incomeMinor:0,outgoingMinor:60000}];
      showcaseSummary.current.outgoingMinor=145000;showcaseSummary.current.categories=[{name:'Salary',incomeMinor:300000,outgoingMinor:0},{name:'Rent',incomeMinor:0,outgoingMinor:100000},{name:'Groceries',incomeMinor:0,outgoingMinor:22000},{name:'Transport',incomeMinor:0,outgoingMinor:5000},{name:'Leisure',incomeMinor:0,outgoingMinor:18000}];
      rules.set(showcaseRef,{essentialMonthlyMinor:120000,emergencySavingsMinor:600000,highCostDebt:'no',horizonYears:6,risk:'balanced',goals:[],manager:{enabled:true,savingsMonthlyMinor:65000,investmentMonthlyMinor:20000,cashBufferMinor:40000,categories:[{name:'Rent',essential:true,targetMonthlyMinor:100000},{name:'Groceries',essential:false,targetMonthlyMinor:50000},{name:'Transport',essential:true,targetMonthlyMinor:20000},{name:'Leisure',essential:false,targetMonthlyMinor:15000}]}});
      const showcase=await browser.newPage({viewport:{width:390,height:844}});showcase.on('pageerror',e=>errors.push(e.message));await showcase.goto(url+'/#coach');await showcase.locator('#content').waitFor({state:'visible'});
      await showcase.evaluate(({account,summary,budgetRef})=>sessionStorage.setItem('myfinance.coach.summary',JSON.stringify({nonce:crypto.randomUUID(),account,created:Date.now(),summary,budgetRef})),{account,summary:showcaseSummary,budgetRef:showcaseRef});await showcase.reload();await showcase.locator('#manager-dashboard').waitFor({state:'visible'});await showcase.locator('#coach-profile-card').evaluate(e=>e.open=false);
      for(const width of [320,390,768,1280]){
        await showcase.setViewportSize({width,height:850});await showcase.evaluate(()=>window.scrollTo({top:0,behavior:'instant'}));await showcase.waitForTimeout(250);
        assert.equal(await showcase.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,'manager overflow '+width);
        if(width<=850)assert.equal(await showcase.locator('aside').evaluate(e=>getComputedStyle(e).visibility==='hidden'&&e.inert&&e.getBoundingClientRect().right<=1),true,'drawer must be fully closed before capture');
        await showcase.screenshot({path:path.join(evidence,`${width}-coach.png`),fullPage:true});
      }
      await showcase.setViewportSize({width:390,height:844});await showcase.evaluate(()=>window.scrollTo({top:0,behavior:'instant'}));await showcase.waitForTimeout(250);await showcase.screenshot({path:path.join(evidence,'390-coach-top.png')});
      await showcase.locator('#manager-dashboard').evaluate(e=>window.scrollTo({top:e.getBoundingClientRect().top+scrollY-90,behavior:'instant'}));await showcase.waitForTimeout(250);await showcase.screenshot({path:path.join(evidence,'390-coach-plan.png')});
      await showcase.setViewportSize({width:390,height:1100});await showcase.locator('#manager-category-plan').evaluate(e=>window.scrollTo({top:e.getBoundingClientRect().top+scrollY-110,behavior:'instant'}));await showcase.waitForTimeout(250);await showcase.screenshot({path:path.join(evidence,'390-manager-categories.png')});await showcase.close();
    }
    assert.deepEqual(errors,[]);console.log('PASS: account-bound handoff consumed, local plan, provider save/remove capture and cancellation races, provider failure, consent reset, stale/in-flight edits rejected, safe model text, 4 responsive widths, manual/period changes invalidate old data, account switch clears all data.');
  }finally{await browser.close();server.close();}
})().catch(e=>{console.error(e);server.close();process.exitCode=1;});
