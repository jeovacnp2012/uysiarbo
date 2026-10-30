# Backups e restauração (13.2)

O comando `uysiarbo backup` usa o `pg_dump` e guarda **um arquivo `.dump` por schema**: o catálogo (`public`) e um por cliente (`c_<slug>`). Cada rodada vira uma pasta `AAAAMMDD-HHMMSS` com os arquivos e um `manifesto.json` (tamanho e SHA-256 de cada um). Ao terminar, o comando **confere a própria rodada** (SHA-256 e leitura do índice pelo `pg_restore --list`) e apaga as mais antigas, mantendo as 14 mais novas.

## Rodar
```
uysiarbo backup                      # copia, confere e rotaciona
uysiarbo backup listar               # rodadas guardadas
uysiarbo backup verificar [rodada]   # reconfere (a mais nova se omitida)
uysiarbo backup -manter 30 -dir /var/backups/uysiarbo
```
Configuração por ambiente (ou `.env`): `BACKUP_DIR` (padrão `./backups`) e `PG_BIN_DIR` (pasta do `pg_dump`/`pg_restore`, se não estiverem no PATH; no Laragon: `C:\laragon\bin\postgresql\postgresql\bin`). A senha do banco vai ao `pg_dump` por variável de ambiente, não aparece na lista de processos.

## Agendar
**VPS (systemd timer)**, `/etc/systemd/system/uysiarbo-backup.service`:
```
[Service]
Type=oneshot
User=uysiarbo
WorkingDirectory=/var/www/uysiarbo
ExecStart=/var/www/uysiarbo/uysiarbo backup -dir /var/backups/uysiarbo
```
e `/etc/systemd/system/uysiarbo-backup.timer`:
```
[Timer]
OnCalendar=*-*-* 02:30:00
Persistent=true
[Install]
WantedBy=timers.target
```
Ative com `sudo systemctl enable --now uysiarbo-backup.timer` (precisa de sudo: o Claude não roda). Alternativa: `cron` diário com a mesma linha de comando.

**Windows (desenvolvimento)**: `schtasks /create /tn "UYSI ARBO backup" /tr "C:\laragon\www\uysiarbo\bin\uysiarbo.exe backup" /sc daily /st 02:30` (rodando na pasta do projeto, para achar o `.env`).

## Fora do servidor
Backup só no mesmo disco não protege contra perda do servidor. Copie a pasta de backups para outro lugar (outro servidor, armazenamento de objetos), por exemplo `rsync`/`rclone` logo depois do timer. Os arquivos têm CPF cifrado, mas **telefones, nomes e e-mails estão em texto**: trate a cópia como dado pessoal (disco cifrado, acesso restrito).

## A chave dos dados
Os CPFs só abrem com a **`DADOS_CHAVE`** do `.env`. Ela **não vai no backup**: guarde uma cópia em cofre de senhas, fora do servidor. Sem ela, um backup restaurado mostra os CPFs como vazios.

## Restaurar
Sempre ensaie primeiro num banco de teste:
```
# 1) banco novo com PostGIS
createdb uysiarbo_restauro && psql -d uysiarbo_restauro -c "CREATE EXTENSION postgis"
# 2) cada schema que precisar (catálogo e/ou clientes)
uysiarbo backup restaurar -arquivo backups/20261030-233725/c_novamutum.dump -banco postgres://usuario:senha@host:5432/uysiarbo_restauro
```
- O destino precisa ter o PostGIS e **não ter** o schema (ou use `-limpar` para apagar antes os objetos existentes).
- Restaurar sobre o banco do sistema exige `-sobre-o-atual` (de propósito, para não ser por engano).
- Recuperação total: restaure `public.dump` e depois o `.dump` de cada cliente, rode `uysiarbo migrate-all` e confira `/readyz`.
- Depois de restaurar, confira contagens (locais, visitas) com as do banco de origem.

## Testado (2026-10-30)
Rodada real do banco local (catálogo + `c_teste` + `c_novamutum`, 3,8 MB), conferida e restaurada num banco novo: 39.131 locais e 36.384 visitas, iguais às do original. Teste automatizado em `internal/backup` (cópia, conferência, restauração e detecção de arquivo alterado).
