# Cria os bancos do UYSI ARBO (desenvolvimento e testes) no Postgres do Laragon e habilita o PostGIS.
# Uso: powershell -File scripts\criar-banco.ps1 [-Bancos uysiarbo,uysiarbo_test] [-Usuario postgres] [-Porta 5432]
param(
    [string[]]$Bancos = @("uysiarbo", "uysiarbo_test"),
    [string]$Usuario = "postgres",
    [int]$Porta = 5432,
    [string]$Bin = "C:\laragon\bin\postgresql\postgresql\bin"
)

$psql = Join-Path $Bin "psql.exe"
$base = @("-h", "localhost", "-p", $Porta, "-U", $Usuario, "-v", "ON_ERROR_STOP=1")

foreach ($Banco in $Bancos) {
    $existe = & $psql @base -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$Banco'"
    if ($existe -ne "1") {
        & $psql @base -d postgres -c "CREATE DATABASE $Banco ENCODING 'UTF8'"
        if ($LASTEXITCODE -ne 0) { exit 1 }
        Write-Host "Banco '$Banco' criado."
    } else {
        Write-Host "Banco '$Banco' já existe."
    }

    & $psql @base -d $Banco -c "CREATE EXTENSION IF NOT EXISTS postgis"
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Falha ao habilitar o PostGIS. Instale-o no Postgres do Laragon (veja MEMORIA.md)." -ForegroundColor Red
        exit 1
    }
    & $psql @base -d $Banco -tAc "SELECT '${Banco}: PostGIS ' || postgis_version()"
}
