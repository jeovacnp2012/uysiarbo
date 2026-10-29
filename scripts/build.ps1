# Gera o binário único do UYSI ARBO (servidor + migrações + arquivos estáticos embutidos).
# Uso: powershell -ExecutionPolicy Bypass -File scripts\build.ps1 [-Linux] [-Versao 0.6.0]
#   padrão: executável para este Windows em bin\uysiarbo.exe
#   -Linux: executável para a VPS (Linux amd64) em bin\uysiarbo-linux-amd64
param(
    [switch]$Linux,
    [string]$Versao = ""
)

$ErrorActionPreference = "Stop"
Set-Location (Split-Path $PSScriptRoot -Parent)

if (-not $Versao) {
    $Versao = (git describe --tags --always --dirty 2>$null)
    if (-not $Versao) { $Versao = "dev" }
}

go tool templ generate
if ($LASTEXITCODE -ne 0) { exit 1 }

$env:CGO_ENABLED = "0"
if ($Linux) {
    $env:GOOS = "linux"; $env:GOARCH = "amd64"
    $saida = "bin\uysiarbo-linux-amd64"
} else {
    $saida = "bin\uysiarbo.exe"
}

go build -trimpath -ldflags "-s -w -X main.version=$Versao" -o $saida ./cmd/server
if ($LASTEXITCODE -ne 0) { exit 1 }

Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
$kb = [int]((Get-Item $saida).Length / 1KB)
Write-Host "Gerado $saida ($kb KB, versão $Versao)"
