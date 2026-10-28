param([string]$Cache='C:/Projects/.cache/my-finance-20261028',[string]$Output='I:/Projects/tooling/my-finance-20261028/release',[string]$Go='go')
$ErrorActionPreference='Stop'
$source=Split-Path $PSScriptRoot -Parent
New-Item -ItemType Directory -Force $Cache,$Output,(Join-Path $Cache 'go-tmp')|Out-Null
Copy-Item (Join-Path $source 'package.json'),(Join-Path $source 'package-lock.json') $Cache
$env:PATH=(Join-Path $Cache 'node-v24.22.1-win-x64')+';'+$env:PATH
Push-Location $Cache
try{npm ci --omit=dev --no-audit --no-fund;if($LASTEXITCODE){throw 'npm ci failed'}}finally{Pop-Location}
node (Join-Path $source 'scripts/patch-actual.mjs') $Cache
if($LASTEXITCODE){throw 'Upstream patch gate failed'}
python (Join-Path $source 'scripts/package-runtime.py') --cache $Cache --output (Join-Path $Output 'finance-runtime.zip')
if($LASTEXITCODE){throw 'Runtime packaging failed'}
$digest=(Get-FileHash (Join-Path $Output 'finance-runtime.zip')).Hash.ToLowerInvariant()
$env:GOTMPDIR=Join-Path $Cache 'go-tmp'
Push-Location $source
try{& $Go build -trimpath -ldflags "-s -w -X main.runtimeSHA=$digest" -o (Join-Path $Output 'MyFinance.exe') ./cmd/my-finance;if($LASTEXITCODE){throw 'Launcher build failed'}}finally{Pop-Location}
Copy-Item (Join-Path $source 'README.md'),(Join-Path $source 'LICENSE'),(Join-Path $source 'NOTICE') $Output
