"""Collect installed runtime and statically linked Go notices without user data."""
import argparse,json,pathlib,subprocess
p=argparse.ArgumentParser();p.add_argument('--cache',required=True);p.add_argument('--output',required=True);p.add_argument('--go',default='go');a=p.parse_args()
root=pathlib.Path(__file__).resolve().parent.parent;cache=pathlib.Path(a.cache)
parts=['My Finance third-party notices\n\nMy Finance gateway source is MIT. Bundled software retains its own licences.\nActual includes HyperFormula under GPL-3.0-only; see LICENSES/GPL-3.0.txt and\nthe matching source archives distributed beside this release. The combined\nActual/HyperFormula work is provided under GPLv3 where that licence applies.\nThe separate Go gateway communicates with Actual over authenticated HTTP.\n']
def section(name,directory):
 d=pathlib.Path(directory);texts=[]
 for f in sorted(d.iterdir()):
  if f.is_file() and (f.name.lower().startswith(('license','licence','copying','notice','copyright'))):
   try:texts.append(f.name+'\n'+f.read_text(encoding='utf-8'))
   except UnicodeError:pass
 if not texts:
  for f in sorted(d.glob('*')):
   if f.is_file() and f.name.lower().startswith('readme'):
    body=f.read_text(encoding='utf-8')
    if 'license' in body.lower() or 'licence' in body.lower():texts.append(f.name+'\n'+body)
 if not texts and (d/'package.json').exists():texts.append('Upstream package declaration:\n'+(d/'package.json').read_text(encoding='utf-8'))
 parts.append('\n'+'='*72+'\n'+name+'\n'+'\n\n'.join(texts or ['No top-level licence file in this package; inspect its source archive.'])+'\n')
lock=json.loads((root/'package-lock.json').read_text())['packages']
for name,v in sorted(lock.items()):
 if not name or v.get('dev') or v.get('optional') and not (cache/name).exists():continue
 d=cache/name
 if d.exists():section('npm '+name+' '+v.get('version','')+' ['+str(v.get('license','see source'))+']',d)
raw=subprocess.check_output([a.go,'list','-deps','-json','./cmd/my-finance'],cwd=root,text=True,encoding='utf-8');decoder=json.JSONDecoder();seen=set()
while raw.strip():
 package,n=decoder.raw_decode(raw.lstrip());raw=raw.lstrip()[n:];mod=package.get('Module',{})
 if not mod or mod.get('Main') or mod['Path'] in seen:continue
 seen.add(mod['Path'])
 section('Go '+mod['Path']+' '+mod.get('Version',''),mod['Dir'])
goroot=subprocess.check_output([a.go,'env','GOROOT'],cwd=root,text=True).strip();section('Go standard library/toolchain',goroot)
section('Node 24.22.1',cache/'node-v24.22.1-win-x64')
section('Actual source copyright',(root/'LICENSES'))
section('Additional upstream notices',(root/'LICENSES/npm'))
text='\n'.join(parts)
pathlib.Path(a.output).write_bytes(text.replace('\r\n','\n').encode('utf-8'))
print('Collected',len(parts)-1,'runtime/module notice groups')
