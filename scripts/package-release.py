"""Package an independently updated gateway with its unchanged verified runtime.

Run locally after committing the tested source. This creates assets, never publishes.
The baseline directory must contain a verified earlier public portable release.
"""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parents[1]
REPO = 'oneworkspacestudio/my-finance'
RUNTIME_SHA = 'f3c4675bf401042356310325a35de736de3c7b2f53aab2e74b611839f98da3ca'


def sha(data):
    return hashlib.sha256(data).hexdigest()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--baseline', required=True, type=Path)
    parser.add_argument('--exe', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    def git(*values):
        return subprocess.check_output(['git', '-C', str(ROOT), *values], text=True, encoding='utf-8').strip()
    if git('status', '--porcelain'):
        raise SystemExit('Commit the tested source before packaging')
    contract = json.loads((ROOT / 'compatibility.json').read_text(encoding='utf-8'))
    version = contract['version']
    if json.loads((ROOT / 'package.json').read_text())['version'] != version:
        raise SystemExit('Version metadata mismatch')
    actual_version = subprocess.check_output([str(args.exe), '--version'], text=True).strip()
    if actual_version != version:
        raise SystemExit('Executable version does not match source')
    source = git('rev-parse', 'HEAD')
    old = json.loads((args.baseline / 'workspace-update.json').read_text())
    if contract['workspaceInstallerIntegration'] != old['integration'] or contract['minimumWorkspaceVersion'] != old['minimumManager']:
        raise SystemExit('Unchanged-runtime packaging requires the existing manager compatibility contract')
    baseline = args.baseline / old['packages']['finance']['asset']
    if old['repository'] != REPO or old['tag'] == 'v' + version:
        raise SystemExit('Use an earlier public Finance baseline')
    with zipfile.ZipFile(baseline) as archive:
        for package in old['packages'].values():
            data = archive.read(package['member'])
            if len(data) != package['size'] or sha(data) != package['sha256']:
                raise SystemExit('Baseline package hash mismatch')
        runtime = archive.read('finance-runtime.zip')
        if sha(runtime) != RUNTIME_SHA:
            raise SystemExit('Runtime changes require rebuilding and validating upstream patches')
        retained = {name: archive.read(name) for name in archive.namelist()
                    if name.startswith('LICENSES/') or name == 'THIRD-PARTY-LICENSES.txt'}
    args.output.mkdir(parents=True, exist_ok=False)
    filename = 'my-finance-' + version + '-windows-amd64.zip'
    portable = args.output / filename
    files = {**retained, 'finance-runtime.zip': runtime, 'MyFinance.exe': args.exe.read_bytes()}
    for name in ['README.md', 'LICENSE', 'NOTICE', 'compatibility.json']:
        files[name] = (ROOT / name).read_bytes()
    for path in (ROOT / 'docs').rglob('*'):
        if path.is_file():
            files[path.relative_to(ROOT).as_posix()] = path.read_bytes()
    with zipfile.ZipFile(portable, 'w', zipfile.ZIP_DEFLATED) as archive:
        for name, data in sorted(files.items()):
            info = zipfile.ZipInfo(name, (2026, 11, 3, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(info, data)
    old['tag'] = 'v' + version
    for app in old['apps']:
        app.update(version=version, source='https://github.com/' + REPO + '/releases/tag/v' + version)
    for package in old['packages'].values():
        data = files[package['member']]
        package.update(asset=filename, size=len(data), sha256=sha(data))
    (args.output / 'workspace-update.json').write_text(json.dumps(old, indent=2), encoding='utf-8')
    for name in ['Actual-26.10.0-source.zip', 'HyperFormula-3.5.0-source.zip', 'THIRD-PARTY-LICENSES.txt']:
        shutil.copyfile(args.baseline / name, args.output / name)
    shutil.copyfile(args.exe, args.output / 'MyFinance.exe')
    shutil.copyfile(ROOT / 'compatibility.json', args.output / 'compatibility.json')
    info = json.loads((args.baseline / 'build-info.json').read_text())
    info.update(commit=source, version=version, financeDataSchema=contract['financeDataSchema'],
                aiProviders=['openai', 'deepseek'], liveAIProviderVerified=False)
    (args.output / 'build-info.json').write_text(json.dumps(info, indent=2), encoding='utf-8')
    subprocess.run(['git', '-C', str(ROOT), 'archive', '--format=zip',
                    '--output=' + str(args.output / 'my-finance-source.zip'), source], check=True)
    (args.output / 'SHA256SUMS.txt').write_text(''.join(
        sha(path.read_bytes()) + '  ' + path.name + '\n'
        for path in sorted(args.output.iterdir()) if path.name != 'SHA256SUMS.txt'), encoding='utf-8')
    print(json.dumps({'source': source, 'tag': 'v' + version, 'assets': len(list(args.output.iterdir())),
                      'portableSHA256': sha(portable.read_bytes())}))


if __name__ == '__main__':
    main()
