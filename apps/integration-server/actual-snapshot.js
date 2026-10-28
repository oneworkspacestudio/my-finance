// Quiescent server snapshots preserve Actual's opaque/E2EE budget blobs. Only
// the authenticated gateway can request one. New writes wait briefly; an active
// long poll causes a safe busy failure rather than an inconsistent snapshot.
let financeSnapshotLocked=false,financeRequests=0;
const financeWaiting=[];
app.use((req,res,next)=>{
 const cap=req.headers['x-finance-capability'],expected=process.env.MYFINANCE_ENGINE_CAPABILITY;
 if(!expected||typeof cap!=='string'||cap.length!==expected.length||!crypto.timingSafeEqual(Buffer.from(cap),Buffer.from(expected)))return res.sendStatus(403);
 if(req.path.startsWith('/finance-internal/')){
   if(req.headers['x-finance-owner']!=='1')return res.sendStatus(403);
   return next();
 }
 const run=()=>{if(res.destroyed)return;financeRequests++;let finished=false;const release=()=>{if(!finished){finished=true;financeRequests--}};res.once('finish',release);res.once('close',release);next()};
 if(financeSnapshotLocked){if(financeWaiting.length>=128)return res.sendStatus(503);financeWaiting.push(run)}else run();
});
app.post('/finance-internal/snapshot',async(req,res)=>{
 if(financeSnapshotLocked)return res.sendStatus(409);
 financeSnapshotLocked=true;
 const temp=join(resolve(configSchema.get('serverFiles')),'finance-snapshot-'+crypto.randomUUID()+'.sqlite');
 try{
  const deadline=Date.now()+5000;
  while(financeRequests>0&&Date.now()<deadline)await new Promise(r=>setTimeout(r,25));
  if(financeRequests>0)return res.status(503).json({status:'error',reason:'budget-busy-retry'});
  await getAccountDb().db.backup(temp);
  const copy=openDatabase(temp);
  try{
   copy.exec("PRAGMA secure_delete=ON; DELETE FROM secrets; DELETE FROM auth; DELETE FROM sessions; DELETE FROM pending_openid_requests; UPDATE users SET owner=0,role='BASIC'; VACUUM; PRAGMA wal_checkpoint(TRUNCATE);");
   if(copy.first('PRAGMA integrity_check')?.integrity_check!=='ok')throw Error('Snapshot integrity');
   for(const table of ['secrets','auth','sessions','pending_openid_requests'])if(copy.first('SELECT count(*) AS n FROM '+table).n!==0)throw Error('Snapshot credential exclusion');
  }finally{copy.close()}
  const files=[];let total=0;
  const add=(name,bytes)=>{total+=bytes.length;if(total>128*1024*1024)throw Error('Snapshot limit');files.push({path:name,data:bytes.toString('base64')})};
  add('server-files/account.sqlite',fs.readFileSync(temp));
  const migration=join(resolve(configSchema.get('dataDir')),'.migrate');if(fs.existsSync(migration))add('.migrate',fs.readFileSync(migration));
  const root=resolve(configSchema.get('userFiles'));
  for(const name of fs.readdirSync(root).sort()){
   if(!/^(?:file-[0-9a-f-]{36}\.blob|group-[0-9a-f-]{36}\.sqlite)$/i.test(name))continue;
   const p=join(root,name),stat=fs.lstatSync(p);if(!stat.isFile()||stat.isSymbolicLink())throw Error('Linked budget file');add('user-files/'+name,fs.readFileSync(p));
  }
  return res.json({status:'ok',data:{files}});
 }catch{return res.status(503).json({status:'error',reason:'snapshot-not-created'})}
 finally{
  for(const suffix of ['', '-wal','-shm'])try{fs.unlinkSync(temp+suffix)}catch{}
  financeSnapshotLocked=false;for(const run of financeWaiting.splice(0))run();
 }
});
