// Deterministic mock only. This never contacts a provider or a real bank.
const original=globalThis.fetch;
globalThis.fetch=async(input,options={})=>{
 const u=new URL(input instanceof Request?input.url:input);
 if(u.origin!=='https://api.enablebanking.com') return original(input,options);
 const json=x=>new Response(JSON.stringify(x),{headers:{'Content-Type':'application/json'}});
 if(u.pathname==='/application')return json({name:'Fictional test application',environment:'SANDBOX'});
 if(u.pathname==='/aspsps')return json({aspsps:[{name:'Fictional Test Bank',country:'GB',psu_types:['personal']}]});
 if(u.pathname==='/auth'){const b=JSON.parse(options.body);if(b.aspsp.country!=='GB'||b.psu_type!=='personal')throw Error('AIS request mismatch');return json({url:'https://provider.example.test/consent?state='+b.state});}
 if(u.pathname==='/sessions')return json({session_id:'fictional-session',access:{valid_until:'2026-11-01T12:00:00Z'},aspsp:{name:'Fictional Test Bank',country:'GB'},accounts:[{uid:'fictional-account',name:'Fictional current account',currency:'GBP',account_id:{iban:'GB00FICTIONAL00001234'}}]});
 if(u.pathname==='/sessions/fictional-session')return json(options.method==='DELETE'?{message:'OK'}:{status:'AUTHORIZED',access:{valid_until:'2026-11-01T12:00:00Z'}});
 if(u.pathname.endsWith('/balances'))return json({balances:[{balance_amount:{amount:'1234.56',currency:'GBP'},balance_type:'CLAV',reference_date:'2026-10-28'}]});
 if(u.pathname.endsWith('/transactions')){
  const second=u.searchParams.get('continuation_key')==='page-two';
  return json({transactions:Array.from({length:50},(_,i)=>({entry_reference:'fictional-'+(i+(second?50:0)),booking_date:'2026-10-20',transaction_amount:{amount:'1.01',currency:'GBP'},credit_debit_indicator:'DBIT',creditor:{name:'Fictional merchant'},status:'BOOK'})),...(!second?{continuation_key:'page-two'}:{})});
 }
 throw Error('Unexpected mock provider path: '+u.pathname);
};
await import(process.env.FINANCE_ACTUAL_ENTRY);
