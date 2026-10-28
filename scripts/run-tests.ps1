param([string]$Go='go')
$ErrorActionPreference='Stop'
& $Go test ./...;if($LASTEXITCODE){throw 'Go tests failed'}
& $Go vet ./...;if($LASTEXITCODE){throw 'Go vet failed'}
