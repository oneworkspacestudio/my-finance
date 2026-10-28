param([string]$Executable='I:/Projects/tooling/my-finance-20261028/release/MyFinance.exe',[string]$Data='C:/Projects/.cache/my-finance-20261028/Standalone')
& $Executable --data $Data --listen 127.0.0.1:5016 --origin http://localhost:5016/finance
