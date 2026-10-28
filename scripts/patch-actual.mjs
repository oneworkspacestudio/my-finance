import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const root=path.resolve(process.argv[2]||fileURLToPath(new URL('..',import.meta.url)));
const pkg=path.join(root,'node_modules/@actual-app/sync-server');
if(JSON.parse(fs.readFileSync(path.join(pkg,'package.json'))).version!=='26.10.0') throw Error('Review upstream before upgrading');
function patch(name,from,to){
 const p=path.join(pkg,'build/chunks',name),s=fs.readFileSync(p,'utf8');
 if(s.includes(to)) return;
 if(s.split(from).length!==2) throw Error('Upstream patch anchor changed: '+name);
 fs.writeFileSync(p,s.replace(from,to));
}
// Only the signed-in gateway can choose an Actual identity. Browser tokens cannot
// override it through JSON bodies. Ledger and file ACL code remain upstream.
patch('account-db-CyiKvvrD.js','let { token } = req.body || {};','let token = req.headers["x-actual-token"];');
const bridge=fs.readFileSync(new URL('../apps/integration-server/actual-bridge.js',import.meta.url),'utf8');
patch('app-BgMEdvse.js','app.use(cors());',bridge+'\napp.use(cors());');
// Keep provider errors and authorization codes out of upstream request logging.
patch('app-BgMEdvse.js','app$7.use(requestLoggerMiddleware);','// Finance: banking request URLs are not logged.');
patch('app-BgMEdvse.js','requestUrl: req.url,\n\t\tstacktrace: err.stack','requestUrl: req.path,\n\t\tstacktrace: "redacted"');
// Other upstream console/debug output is suppressed by the supervisor. Only safe
// process lifecycle events are recorded by the gateway.
console.log('Actual 26.10.0 bridge applied; native ledger unchanged.');
