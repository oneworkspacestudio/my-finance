function financeMinor(value){
 if(typeof value!=='string'||!/^[-+]?\d{1,12}(?:\.\d{1,2})?$/.test(value.trim())) throw Error('Unsupported bank amount precision');
 const s=value.trim(),negative=s.startsWith('-'),[units,fraction='']=s.replace(/^[-+]/,'').split('.');
 const minor=BigInt(units)*100n+BigInt(fraction.padEnd(2,'0'));
 if(minor>BigInt(Number.MAX_SAFE_INTEGER)) throw Error('Bank amount exceeds safe minor units');
 return Number(negative?-minor:minor);
}
// Injected once, before all upstream routes. Access requires the random service
// capability. It is supplied only by the Go gateway, never browser JavaScript.
app.use((req,res,next)=>{
 const cap=req.headers['x-finance-capability'];
 const expected=process.env.MYFINANCE_ENGINE_CAPABILITY;
 if(!expected||typeof cap!=='string'||cap.length!==expected.length||
    !crypto.timingSafeEqual(Buffer.from(cap),Buffer.from(expected))) return res.sendStatus(403);
 if(req.path==='/health') return res.json({status:'UP'});
 const identity=req.headers['x-finance-identity'];
 if(typeof identity!=='string'||!/^\w{64}$/.test(identity)) return res.sendStatus(403);
 const db=getAccountDb();
 const display=Buffer.from(String(req.headers['x-finance-display-name']||''),'base64url').toString('utf8').slice(0,120)||'OneAccount member';
 let user=db.first('SELECT * FROM users WHERE user_name = ?',[identity]);
 const owner=req.headers['x-finance-owner']==='1';
 if(!user){
   insertUser(v4(),identity,display,1,owner?'ADMIN':'BASIC');
   user=db.first('SELECT * FROM users WHERE user_name = ?',[identity]);
 }
 if(!user.enabled) return res.sendStatus(403);
 // Upstream directory changes cannot turn a different OneAccount into an admin.
 db.mutate('UPDATE users SET display_name=?,owner=?,role=? WHERE id=?',[display,owner?1:0,owner?'ADMIN':'BASIC',user.id]);
 const token=crypto.createHmac('sha256',expected).update(identity).digest('hex');
 db.mutate('INSERT INTO sessions(token,expires_at,user_id,auth_method) VALUES(?,?,?,?) ON CONFLICT(token) DO UPDATE SET expires_at=excluded.expires_at',
   [token,Math.floor(Date.now()/1000)+900,user.id,'openid']);
 req.headers['x-actual-token']=token;
 if(req.path==='/account/needs-bootstrap') return res.json({status:'ok',data:{bootstrapped:true,loginMethod:'header',availableLoginMethods:[{method:'header',active:true,displayName:'OneAccount'}],multiuser:true}});
 if(req.path==='/account/login-methods') return res.json({status:'ok',methods:[{method:'header',active:true,displayName:'OneAccount'}]});
 if(req.path==='/account/login') return res.json({status:'ok',data:{token}});
 if(['/gocardless/status','/akahu/status','/pluggyai/status'].includes(req.path)||
    !owner&&['/enablebanking/status','/simplefin/status'].includes(req.path)) return res.json({status:'ok',data:{configured:false,enabled:false}});
 if(['/account/bootstrap','/account/change-password'].includes(req.path)||req.path.startsWith('/openid/')) return res.sendStatus(403);
 // Native bank credentials are instance-wide. Management/sync is owner-only.
 if(['/enablebanking','/simplefin','/secret','/gocardless','/akahu','/pluggyai'].some(p=>req.path.startsWith(p+'/'))&&!owner) return res.sendStatus(403);
 // New installations do not expose legacy providers or the generic CORS proxy.
 if(['/gocardless/','/akahu/','/pluggyai/','/cors-proxy/'].some(p=>req.path.startsWith(p))) return res.sendStatus(403);
 next();
});

app.post('/enablebanking/finance-session-status',express.json({limit:'4kb'}),async(req,res)=>{
 try{
  const id=req.body?.sessionId;if(typeof id!=='string'||!/^[A-Za-z0-9-]{1,200}$/.test(id))return res.sendStatus(400);
  const session=await enableBankingService.getSession(id);
  return res.json({status:'ok',data:{providerStatus:session.status,validUntil:session.access?.valid_until||null}});
 }catch{return res.status(502).json({status:'error',reason:'provider-permission-unavailable'})}
});
app.post('/enablebanking/finance-revoke',express.json({limit:'4kb'}),async(req,res)=>{
 try{
  const id=req.body?.sessionId;if(typeof id!=='string'||!/^[A-Za-z0-9-]{1,200}$/.test(id))return res.sendStatus(400);
  const result=await request('DELETE','/sessions/'+encodeURIComponent(id));
  if(result.message!=='OK')throw Error('Provider did not confirm closure');
  return res.json({status:'ok',data:{revoked:true}});
 }catch{return res.status(502).json({status:'error',reason:'provider-revocation-unconfirmed'})}
});
