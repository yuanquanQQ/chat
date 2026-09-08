$ErrorActionPreference = 'Stop'
$root = Resolve-Path (Join-Path $PSScriptRoot '..\..')
$out = Join-Path $root 'release\Intrachat-Server'
if (Test-Path $out) { Remove-Item $out -Recurse -Force }
New-Item -ItemType Directory -Force $out | Out-Null
Copy-Item (Join-Path $root 'server') $out -Recurse
Copy-Item (Join-Path $root 'docs') $out -Recurse
Copy-Item (Join-Path $root 'deploy\server\README.md') $out
Copy-Item (Join-Path $root 'deploy\server\install.ps1') $out
Copy-Item (Join-Path $root 'deploy\server\stop.ps1') $out
Copy-Item (Join-Path $root 'docker-compose.yml') $out
Copy-Item (Join-Path $root '.env.example') $out
Copy-Item (Join-Path $root 'go.mod') $out
Copy-Item (Join-Path $root 'go.sum') $out
Write-Host "服务器部署包已生成：$out"
