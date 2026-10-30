package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/cifra"
	"uysiarbo/web/views"
)

// alvoLGPD descreve onde uma pessoa pode aparecer com dados pessoais e como anonimizar o registro.
type alvoLGPD struct {
	Tabela, Rotulo string
	CPF, Tel, Mail []string // colunas de cada tipo de identificador
	Limpar         string   // cláusula SET que apaga os dados pessoais
}

var alvosLGPD = []alvoLGPD{
	{"locais", "Locais (proprietário/inquilino)", []string{"proprietario_cpf"}, []string{"proprietario_telefone", "proprietario_whatsapp", "inquilino_telefone"}, []string{"proprietario_email"},
		"proprietario='', proprietario_telefone='', proprietario_cpf='', proprietario_whatsapp='', proprietario_email='', aceita_whatsapp=false, aceita_email=false, inquilino='', inquilino_telefone=''"},
	{"denuncias", "Denúncias", []string{"cidadao_cpf"}, []string{"cidadao_telefone"}, nil,
		"anonima=true, cidadao_nome='', cidadao_telefone='', cidadao_cpf=''"},
	{"lvc_testes", "Testes de LVC", []string{"proprietario_cpf"}, []string{"proprietario_telefone"}, nil,
		"proprietario='', proprietario_telefone='', proprietario_cpf=''"},
	{"investigacoes", "Investigações", nil, []string{"telefone"}, nil,
		"paciente='ANONIMIZADO', telefone=''"},
	{"baap_notificacoes", "Notificações BAAP", nil, []string{"telefone", "telefone2"}, nil,
		"paciente='ANONIMIZADO', nome_mae='', telefone='', telefone2='', sinam=''"},
}

// telefoneIgual compara pelos dígitos, aceitando um dos lados com ou sem o 55 e o DDD (mínimo 8 dígitos).
func telefoneIgual(guardado, procurado string) bool {
	a, b := soDigitos(guardado), soDigitos(procurado)
	if len(a) < 8 || len(b) < 8 {
		return false
	}
	return strings.HasSuffix(a, b) || strings.HasSuffix(b, a)
}

type achadoLGPD struct {
	Alvo alvoLGPD
	IDs  []int64
}

type consultadorLGPD interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

