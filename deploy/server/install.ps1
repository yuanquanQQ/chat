$ErrorActionPreference = 'Stop'
$root = Resolve-Path (Join-Path $PSScriptRoot '..\..')
Set-Location $root
if (-not (Test-Path '.env')) {
  Copy-Item '.env.example' '.env'
  Write-Host '已创建 .env，请先填写强密码和 JWT_SECRET，然后重新运行此脚本。' -ForegroundColor Yellow
  exit 1
}
docker compose config --quiet
docker compose up -d --build
docker compose ps
Write-Host '服务器已启动。健康检查：http://127.0.0.1:8080/health' -ForegroundColor Green

