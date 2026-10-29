package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/geo"
	"uysiarbo/web/views"
)

// completarLimites preenche, na ficha do bairro, o limite gravado e quais quarteirões já estão no mapa.
func (h *handlers) completarLimites(ctx context.Context, r *http.Request, id int64, form *views.BairroFormData) error {
	_, pool, err := h.cliente(r)
	if err != nil {
		return err
	}
	res, ok, err := geo.BairroResumo(ctx, pool, id)
	if err != nil {
		return err
	}
	if ok {
		form.Limite = &views.LimiteInfo{
			AreaHa:    strconv.FormatFloat(res.AreaHa, 'f', 2, 64),
			Latitude:  strconv.FormatFloat(res.Latitude, 'f', 6, 64),
			Longitude: strconv.FormatFloat(res.Longitude, 'f', 6, 64),
			GeoJSON:   res.GeoJSON,
		}
	}
	com, err := geo.QuarteiroesComLimite(ctx, pool, id)
	if err != nil {
		return err
	}
	for i := range form.Quarteiroes {
		form.Quarteiroes[i].ComLimite = com[form.Quarteiroes[i].ID]
	}
	return nil
}

// bairroParaGeometria carrega o bairro e confere o escopo "apenas atribuídos"; responde 404 quando não pode.
func (h *handlers) bairroParaGeometria(w http.ResponseWriter, r *http.Request) (sqlccliente.Bairro, *sqlccliente.Queries, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.Bairro{}, nil, false
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return sqlccliente.Bairro{}, nil, false
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return sqlccliente.Bairro{}, nil, false
	}
	b, err := q.GetBairro(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return b, nil, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar bairro", err)
		return b, nil, false
	}
	if visivel, err := h.bairroVisivel(r.Context(), r, q, id); err != nil {
		h.falhaInterna(w, r, "escopo do bairro", err)
		return b, nil, false
	} else if !visivel {
		http.NotFound(w, r)
		return b, nil, false
	}
	return b, q, true
}

// falhaLimite mostra a ficha do bairro de novo com a mensagem na seção do limite.
func (h *handlers) falhaLimite(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, b sqlccliente.Bairro, texto, msg string) {
	ctx := r.Context()
	resp, _ := q.ListResponsaveisDoBairro(ctx, b.ID)
	form, err := h.montarFormBairro(ctx, r, q, b.ID, dadosDoBairro(b, resp), "", "")
	if err != nil {
		h.falhaInterna(w, r, "montar formulário", err)
		return
	}
	form.ErroLimite, form.GeoJSONTexto = msg, texto
	renderStatus(w, r, http.StatusBadRequest, views.BairroForm(h.layout(r, "Bairro", "/bairros"), form))
}

func arredondar(v float64) float64 {
	f, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'f', 2, 64), 64)
	return f
}

