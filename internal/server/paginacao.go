package server

import (
	"github.com/a-h/templ"
	"net/http"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"uysiarbo/web/views"
)

// padraoPorPagina é o tamanho de página de toda listagem (a pessoa escolhe outro em views.PorPaginaOpcoes).
const padraoPorPagina = 10

// paginaDe lê ?pagina= e ?por_pagina= do pedido. O total e o ajuste da página ficam com quem consulta.
func paginaDe(r *http.Request) views.Pagination {
	p := views.Pagination{Page: 1, PerPage: padraoPorPagina}
	qs := r.URL.Query()
	if n, err := strconv.Atoi(qs.Get("por_pagina")); err == nil && slices.Contains(views.PorPaginaOpcoes, n) || n == exportarTudo {
		p.PerPage = n
	}
	if n, err := strconv.Atoi(qs.Get("pagina")); err == nil && n > 1 {
		p.Page = n
	}
	return p
}

// paginar corta uma lista já carregada na memória e deixa o paginador (em cima e embaixo da tabela)
// no layout. Devolve só os itens da página pedida.
func paginar[T any](r *http.Request, lay *views.Layout, itens []T) []T {
	ordenarPorCampo(r, itens)
	p := paginaDe(r)
	p.Total = len(itens)
	if p.Page > p.Pages() {
		p.Page = p.Pages()
	}
	ini := min((p.Page-1)*p.PerPage, len(itens))
	fim := min(ini+p.PerPage, len(itens))
	lay.Pag = &views.Paginador{Pagination: p, Path: r.URL.Path, Query: r.URL.Query()}
	return itens[ini:fim]
}

// telaAcesso monta a tela do controle de acesso com a lista paginada.
func (h *handlers) telaAcesso(r *http.Request, d views.AcessoData) templ.Component {
	lay := h.layout(r, "Controle de acesso aos locais", "/parametros/acesso")
	d.Linhas = paginar(r, &lay, d.Linhas)
	return views.Acesso(lay, d)
}

var reDataBR = regexp.MustCompile(`^(\d{2})/(\d{2})/(\d{4})(?: (\d{2}):(\d{2}))?`)

// chaveDeOrdem transforma o valor de um campo em algo comparável: textos sem caixa, datas dd/mm/aaaa na ordem do calendário.
func chaveDeOrdem(v reflect.Value) (string, float64, bool) {
	switch v.Kind() {
	case reflect.String:
		t := strings.ToLower(strings.TrimSpace(v.String()))
		if m := reDataBR.FindStringSubmatch(t); m != nil {
			return m[3] + m[2] + m[1] + m[4] + m[5], 0, false
		}
		return t, 0, false
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "", float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "", float64(v.Uint()), true
	case reflect.Float32, reflect.Float64:
		return "", v.Float(), true
	case reflect.Bool:
		if v.Bool() {
			return "", 1, true
		}
		return "", 0, true
	case reflect.Struct:
		if t, ok := v.Interface().(time.Time); ok {
			return "", float64(t.UnixNano()), true
		}
	}
	return "", 0, false
}

// ordenarPorCampo ordena, no lugar, uma lista de structs pelo campo pedido em ?ordem=Campo&dir=asc|desc
// (o nome do campo vem do data-ord do título da coluna). Campo que não existe ou não é comparável é ignorado.
func ordenarPorCampo[T any](r *http.Request, itens []T) {
	nome := r.URL.Query().Get("ordem")
	if nome == "" || len(itens) < 2 {
		return
	}
	t := reflect.TypeOf(itens[0])
	if t.Kind() != reflect.Struct {
		return
	}
	f, ok := t.FieldByName(nome)
	if !ok || !f.IsExported() {
		return
	}
	desc := r.URL.Query().Get("dir") == "desc"
	sort.SliceStable(itens, func(i, j int) bool {
		si, ni, numI := chaveDeOrdem(reflect.ValueOf(itens[i]).FieldByIndex(f.Index))
		sj, nj, _ := chaveDeOrdem(reflect.ValueOf(itens[j]).FieldByIndex(f.Index))
		var menor, maior bool
		if numI {
			menor, maior = ni < nj, ni > nj
		} else {
			menor, maior = si < sj, si > sj
		}
		if desc {
			return maior
		}
		return menor
	})
}
