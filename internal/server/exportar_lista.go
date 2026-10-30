package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/relatorio"
	"uysiarbo/web"
)

// exportarTudo é o "tamanho de página" interno usado para gerar o arquivo com todos os registros filtrados.
const exportarTudo = relatorio.MaxLinhas

var reEspacos = regexp.MustCompile(`\s+`)

// exportarLista dá a toda listagem a saída em PDF e planilha: com ?formato=pdf|xlsx, a página é gerada
// normalmente (mesmos filtros, mesma ordem e as permissões de sempre), porém com todos os registros numa só página;
// a tabela principal vira o arquivo, só com as colunas pedidas em ?colunas= (as visíveis na tela).
func (h *handlers) exportarLista(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		formato := r.URL.Query().Get("formato")
		if r.Method != http.MethodGet || (formato != "pdf" && formato != "xlsx") ||
			strings.HasPrefix(r.URL.Path, "/relatorios") || strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		qs := r.URL.Query()
		colunas := qs.Get("colunas")
		qs.Del("formato")
		qs.Del("colunas")
		qs.Del("pagina")
		qs.Set("por_pagina", "20000")
		interno := r.Clone(r.Context())
		u := *r.URL
		u.RawQuery = qs.Encode()
		interno.URL = &u
		interno.RequestURI = u.RequestURI()
		interno.Header = r.Header.Clone()
		interno.Header.Del("HX-Request")
		interno.Header.Del("Accept-Encoding")

		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, interno)
		corpo := rec.Body.Bytes()
		if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			for k, v := range rec.Header() {
				w.Header()[k] = v
			}
			w.WriteHeader(rec.Code)
			_, _ = w.Write(corpo)
			return
		}
		doc, ok := h.docDaPagina(r, corpo, qs, colunas)
		if !ok {
			http.Error(w, "Esta tela não tem uma tabela para exportar.", http.StatusNotFound)
			return
		}
		nome := strings.Trim(strings.ReplaceAll(r.URL.Path, "/", "-"), "-")
		if nome == "" {
			nome = "lista"
		}
		nome += "-" + time.Now().In(h.fuso(r)).Format("20060102-1504")
		var err error
		if formato == "pdf" {
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", `attachment; filename="`+nome+`.pdf"`)
			err = doc.PDF(w)
		} else {
			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
			w.Header().Set("Content-Disposition", `attachment; filename="`+nome+`.xlsx"`)
			err = doc.XLSX(w)
		}
		if err != nil {
			h.log.Error("exportar lista", "caminho", r.URL.Path, "formato", formato, "erro", err)
			return
		}
		h.auditar(r, audit.Evento{Acao: "relatorio.baixar", Entidade: "lista", EntidadeID: r.URL.Path, Rotulo: doc.Titulo,
			Detalhes: map[string]any{"formato": formato, "linhas": len(doc.Linhas)}})
	})
}

func textoHTML(n *html.Node) string {
	var b strings.Builder
	var anda func(*html.Node)
	anda = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "svg" || n.Data == "script" || n.Data == "style" || temClasseHTML(n, "sr-only") {
				return
			}
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			anda(c)
		}
	}
	anda(n)
	t := strings.TrimSpace(reEspacos.ReplaceAllString(b.String(), " "))
	return strings.TrimSpace(strings.NewReplacer("▲", "", "▼", "").Replace(t))
}

func atributoHTML(n *html.Node, nome string) string {
	for _, a := range n.Attr {
		if a.Key == nome {
			return a.Val
		}
	}
	return ""
}

func temClasseHTML(n *html.Node, classe string) bool {
	for _, c := range strings.Fields(atributoHTML(n, "class")) {
		if c == classe {
			return true
		}
	}
	return false
}