func (h *handlers) bairroLimite(w http.ResponseWriter, r *http.Request) {
	b, q, ok := h.bairroParaGeometria(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	_, pool, _ := h.cliente(r)
	texto := r.PostFormValue("geojson")
	voltar := func(resultado string) {
		http.Redirect(w, r, fmt.Sprintf("/bairros/%d?limite=%s#limite", b.ID, resultado), http.StatusSeeOther)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(ctx)
	antes, tinha, err := geo.BairroResumo(ctx, tx, b.ID)
	if err != nil {
		h.falhaInterna(w, r, "ler limite", err)
		return
	}

	if r.PostFormValue("acao") == "remover" {
		if !tinha {
			naoAuditar(r)
			voltar("removido")
			return
		}
		if _, err := geo.RemoverBairro(ctx, tx, b.ID); err != nil {
			h.falhaInterna(w, r, "remover limite", err)
			return
		}
		h.auditarCom(audit.Cliente{Q: sqlccliente.New(tx)}, r, audit.Evento{
			Acao: "bairro.limite_remover", Entidade: "bairro", EntidadeID: strconv.FormatInt(b.ID, 10), Rotulo: b.Nome,
			Antes: map[string]any{"area_ha": arredondar(antes.AreaHa)},
		})
		if err := tx.Commit(ctx); err != nil {
			h.falhaInterna(w, r, "confirmar remoção", err)
			return
		}
		voltar("removido")
		return
	}

	feicoes, err := geo.Ler(texto)
	if err != nil {
		h.falhaLimite(w, r, q, b, texto, "GeoJSON recusado: "+err.Error()+".")
		return
	}
	if len(feicoes) != 1 {
		h.falhaLimite(w, r, q, b, texto, fmt.Sprintf("O bairro tem um único limite, e o arquivo trouxe %d polígonos. Para vários quarteirões, use a importação em Quarteirões.", len(feicoes)))
		return
	}
	if err := geo.GravarBairro(ctx, tx, b.ID, feicoes[0].Geometria); errors.Is(err, geo.ErrGeometriaInvalida) {
		_ = tx.Rollback(ctx)
		h.falhaLimite(w, r, q, b, texto, "O polígono é inválido (área vazia ou desenho impossível de consertar). Confira o desenho.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "gravar limite", err)
		return
	}
	depois, _, err := geo.BairroResumo(ctx, tx, b.ID)
	if err != nil {
		h.falhaInterna(w, r, "ler limite gravado", err)
		return
	}
	ev := audit.Evento{
		Acao: "bairro.limite", Entidade: "bairro", EntidadeID: strconv.FormatInt(b.ID, 10), Rotulo: b.Nome,
		Depois: map[string]any{"area_ha": arredondar(depois.AreaHa)},
	}
	if tinha {
		ev.Antes = map[string]any{"area_ha": arredondar(antes.AreaHa)}
	}
	h.auditarCom(audit.Cliente{Q: sqlccliente.New(tx)}, r, ev)
	if err := tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "confirmar limite", err)
		return
	}
	voltar("gravado")
}

// quarteiraoLimites importa os polígonos dos quarteirões de um bairro; cada feição precisa de um nome.
// Quarteirões que ainda não existem são criados. Tudo ou nada: um polígono ruim cancela a importação.
func (h *handlers) quarteiraoLimites(w http.ResponseWriter, r *http.Request) {
	b, q, ok := h.bairroParaGeometria(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	_, pool, _ := h.cliente(r)

	feicoes, err := geo.Ler(r.PostFormValue("geojson"))
	if err != nil {
		h.falhaLimite(w, r, q, b, "", "Importação recusada: "+err.Error()+".")
		return
	}
	vistos := map[string]bool{}
	for i, f := range feicoes {
		nome := strings.TrimSpace(f.Nome)
		switch {
		case nome == "":
			h.falhaLimite(w, r, q, b, "", fmt.Sprintf("O polígono %d não tem a propriedade \"nome\" (ou name, quarteirao, quadra).", i+1))
			return
		case len([]rune(nome)) > 30:
			h.falhaLimite(w, r, q, b, "", fmt.Sprintf("O nome do quarteirão pode ter no máximo 30 caracteres: %q.", nome))
			return
		case vistos[strings.ToLower(nome)]:
			h.falhaLimite(w, r, q, b, "", fmt.Sprintf("O quarteirão %q aparece mais de uma vez no arquivo.", nome))
			return
		}
		vistos[strings.ToLower(nome)] = true
		feicoes[i].Nome = nome
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := sqlccliente.New(tx)
	existentes, err := qt.ListQuarteiroesDoBairro(ctx, b.ID)
	if err != nil {
		h.falhaInterna(w, r, "listar quarteirões", err)
		return
	}
	porNome := map[string]int64{}
	for _, x := range existentes {
		porNome[strings.ToLower(x.Nome)] = x.ID
	}
	criados := 0
	nomes := make([]string, 0, len(feicoes))
	for _, f := range feicoes {
		chave := strings.ToLower(f.Nome)
		if _, existe := porNome[chave]; !existe {
			if _, err := qt.CreateQuarteirao(ctx, sqlccliente.CreateQuarteiraoParams{BairroID: b.ID, Nome: f.Nome}); err != nil {
				h.falhaInterna(w, r, "criar quarteirão", err)
				return
			}
			todos, err := qt.ListQuarteiroesDoBairro(ctx, b.ID)
			if err != nil {
				h.falhaInterna(w, r, "listar quarteirões", err)
				return
			}
			for _, x := range todos {
				porNome[strings.ToLower(x.Nome)] = x.ID
			}
			criados++
		}
		if err := geo.GravarQuarteirao(ctx, tx, porNome[chave], f.Geometria); errors.Is(err, geo.ErrGeometriaInvalida) {
			_ = tx.Rollback(ctx)
			h.falhaLimite(w, r, q, b, "", fmt.Sprintf("O polígono do quarteirão %q é inválido. Nada foi importado.", f.Nome))
			return
		} else if err != nil {
			h.falhaInterna(w, r, "gravar limite do quarteirão", err)
			return
		}
		nomes = append(nomes, f.Nome)
	}
	amostra := nomes
	if len(amostra) > 50 {
		amostra = amostra[:50]
	}
	h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{
		Acao: "quarteirao.limites", Entidade: "bairro", EntidadeID: strconv.FormatInt(b.ID, 10), Rotulo: b.Nome,
		Detalhes: map[string]any{"importados": len(feicoes), "criados": criados, "quarteiroes": amostra},
	})
	if err := tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "confirmar importação", err)
		return
	}
	v := url.Values{"limites": {strconv.Itoa(len(feicoes))}, "novos": {strconv.Itoa(criados)}}
	http.Redirect(w, r, fmt.Sprintf("/bairros/%d?%s#quarteiroes", b.ID, v.Encode()), http.StatusSeeOther)
}
