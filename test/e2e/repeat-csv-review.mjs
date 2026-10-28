if(process.env.FINANCE_SYNTHETIC_ONLY!=='1')throw Error('Explicit disposable fictional fixture acknowledgement required');
import {createRequire} from 'node:module';
import fs from 'node:fs/promises';
const require=createRequire(process.env.FINANCE_MODULE_ROOT+'/package.json');
const {chromium}=require('@playwright/test');
const base=process.env.FINANCE_TEST_URL||'http://localhost:25016/finance/';
const out=process.env.FINANCE_EVIDENCE||'C:/Projects/.cache/my-finance-20261028/evidence';
await fs.mkdir(out,{recursive:true});
const context=await chromium.launchPersistentContext(out+'/synthetic-browser',{executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true,viewport:{width:1440,height:960}});const page=await context.newPage();
const failures=[],consoleLines=[];page.on('console',m=>consoleLines.push(m.type()+': '+m.text()));page.on('pageerror',e=>failures.push({kind:'page',message:e.message}));page.on('response',r=>{if(r.status()>=400) failures.push({kind:'http',status:r.status(),path:new URL(r.url()).pathname})});
try{
 for(let i=0;i<90;i++){try{if((await context.request.get(new URL('/healthz',base).href)).ok())break}catch{}if(i===89)throw Error('Launcher readiness timeout');await new Promise(r=>setTimeout(r,1000))}
 const anonymous=await fetch(base+'ledger/account/validate');if(anonymous.status!==401)throw Error('Anonymous engine access was not denied');
 const login=await context.request.post(base+'api/login',{data:{email:'owner@example.test',password:'Fictional-Test-Only-582!'},headers:{Origin:new URL(base).origin}});if(!login.ok())throw Error('Synthetic owner login failed: '+login.status());
 for(let i=0;i<90;i++){const s=await context.request.get(base+'api/finance/status').then(r=>r.json());if(s.engineReady)break;if(i===89)throw Error('Engine readiness timeout');await new Promise(r=>setTimeout(r,1000))}
 await page.goto(base);await page.getByRole('heading',{name:/A little clarity/}).waitFor();
 await page.screenshot({path:out+'/welcome-desktop.png'});
 await page.setViewportSize({width:360,height:800});if(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth))throw Error('Welcome phone overflow');await page.screenshot({path:out+'/welcome-phone.png'});
 await page.setViewportSize({width:1440,height:960});await page.getByRole('link',{name:'Open my budgets →',exact:true}).click();
 await page.waitForFunction(()=>{const s=document.body.innerText;return s.length>15&&!s.includes('Initializing the connection')&&!s.includes('Loading the app')},{},{timeout:90000}).catch(()=>{});
 await page.waitForTimeout(3000);
 consoleLines.push('Inputs: '+JSON.stringify(await page.locator('input').evaluateAll(v=>v.map(e=>({type:e.type,placeholder:e.placeholder})))));
 consoleLines.push('Native bootstrap: '+JSON.stringify(await context.request.get(base+'ledger/account/needs-bootstrap').then(r=>r.json())));
 if(await page.getByRole('button',{name:'Sign in',exact:true}).isVisible().catch(()=>false)){await page.getByRole('button',{name:'Sign in',exact:true}).click();await page.waitForTimeout(5000)}
 if(await page.getByRole('button',{name:'Start budgeting',exact:true}).isVisible().catch(()=>false)){await page.getByRole('button',{name:'Start budgeting',exact:true}).click();await page.waitForTimeout(3000)}
 if(!await page.getByRole('button',{name:'Add account',exact:true}).isVisible().catch(()=>false)){const budget=page.getByText('My Finances',{exact:true}).first();if(await budget.isVisible().catch(()=>false)){await budget.click();await page.waitForTimeout(3000)}}
 const cash=page.getByText('Fictional cash',{exact:true}).first();if(await cash.isVisible().catch(()=>false)){await cash.click();await page.waitForTimeout(1000)}else{
  if(await page.getByRole('button',{name:'Add account',exact:true}).isVisible().catch(()=>false)){await page.getByRole('button',{name:'Add account',exact:true}).click();await page.waitForTimeout(1000)}
  if(await page.getByRole('button',{name:'Create a local account',exact:true}).isVisible().catch(()=>false)){await page.getByRole('button',{name:'Create a local account',exact:true}).click();await page.waitForTimeout(1000)}
  if(await page.getByRole('button',{name:'Create',exact:true}).isVisible().catch(()=>false)){const fields=page.getByRole('textbox');await fields.nth(0).fill('Fictional cash');await fields.nth(1).fill('1234.56');await page.getByRole('button',{name:'Create',exact:true}).click();await page.waitForTimeout(1500)}
 }
 await page.getByText('Starting Balance',{exact:true}).waitFor({timeout:30000});
 if(process.env.FINANCE_TEST_CSV==='1'){
  const chooser=page.waitForEvent('filechooser');await page.getByRole('button',{name:'Import',exact:true}).click();await(await chooser).setFiles({name:'fictional-statement.csv',mimeType:'text/csv',buffer:Buffer.from('Date,Payee,Amount,Notes\n2026-10-20,Fictional CSV Grocer,-12.34,Fictional CSV import\n2026-10-21,Fictional CSV Refund,2.50,Fictional CSV refund\n')});await page.waitForTimeout(1500);const apply=page.getByRole('button',{name:'Import 0 transactions',exact:true});await apply.waitFor();if(await page.locator('#start-date-filter').inputValue()!=='')throw Error('Date filtering must not explain skipped rows');await page.screenshot({path:out+'/duplicate-review.png'});consoleLines.push('PASS repeated CSV review reports zero new transactions with no date filter');await page.keyboard.press('Escape');await page.getByText('Fictional Csv Grocer',{exact:true}).waitFor();
 }
 await page.screenshot({path:out+'/native-current.png'});
 for(const width of [360,768]){await page.setViewportSize({width,height:900});await page.waitForTimeout(1000);if(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth))throw Error('Native overflow at '+width);await page.screenshot({path:out+'/native-'+width+'.png'});}consoleLines.push('PASS native 360px/768px no document overflow');
 await fs.writeFile(out+'/browser-current.json',JSON.stringify({url:page.url(),text:await page.locator('body').innerText(),failures},null,2));
 console.log(JSON.stringify({url:page.url(),text:(await page.locator('body').innerText()).slice(0,5000),failures,consoleLines:consoleLines.slice(-30)},null,2));
}catch(e){await page.screenshot({path:out+'/failure.png'}).catch(()=>{});await fs.writeFile(out+'/failure.json',JSON.stringify({message:e.message,text:await page.locator('body').innerText(),failures},null,2));throw e}finally{await context.close()}
