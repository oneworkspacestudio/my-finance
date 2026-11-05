// Real HTTP + browser checks against a dedicated fictional managed installation.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import { createRequire } from 'node:module';
import { syntheticFinanceLogin } from './synthetic-finance-login.mjs';
if(process.env.FINANCE_SYNTHETIC_ONLY!=='1'||!process.env.FINANCE_TEST_URL)throw Error('Explicit synthetic fixture required');
const base=process.env.FINANCE_TEST_URL;
if(!['localhost','127.0.0.1'].includes(new URL(base).hostname))throw Error('Synthetic loopback fixture required');
const require=createRequire(process.env.FINANCE_MODULE_ROOT+'/package.json'),{chromium}=require('playwright');
const out=process.env.FINANCE_EVIDENCE,profile=process.env.FINANCE_BROWSER_PROFILE;
if(!out||!profile)throw Error('Explicit isolated evidence and browser profile paths required');
await fs.mkdir(out,{recursive:true});
const browser=await chromium.launchPersistentContext(profile,{executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true,viewport:{width:390,height:844}});
const page=await browser.newPage(),errors=[];let remoteRequests=0;
page.on('pageerror',e=>errors.push(e.message));
await browser.route(/https:\/\/(?:api\.(?:openai|deepseek)\.com|(?:live|demo)\.trading212\.com)\//,route=>{remoteRequests++;return route.abort();});
try{
  await syntheticFinanceLogin(browser,page,base);
  const me=await browser.request.get(base+'api/me').then(r=>r.json());
  const headers={Origin:new URL(base).origin,'X-CSRF-Token':me.csrf,'X-Workspace-Expected-Account':me.user.id};
  const endpoint=base+'api/finance/investments/paper';
  const providers=await browser.request.get(base+'api/finance/ai/settings').then(r=>r.json());
  assert.ok(providers.providers.every(p=>!p.configured),'fixture must have no paid provider key');
  const brokerSettings=await browser.request.get(base+'api/finance/investments/broker/settings').then(r=>r.json());
  assert.equal(brokerSettings.configured,false,'fixture must have no broker key');
  await page.goto(base+'#investments');
  await page.locator('#paper-provider-status').filter({hasText:'No saved key'}).waitFor();
  let initial=await browser.request.get(endpoint).then(r=>r.json());
  if(initial.configured){
    await page.locator('#paper-reset-details').evaluate(e=>e.open=true);
    await page.locator('#paper-reset-confirm').check();
    await page.locator('#paper-reset').click();
  }
  const form=page.locator('#paper-setup-form');
  await form.locator('[name=startingCash]').fill('1000');
  await form.locator('[name=goal]').fill('1200');
  await form.locator('[name=maxLoss]').fill('100');
  await form.locator('[name=maxOrder]').fill('100');
  await form.locator('[name=maxPosition]').fill('40');
  await form.locator('[name=maxDecisions]').fill('3');
  await page.locator('#paper-setup-agreement').check();
  await page.locator('#paper-create').click();
  await page.locator('#investment-status').filter({hasText:'Paper portfolio created'}).waitFor();
  const created=await browser.request.get(endpoint).then(r=>r.json());
  assert.equal(created.simulation.cashMinor,100000);
  assert.equal(created.simulation.equityMinor,100000);
  assert.equal(created.simulation.pnlMinor,0);
  assert.equal(created.simulation.config.maxDecisions,3);
  assert.ok(await page.locator('#paper-run-start').isDisabled(),'no API key cannot start a paid run');
  const missingKey=await browser.request.post(endpoint+'/run',{headers,data:{revision:created.simulation.revision,provider:'openai',decisions:1,consent:true}});
  assert.equal(missingKey.status(),400,'server independently rejects run without provider key');
  const unconsented=await browser.request.post(endpoint+'/run',{headers,data:{revision:created.simulation.revision,provider:'openai',decisions:1,consent:false}});
  assert.equal(unconsented.status(),400,'server requires explicit provider consent');
  const unsupported=await browser.request.post(endpoint+'/setup',{headers,data:{revision:created.simulation.revision,confirm:true,config:created.simulation.config,prices:[{instrument:'REAL-STOCK',priceMinor:100}]}});
  assert.equal(unsupported.status(),400,'real ticker cannot become a simulation instrument');
  await page.locator('#paper-prices-details').evaluate(e=>e.open=true);
  const firstPrice=page.locator('#paper-price-inputs input').first();
  const instrument=await firstPrice.getAttribute('data-instrument');
  await firstPrice.fill('123.45');
  await page.locator('#paper-save-prices').click();
  await page.locator('#investment-status').filter({hasText:'Virtual prices updated'}).waitFor();
  const priced=await browser.request.get(endpoint).then(r=>r.json());
  assert.equal(priced.simulation.prices.find(p=>p.instrument===instrument).priceMinor,12345);
  assert.equal(priced.simulation.cashMinor,100000,'scenario prices cannot create cash');
  assert.equal(priced.simulation.attempts,0,'missing-key requests did not consume a model attempt');
  await page.reload();
  await page.locator('#paper-overview').waitFor();
  const restored=await browser.request.get(endpoint).then(r=>r.json());
  assert.deepEqual(restored.simulation,priced.simulation,'paper state persists through browser reload');
  for(const width of [320,360,390,768,1366]){
    await page.setViewportSize({width,height:900});
    for(const tab of ['paper','broker']){
      await page.locator('#investment-'+tab+'-tab').click();
      assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,tab+' fits '+width);
      await page.screenshot({path:out+'/investments-real-'+tab+'-'+width+'.png',fullPage:true});
    }
  }
  const privateSnapshot=await browser.request.post(base+'api/finance/investments/broker/refresh',{headers,data:{consent:true}});
  assert.equal(privateSnapshot.status(),422,'missing broker key fails locally');
  const anonymous=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
  try{const anon=await anonymous.newContext();assert.equal((await anon.request.get(endpoint)).status(),401,'anonymous paper access denied');}finally{await anonymous.close();}
  assert.equal(remoteRequests,0);
  assert.deepEqual(errors,[]);
  await fs.writeFile(out+'/investments-real-report.json',JSON.stringify({synthetic:true,managed:process.env.FINANCE_MANAGED==='1',realHTTP:true,paperSetupViaUI:true,scenarioPricesViaUI:true,persistentReload:true,exactVirtualCashMinor:100000,missingProviderKeyRejected:true,unconsentedRunRejected:true,realTickerRejected:true,missingBrokerKeyRejected:true,anonymousDenied:true,providerRequests:0,mobileOverflow:false,widths:[320,360,390,768,1366],pageErrors:[]},null,2));
  console.log('PASS real managed investment UI: virtual setup/prices/persistence, no-key guards and responsive layouts; no paid/provider calls');
}catch(error){await page.screenshot({path:out+'/investments-real-failure.png',fullPage:true}).catch(()=>{});throw error;}finally{await browser.close();}
