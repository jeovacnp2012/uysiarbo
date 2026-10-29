# Mesmas verificações do CI, para rodar antes de commitar:
# formatação (gofmt), código gerado em dia (templ e sqlc), análise estática (go vet) e testes.
# Uso: powershell -ExecutionPolicy Bypass -File scripts\verificar.ps1
$ErrorActionPreference = "Stop"
Set-Location (Split-Path $PSScriptRoot -Parent)

function Falhar($msg) { Write-Host "FALHOU: $msg" -ForegroundColor Red; exit 1 }

# Impressão digital do conteúdo de uma pasta, para saber se a geração de código mudou algo.
function Digital($padrao) {
    (Get-ChildItem -Path $padrao -File -ErrorAction SilentlyContinue | Sort-Object FullName |
        ForEach-Object { (Get-FileHash $_.FullName).Hash }) -join ""
}

Write-Host "==> gofmt"
$sujos = gofmt -l cmd internal migrations web
if ($sujos) { Falhar "arquivos sem gofmt:`n$sujos" }

Write-Host "==> templ (telas geradas em dia)"
$antes = Digital "web/views/*_templ.go"
go tool templ generate | Out-Null
if ($LASTEXITCODE -ne 0) { Falhar "templ generate" }
if ($antes -ne (Digital "web/views/*_templ.go")) { Falhar "os *_templ.go estavam desatualizados; foram regenerados, confira e commite" }

Write-Host "==> sqlc (código gerado em dia)"
$antes = Digital "internal/db/sqlc*/*.go"
go tool sqlc generate
if ($LASTEXITCODE -ne 0) { Falhar "sqlc generate" }
if ($antes -ne (Digital "internal/db/sqlc*/*.go")) { Falhar "o código do sqlc estava desatualizado; foi regenerado, confira e commite" }

Write-Host "==> go vet"
go vet ./...
if ($LASTEXITCODE -ne 0) { Falhar "go vet" }

Write-Host "==> testes"
$ErrorActionPreference = "Continue"  # o stderr do go (ex.: "unlinkat") não deve abortar o script
$saida = go test -count=1 ./... 2>&1 | Out-String
$codigo = $LASTEXITCODE
$ErrorActionPreference = "Stop"
Write-Host $saida
if ($codigo -ne 0) {
    # No Windows o antivírus às vezes segura o executável temporário do teste e o go acusa "unlinkat",
    # com todos os testes verdes. Isso não é falha de teste.
    $soUnlink = ($saida -match "unlinkat") -and ($saida -notmatch "(?m)^(--- FAIL|FAIL)")
    if (-not $soUnlink) { Falhar "testes" }
    Write-Host "Aviso: o Windows não conseguiu apagar um executável temporário de teste; os testes passaram." -ForegroundColor Yellow
}

Write-Host "Tudo certo." -ForegroundColor Green
