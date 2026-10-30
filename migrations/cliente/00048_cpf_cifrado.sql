-- +goose Up
-- 13.3: o CPF passa a ser guardado cifrado pela aplicação ("v1:..."), então o CHECK de 11 dígitos sai.
-- A validação dos 11 dígitos continua nas telas. Dado antigo em texto puro segue válido até rodar
-- "uysiarbo dados cifrar".
ALTER TABLE locais DROP CONSTRAINT locais_proprietario_cpf_check;
ALTER TABLE locais ADD CONSTRAINT locais_proprietario_cpf_check CHECK (length(proprietario_cpf) <= 200);
ALTER TABLE denuncias DROP CONSTRAINT denuncias_cidadao_cpf_check;
ALTER TABLE denuncias ADD CONSTRAINT denuncias_cidadao_cpf_check CHECK (length(cidadao_cpf) <= 200);
ALTER TABLE lvc_testes DROP CONSTRAINT lvc_testes_proprietario_cpf_check;
ALTER TABLE lvc_testes ADD CONSTRAINT lvc_testes_proprietario_cpf_check CHECK (length(proprietario_cpf) <= 200);

-- +goose Down
ALTER TABLE locais DROP CONSTRAINT locais_proprietario_cpf_check;
ALTER TABLE locais ADD CONSTRAINT locais_proprietario_cpf_check CHECK (proprietario_cpf ~ '^([0-9]{11})?$');
ALTER TABLE denuncias DROP CONSTRAINT denuncias_cidadao_cpf_check;
ALTER TABLE denuncias ADD CONSTRAINT denuncias_cidadao_cpf_check CHECK (cidadao_cpf ~ '^[0-9]{0,11}$');
ALTER TABLE lvc_testes DROP CONSTRAINT lvc_testes_proprietario_cpf_check;
ALTER TABLE lvc_testes ADD CONSTRAINT lvc_testes_proprietario_cpf_check CHECK (proprietario_cpf ~ '^[0-9]{0,11}$');
