// Loaded before Actual's modules. Native APIs remain on the authenticated app
// path and receive the apphost CSRF token. No bank or OneAccount secret is here.
(()=>{
 const script=document.currentScript;
 const ledger=new URL('./',script.src),base=new URL('../',ledger);
 const original=window.fetch.bind(window);
 const session=original(new URL('api/me',base),{credentials:'same-origin'}).then(async r=>{
   if(!r.ok){location.replace(new URL('auth/login?return='+encodeURIComponent(ledger.pathname),base));throw Error('Sign in again');}
   return r.json();
 });
 window.fetch=async(input,init={})=>{
   let u=new URL(input instanceof Request?input.url:input,location.href);
   const prefixes=['/account/','/sync/','/admin/','/enablebanking/','/simplefin/','/secret/','/gocardless/','/akahu/','/pluggyai/'];
   if(u.origin===location.origin&&prefixes.some(p=>u.pathname.startsWith(p))) u=new URL(u.pathname.slice(1)+u.search,ledger);
   if(u.origin===location.origin&&u.pathname.startsWith(ledger.pathname)){
     const s=await session;
     const h=new Headers(init.headers||(input instanceof Request?input.headers:undefined));
     h.set('X-CSRF-Token',s.csrf);h.set('X-Workspace-Expected-Account',s.user.id);
     init={...init,headers:h,credentials:'same-origin'};
   }
   return original(input instanceof Request?new Request(u,input):u,init);
 };
 // Actual persists its server URL separately from the browser route. A relative
 // native endpoint is required when running below the Workspace HTTPS proxy.
 const ready=()=>{
   document.title='My Finance · Actual Budget';
   if(!document.getElementById('finance-home')){
     const a=document.createElement('a');a.id='finance-home';a.href=base.href;
     a.textContent='My Finance';a.setAttribute('aria-label','My Finance settings and backups');
     a.style.cssText='position:fixed;right:12px;bottom:8px;z-index:100000;font:12px system-ui;background:#fff;color:#164aac;padding:5px 8px;border-radius:8px;border:1px solid #b9cae6';
     // Use a normal document link outside the engine viewport, never an Apps overlay.
     a.style.position='static';document.body.prepend(a);
   }
 };
 if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',ready);else ready();
})();
