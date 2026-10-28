import {test} from 'node:test';import assert from 'node:assert/strict';import {spawn} from 'node:child_process';import {createRequire} from 'node:module';import {generateKeyPairSync,randomBytes} from 'node:crypto';import fs from 'node:fs/promises';import path from 'node:path';import {pathToFileURL,fileURLToPath} from 'node:url';
test('MOCK: native Enable Banking directory, AIS consent, session, two pages, exact balance and actor boundary',async()=>{
 const root=process.env.FINANCE_MODULE_ROOT;assert.ok(root,'Set FINANCE_MODULE_ROOT to the pinned npm installation');
 const tmp=await fs.mkdtemp(path.join(root,'provider-mock-'));const cap=randomBytes(48).toString('hex');const owner='a'.repeat(64),member='b'.repeat(64);const port=25176;
 const child=spawn(process.execPath,[fileURLToPath(new URL('./fixtures/mock-engine.mjs',import.meta.url))],{cwd:root,env:{SystemRoot:process.env.SystemRoot,TEMP:tmp,NODE_ENV:'production',ACTUAL_DATA_DIR:tmp,ACTUAL_HOSTNAME:'127.0.0.1',ACTUAL_PORT:String(port),MYFINANCE_ENGINE_CAPABILITY:cap,FINANCE_ACTUAL_ENTRY:pathToFileURL(path.join(root,'node_modules/@actual-app/sync-server/build/app.js')).href},stdio:['ignore','pipe','pipe'],windowsHide:true});
 let startup='';child.stdout.on('data',b=>{startup=(startup+b).slice(-6000)});child.stderr.on('data',b=>{startup=(startup+b).slice(-6000)});
 const call=async(route,body,actor=owner)=>{const r=await fetch('http://127.0.0.1:'+port+route,{method:body?'POST':'GET',headers:{'Content-Type':'application/json','X-Finance-Capability':cap,'X-Finance-Identity':actor,'X-Finance-Owner':actor===owner?'1':'0'},...(body?{body:JSON.stringify(body)}:{})});return {status:r.status,data:await r.json().catch(()=>null)}};
 try{
  let ready=false;for(let i=0;i<300;i++){try{ready=(await call('/health')).status===200;if(ready)break}catch{}if(child.exitCode!==null)break;await new Promise(r=>setTimeout(r,150))}assert.ok(ready,'Native engine launch: '+startup);
  assert.equal((await fetch('http://127.0.0.1:'+port+'/health')).status,403,'No capability bypass');
  const {privateKey}=generateKeyPairSync('rsa',{modulusLength:2048});const pem=privateKey.export({type:'pkcs8',format:'pem'});
  assert.equal((await call('/enablebanking/configure',{applicationId:'fictional-application',secretKey:pem})).data.data.configured,true);
  assert.equal((await call('/enablebanking/aspsps',{country:'GB'})).data.data.aspsps[0].country,'GB');
  const auth=await call('/enablebanking/start-auth',{aspsp:{name:'Fictional Test Bank',country:'GB'},redirectUrl:'https://workspace.example.test/finance/ledger/enablebanking/auth_callback'});assert.ok(auth.data.data.state);assert.match(auth.data.data.url,/provider.example.test/);
  const session=await call('/enablebanking/complete-auth',{code:'fictional-provider-code',state:auth.data.data.state});assert.equal(session.data.data.accounts[0].balance,123456);
  const tx=await call('/enablebanking/transactions',{accountId:'fictional-account',startDate:'2026-10-01'});assert.equal(tx.data.data.transactions.all.length,100);assert.equal(tx.data.data.transactions.booked[0].transactionAmount.amount,'-1.01');assert.equal(tx.data.data.startingBalance,123456);
  assert.equal((await call('/enablebanking/aspsps',{country:'GB'},member)).status,403,'Member denied banking');
  const permission=await call('/enablebanking/finance-session-status',{sessionId:'fictional-session'});assert.equal(permission.data.data.providerStatus,'AUTHORIZED');assert.equal(permission.data.data.validUntil,'2026-11-01T12:00:00Z');
  assert.equal((await call('/enablebanking/finance-revoke',{sessionId:'fictional-session'})).data.data.revoked,true,'Provider confirms permission revocation');
  const ownerLogin=await call('/account/login',{});const memberValidation=await fetch('http://127.0.0.1:'+port+'/account/validate',{headers:{'X-Finance-Capability':cap,'X-Finance-Identity':member,'X-Finance-Owner':'0','X-Actual-Token':ownerLogin.data.data.token}}).then(r=>r.json());assert.equal(memberValidation.data.permission,'BASIC','Forged native token cannot select owner');
  const snapshot=await call('/finance-internal/snapshot',{});assert.equal(snapshot.status,200,JSON.stringify(snapshot.data));
  const meta=snapshot.data.data.files.find(f=>f.path==='server-files/account.sqlite');assert.ok(meta);
  const dbPath=path.join(tmp,'sanitized-check.sqlite');await fs.writeFile(dbPath,Buffer.from(meta.data,'base64'));
  const require=createRequire(root+'/package.json'),Database=require('better-sqlite3'),db=new Database(dbPath,{readonly:true});
  try{assert.equal(db.prepare('PRAGMA integrity_check').get().integrity_check,'ok');for(const table of ['secrets','auth','sessions','pending_openid_requests'])assert.equal(db.prepare('SELECT count(*) AS n FROM '+table).get().n,0);assert.equal(db.prepare('SELECT count(*) AS n FROM users WHERE owner<>0 OR role<>?').get('BASIC').n,0)}finally{db.close()}
  assert.ok(!Buffer.from(meta.data,'base64').includes(Buffer.from(pem)),'Provider key excluded from snapshot');
  assert.equal((await call('/finance-internal/snapshot',{},member)).status,403,'Member cannot snapshot server');
 }finally{if(child.exitCode===null){child.kill();await new Promise(r=>child.once('exit',r));}}
});
