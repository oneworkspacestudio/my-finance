import assert from 'node:assert/strict';import fs from 'node:fs/promises';
const base=process.env.FINANCE_TEST_URL||'http://127.0.0.1:25016/finance/';const origin=new URL(base).origin;
async function login(email){const r=await fetch(base+'api/login',{method:'POST',headers:{Origin:origin,'Content-Type':'application/json'},body:JSON.stringify({email,password:'Fictional-Test-Only-582!'})});assert.equal(r.status,200);return r.headers.getSetCookie().find(s=>s.startsWith('finance_session=')).split(';')[0]}
async function call(cookie,p,body,method){return fetch(base+p,{method:method||(body?'POST':'GET'),headers:{Origin:origin,Cookie:cookie,'Content-Type':'application/json'},...(body?{body:JSON.stringify(body)}:{})})}
const owner=await login('owner@example.test'),session=await(await call(owner,'api/me')).json();
const memberEmail='member-'+Date.now()+'@example.test';const added=await fetch(base+'api/local-users',{method:'POST',headers:{Origin:origin,Cookie:owner,'Content-Type':'application/json','X-CSRF-Token':session.csrf},body:JSON.stringify({email:memberEmail,password:'Fictional-Test-Only-582!'})});assert.equal(added.status,201);
const member=await login(memberEmail);const identity=await(await call(member,'ledger/account/validate')).json();assert.equal(identity.data.permission,'BASIC');
const ownerFiles=(await(await call(owner,'ledger/sync/list-user-files')).json()).data;const files=ownerFiles.files||ownerFiles;assert.ok(Array.isArray(files)&&files.length,'Native files exist');const file=files[0];const fileId=file.fileId||file.id;
const memberFiles=await(await call(member,'ledger/sync/list-user-files')).json();assert.deepEqual(memberFiles.data,[]);
assert.notEqual((await call(member,'ledger/sync/download-user-file')).status,200,'Unknown file cannot download');
const blocked=await fetch(base+'ledger/sync/download-user-file',{headers:{Cookie:member,'X-Actual-File-Id':fileId}});assert.notEqual(blocked.status,200,'Unshared file denied by server');
assert.equal((await call(member,'api/finance/backups/encrypt',{password:'synthetic phrase',file:''})).status,403);assert.equal((await call(member,'ledger/enablebanking/aspsps',{country:'GB'})).status,403);
const forged=await fetch(base+'ledger/admin/users',{method:'POST',headers:{Cookie:member,Origin:origin,'Content-Type':'application/json'},body:JSON.stringify({userName:'forged',displayName:'Fictional',role:'ADMIN',enabled:true})});assert.equal(forged.status,403);
const noOrigin=await fetch(base+'ledger/account/login',{method:'POST',headers:{Cookie:owner,'Content-Type':'application/json'},body:'{}'});assert.equal(noOrigin.status,403);
const share=await call(owner,'ledger/admin/access',{fileId,userId:identity.data.userId});assert.equal(share.status,200,await share.clone().text());
const allowed=await fetch(base+'ledger/sync/download-user-file',{headers:{Cookie:member,'X-Actual-File-Id':fileId}});assert.equal(allowed.status,200);
const revoke=await call(owner,'ledger/admin/access?fileId='+encodeURIComponent(fileId),{ids:[identity.data.userId]},'DELETE');assert.equal(revoke.status,200);
const denied=await fetch(base+'ledger/sync/download-user-file',{headers:{Cookie:member,'X-Actual-File-Id':fileId}});assert.notEqual(denied.status,200);
const receipt={schema:1,synthetic:true,checks:['native member BASIC','unshared file list/download denied','member backup/bank/admin denied','Origin required','native share grants download','native revoke denies download'],oneAccountLive:false};await fs.writeFile(process.env.FINANCE_MODULE_ROOT+'/household-report.json',JSON.stringify(receipt,null,2));console.log('PASS six native household/access checks (synthetic local app sessions)');