func acharNos(n *html.Node, ok func(*html.Node) bool, saida *[]*html.Node) {
	if n.Type == html.ElementNode && ok(n) {
		*saida = append(*saida, n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		acharNos(c, ok, saida)
	}
}

// docDaPagina lê a primeira tabela da listagem e monta o documento com as colunas pedidas.
func (h *handlers) docDaPagina(r *http.Request, corpo []byte, qs url.Values, colunas string) (relatorio.Doc, bool) {
	raiz, err := html.Parse(bytes.NewReader(corpo))
	if err != nil {
		return relatorio.Doc{}, false
	}
	var tabelas []*html.Node
	acharNos(raiz, func(n *html.Node) bool { return n.Data == "table" && temClasseHTML(n, "tabela") }, &tabelas)
	if len(tabelas) == 0 {
		return relatorio.Doc{}, false
	}
	tab := tabelas[0]
	var ths []*html.Node
	var cab []*html.Node
	acharNos(tab, func(n *html.Node) bool { return n.Data == "thead" }, &cab)
	if len(cab) == 0 {
		return relatorio.Doc{}, false
	}
	acharNos(cab[0], func(n *html.Node) bool { return n.Data == "th" }, &ths)

	quer := map[string]bool{}
	for _, c := range strings.Split(colunas, "||") {
		if c = strings.TrimSpace(c); c != "" {
			quer[c] = true
		}
	}
	var idx []int
	doc := relatorio.Doc{}
	for i, th := range ths {
		t := textoHTML(th)
		if t == "" || strings.EqualFold(t, "Ações") || strings.EqualFold(t, "Marcar") {
			continue
		}
		if len(quer) > 0 && !quer[t] {
			continue
		}
		idx = append(idx, i)
		peso := 2.0
		if len(t) <= 4 {
			peso = 1
		}
		if strings.Contains(strings.ToLower(t), "endere") || strings.Contains(strings.ToLower(t), "agente") {
			peso = 4
		}
		doc.Colunas = append(doc.Colunas, relatorio.Coluna{Titulo: t, Peso: peso})
	}
	if len(idx) == 0 {
		return relatorio.Doc{}, false
	}
	var linhas []*html.Node
	acharNos(tab, func(n *html.Node) bool { return n.Data == "tr" }, &linhas)
	for _, tr := range linhas {
		var tds []*html.Node
		for c := tr.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "td" {
				tds = append(tds, c)
			}
		}
		if len(tds) != len(ths) {
			continue
		}
		linha := make([]string, len(idx))
		for j, i := range idx {
			linha[j] = textoHTML(tds[i])
		}
		doc.Linhas = append(doc.Linhas, linha)
	}
	if len(doc.Linhas) > relatorio.MaxLinhas {
		doc.Linhas, doc.Cortado = doc.Linhas[:relatorio.MaxLinhas], true
	}

	// título, cliente e filtros em uso (rótulos lidos do formulário de filtros da própria tela)
	var h1 []*html.Node
	acharNos(raiz, func(n *html.Node) bool { return n.Data == "h1" }, &h1)
	doc.Titulo = "Lista"
	if len(h1) > 0 {
		doc.Titulo = textoHTML(h1[0])
	}
	var cli []*html.Node
	acharNos(raiz, func(n *html.Node) bool { return temClasseHTML(n, "menu-cliente") }, &cli)
	if len(cli) > 0 {
		doc.Cliente = textoHTML(cli[0])
	}
	doc.Gerado = time.Now().In(h.fuso(r)).Format("02/01/2006 15:04")
	doc.Filtros = filtrosDaPagina(raiz, qs)
	if b, err := web.Static().Open("brand/uysi-symbol.png"); err == nil {
		defer b.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(b)
		doc.Logo = buf.Bytes()
	}
	return doc, true
}

var paramsIgnorados = map[string]bool{"ordem": true, "dir": true, "pagina": true, "por_pagina": true, "csrf": true, "local": true}

func filtrosDaPagina(raiz *html.Node, qs url.Values) []relatorio.Par {
	rotulos := map[string]string{}
	var labels []*html.Node
	acharNos(raiz, func(n *html.Node) bool { return n.Data == "label" && atributoHTML(n, "for") != "" }, &labels)
	for _, l := range labels {
		rotulos[atributoHTML(l, "for")] = textoHTML(l)
	}
	var ctrls []*html.Node
	acharNos(raiz, func(n *html.Node) bool {
		return (n.Data == "input" || n.Data == "select") && atributoHTML(n, "name") != "" && atributoHTML(n, "id") != ""
	}, &ctrls)
	var out []relatorio.Par
	vistos := map[string]bool{}
	for _, c := range ctrls {
		nome := atributoHTML(c, "name")
		v := strings.TrimSpace(qs.Get(nome))
		if v == "" || paramsIgnorados[nome] || vistos[nome] {
			continue
		}
		rot := rotulos[atributoHTML(c, "id")]
		if rot == "" {
			continue
		}
		vistos[nome] = true
		if c.Data == "select" {
			var ops []*html.Node
			acharNos(c, func(n *html.Node) bool { return n.Data == "option" && atributoHTML(n, "value") == v }, &ops)
			if len(ops) > 0 {
				v = textoHTML(ops[0])
			}
		}
		out = append(out, relatorio.Par{Nome: rot, Valor: v})
	}
	return out
}