// buscarTitular varre as tabelas com dados pessoais (o CPF fica cifrado, então a comparação é feita aqui).
func buscarTitular(ctx context.Context, db consultadorLGPD, tipo, valor string) ([]achadoLGPD, error) {
	var out []achadoLGPD
	for _, a := range alvosLGPD {
		var cols []string
		switch tipo {
		case "cpf":
			cols = a.CPF
		case "telefone":
			cols = a.Tel
		case "email":
			cols = a.Mail
		}
		if len(cols) == 0 {
			continue
		}
		rows, err := db.Query(ctx, "SELECT id, "+strings.Join(cols, ", ")+" FROM "+a.Tabela)
		if err != nil {
			return nil, err
		}
		var ids []int64
		for rows.Next() {
			var id int64
			vals := make([]string, len(cols))
			ptr := make([]any, 0, len(cols)+1)
			ptr = append(ptr, &id)
			for i := range vals {
				ptr = append(ptr, &vals[i])
			}
			if err := rows.Scan(ptr...); err != nil {
				rows.Close()
				return nil, err
			}
			for _, v := range vals {
				if v == "" {
					continue
				}
				bate := false
				switch tipo {
				case "cpf":
					bate = soDigitos(cifra.Decifrar(v)) == valor
				case "telefone":
					bate = telefoneIgual(v, valor)
				case "email":
					bate = strings.EqualFold(strings.TrimSpace(v), valor)
				}
				if bate {
					ids = append(ids, id)
					break
				}
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if len(ids) > 0 {
			out = append(out, achadoLGPD{a, ids})
		}
	}
	return out, nil
}

func normalizarTitular(tipo, valor string) (string, string) {
	valor = strings.TrimSpace(valor)
	switch tipo {
	case "cpf":
		d := soDigitos(valor)
		if len(d) != 11 {
			return "", "Informe um CPF com 11 dígitos."
		}
		return d, ""
	case "telefone":
		d := soDigitos(valor)
		if len(d) < 8 {
			return "", "Informe um telefone com pelo menos 8 dígitos."
		}
		return d, ""
	case "email":
		if !strings.Contains(valor, "@") || len(valor) < 5 {
			return "", "Informe um e-mail válido."
		}
		return strings.ToLower(valor), ""
	}
	return "", "Escolha o tipo de identificador."
}

func (h *handlers) anonimizarPagina(w http.ResponseWriter, r *http.Request) {
	h.renderAnonimizar(w, r, views.AnonimizarData{Tipo: "cpf"}, http.StatusOK)
}

func (h *handlers) renderAnonimizar(w http.ResponseWriter, r *http.Request, d views.AnonimizarData, status int) {
	d.CSRF = csrfToken(r.Context())
	lay := h.layout(r, "Anonimização (LGPD)", "/lgpd/anonimizar")
	w.WriteHeader(status)
	render(w, r, views.Anonimizar(lay, d))
}

func resumoAchados(ach []achadoLGPD) []views.AnonimizarAchado {
	var out []views.AnonimizarAchado
	for _, a := range ach {
		out = append(out, views.AnonimizarAchado{Rotulo: a.Alvo.Rotulo, Total: len(a.IDs)})
	}
	return out
}

func (h *handlers) anonimizarBuscar(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	tipo := r.PostFormValue("tipo")
	d := views.AnonimizarData{Tipo: tipo}
	valor, msg := normalizarTitular(tipo, r.PostFormValue("valor"))
	if msg != "" {
		d.Erro = msg
		h.renderAnonimizar(w, r, d, http.StatusBadRequest)
		return
	}
	_, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	ach, err := buscarTitular(r.Context(), pool, tipo, valor)
	if err != nil {
		h.falhaInterna(w, r, "buscar titular", err)
		return
	}
	d.Buscou, d.Valor, d.Achados = true, valor, resumoAchados(ach)
	h.renderAnonimizar(w, r, d, http.StatusOK)
}

func (h *handlers) anonimizarConfirmar(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	tipo := r.PostFormValue("tipo")
	d := views.AnonimizarData{Tipo: tipo}
	valor, msg := normalizarTitular(tipo, r.PostFormValue("valor"))
	motivo := strings.TrimSpace(r.PostFormValue("motivo"))
	if msg == "" && (len(motivo) < 5 || len(motivo) > 300) {
		msg = "Informe o motivo (de 5 a 300 caracteres), por exemplo o número do pedido do titular."
	}
	if msg == "" && r.PostFormValue("confirmo") != "1" {
		msg = "Marque a confirmação: a anonimização não tem volta."
	}
	if msg != "" {
		d.Erro = msg
		h.renderAnonimizar(w, r, d, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	_, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar anonimização", err)
		return
	}
	defer tx.Rollback(ctx)
	ach, err := buscarTitular(ctx, tx, tipo, valor)
	if err != nil {
		h.falhaInterna(w, r, "buscar titular", err)
		return
	}
	detalhes := map[string]any{"tipo": tipo, "motivo": motivo}
	total := 0
	for _, a := range ach {
		if _, err := tx.Exec(ctx, "UPDATE "+a.Alvo.Tabela+" SET "+a.Alvo.Limpar+" WHERE id = ANY($1)", a.IDs); err != nil {
			h.falhaInterna(w, r, "anonimizar "+a.Alvo.Tabela, err)
			return
		}
		detalhes[a.Alvo.Tabela] = len(a.IDs)
		total += len(a.IDs)
	}
	if err := tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "gravar anonimização", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "lgpd.anonimizar", Entidade: "lgpd", Rotulo: "Anonimização por " + tipo, Detalhes: detalhes})
	d.Feito, d.Total, d.Achados = true, total, resumoAchados(ach)
	h.renderAnonimizar(w, r, d, http.StatusOK)
}
