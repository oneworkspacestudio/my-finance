"""Package verified Node and pinned npm output; never include user data."""
import argparse,hashlib,json,pathlib,zipfile
p=argparse.ArgumentParser();p.add_argument('--cache',required=True);p.add_argument('--output',required=True);a=p.parse_args()
c=pathlib.Path(a.cache);out=pathlib.Path(a.output);out.parent.mkdir(parents=True,exist_ok=True)
assert json.loads((c/'node_modules/@actual-app/sync-server/package.json').read_text())['version']=='26.10.0'
assert 'x-finance-capability' in (c/'node_modules/@actual-app/sync-server/build/chunks/app-BgMEdvse.js').read_text(encoding='utf-8')
node=c/'node-v24.22.1-win-x64';expected=next(s.split()[0] for s in (c/'node-shasums.txt').read_text().splitlines() if s.endswith(' node-v24.22.1-win-x64.zip'))
assert hashlib.sha256((c/'node.zip').read_bytes()).hexdigest()==expected
with zipfile.ZipFile(out,'w',zipfile.ZIP_DEFLATED,compresslevel=6) as z:
 z.write(node/'node.exe','node.exe');z.write(node/'LICENSE','LICENSES/Node-LICENSE.txt')
 for f in sorted((c/'node_modules').rglob('*')):
  if not f.is_file():continue
  rel=f.relative_to(c).as_posix()
  if any(x in rel for x in ('node_modules/@playwright/','node_modules/playwright/','node_modules/playwright-core/','node_modules/.bin/')):continue
  z.write(f,rel)
print(json.dumps({'file':out.name,'size':out.stat().st_size,'sha256':hashlib.sha256(out.read_bytes()).hexdigest()}))
