# Mesmas verificações do CI, para rodar antes de commitar:
# formatação (gofmt), análise estática (go vet), telas geradas em dia (templ) e testes.
# Uso: powershell -ExecutionPolicy Bypass -File scripts\verificar.ps1
$ErrorActionPreference = "Stop"
Set-Location (Split-Path $PSScriptRoot -Parent)

function Falhar($msg) { Write-Host "FALHOU: $msg" -ForegroundColor Red; exit 1 }

Write-Host "==> gofmt"
$sujos = gofmt -l cmd internal migrations web
if ($sujos) { Falhar "arquivos sem gofmt:`n$sujos" }

Write-Host "==> templ (telas geradas em dia)"
go tool templ generate
if ($LASTEXITCODE -ne 0) { Falhar "templ generate" }
$mudou = git status --porcelain -- "web/views/*_templ.go"
if ($mudou -match '^\s*M') { Falhar "os *_templ.go estavam desatualizados; commite os arquivos regenerados" }

Write-Host "==> sqlc (código gerado em dia)"
go tool sqlc generate
if ($LASTEXITCODE -ne 0) { Falhar "sqlc generate" }
$mudou = git status --porcelain -- internal/db/sqlc
if ($mudou -match '^\s*M') { Falhar "o código do sqlc estava desatualizado; commite os arquivos regenerados" }

Write-Host "==> go vet"
go vet ./...
if ($LASTEXITCODE -ne 0) { Falhar "go vet" }

Write-Host "==> testes"
$saida = go test -count=1 ./... 2>&1 | Out-String
Write-Host $saida
if ($LASTEXITCODE -ne 0) {
    # No Windows o antivírus às vezes segura o executável temporário do teste e o go acusa "unlinkat",
    # com todos os testes verdes. Isso não é falha de teste.
    $soUnlink = ($saida -match "unlinkat") -and ($saida -notmatch "(?m)^(--- FAIL|FAIL)")
    if (-not $soUnlink) { Falhar "testes" }
    Write-Host "Aviso: o Windows não conseguiu apagar um executável temporário de teste; os testes passaram." -ForegroundColor Yellow
}

Write-Host "Tudo certo." -ForegroundColor Green
