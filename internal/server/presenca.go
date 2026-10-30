package server

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"uysiarbo/internal/presenca"
	"uysiarbo/internal/tenant"
	"uysiarbo/web/views"
)

// recursoPresenca descreve um tipo de registro que mostra "quem mais está aqui": a permissão para ver,
// a para editar e se vive dentro de um cliente (ou no console). Tela de ficha nova = linha nova aqui
// e um @views.Presenca(...) na tela.
type recursoPresenca struct {
	ver, editar string
	noCliente   bool
}

var recursosPresenca = map[string]recursoPresenca{
	"bairro":         {ver: "bairro_visualizar", editar: "bairro_adicionar", noCliente: true},
	"grupo":          {ver: "local_grupo_visualizar", editar: "local_grupo_gerenciar", noCliente: true},
	"local":          {ver: "local_visualizar", editar: "local_adicionar", noCliente: true},
	"cidade":         {ver: "bairro_visualizar", editar: "bairro_adicionar", noCliente: true},
	"ciclo":          {ver: "param_ciclo", editar: "param_ciclo", noCliente: true},
	"visita":         {ver: "visita_visualizar", editar: "visita_editar", noCliente: true},
	"reconhecimento": {ver: "reconhecimento_visualizar", editar: "reconhecimento_adicionar", noCliente: true},
	"lei":            {ver: "param_leis", editar: "param_leis", noCliente: true},
	"doenca":         {ver: "param_doenca", editar: "param_doenca", noCliente: true},
	"entrevista":     {ver: "param_doenca", editar: "param_doenca", noCliente: true},
	"investigacao":   {ver: "investigacao_visualizar", editar: "investigacao_editar", noCliente: true},
	"denuncia":       {ver: "denuncia_visualizar", editar: "denuncia_editar", noCliente: true},
	"ovitrampa":      {ver: "ovitrampa_visualizar", editar: "ovitrampa_editar", noCliente: true},
	"edl":            {ver: "edl_visualizar", editar: "edl_editar", noCliente: true},
	"baap":           {ver: "zoonose_baap_visualizar", editar: "zoonose_baap_editar", noCliente: true},
	"campanha":       {ver: "campanha_visualizar", editar: "campanha_editar", noCliente: true},
	"drone":          {ver: "dronesolicitacao_visualizar", editar: "dronesolicitacao_atender", noCliente: true},
	"notificacao":    {ver: "notificacao_visualizar", editar: "notificacao_adicionar", noCliente: true},
	"infracao":       {ver: "notificacao_visualizar", editar: "notificacao_criarauto", noCliente: true},
	"usuario":        {ver: "conf_usuario", editar: "conf_usuario", noCliente: true},
	"papel":          {ver: "conf_papel", editar: "conf_papel", noCliente: true},
}

var (
	reIDPresenca  = regexp.MustCompile(`^[0-9a-z-]{1,40}$`)
	reAbaPresenca = regexp.MustCompile(`^[0-9A-Za-z]{8,64}$`)
)

// escopoPresenca separa os clientes entre si (e a plataforma): o mesmo bairro nº 1 de dois clientes
// são registros diferentes.
func escopoPresenca(r *http.Request) string {
	if c, ok := tenant.FromContext(r.Context()); ok {
		return "cliente:" + c.Slug
	}
	return "plataforma"
}

// presencaPedido valida o que veio na requisição e confere se a pessoa pode ver aquele registro.
// Em caso de recusa já responde e devolve ok = false.
func (h *handlers) presencaPedido(w http.ResponseWriter, r *http.Request, valores func(string) string) (recurso, id, aba string, rec recursoPresenca, ok bool) {
	a, _ := authFrom(r.Context())
	recurso, id, aba = valores("recurso"), valores("id"), valores("aba")
	rec, existe := recursosPresenca[recurso]
	if !existe || !reIDPresenca.MatchString(id) || !reAbaPresenca.MatchString(aba) || a.sess == nil {
		http.Error(w, "Pedido inválido.", http.StatusBadRequest)
		return
	}
	if _, emCliente := tenant.FromContext(r.Context()); emCliente != rec.noCliente {
		http.NotFound(w, r)
		return
	}
	if !a.sess.Principal.Pode(rec.ver) {
		http.Error(w, "Sem permissão.", http.StatusForbidden)
		return
	}
	// Quem só vê os bairros atribuídos não pode espiar quem está num bairro que não é seu.
	if recurso == "bairro" {
		n, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			http.Error(w, "Pedido inválido.", http.StatusBadRequest)
			return
		}
		q, _, err := h.cliente(r)
		if err != nil {
			h.falhaInterna(w, r, "pool do cliente", err)
			return
		}
		visivel, err := h.bairroVisivel(r.Context(), r, q, n)
		if err != nil {
			h.falhaInterna(w, r, "escopo do bairro", err)
			return
		}
		if !visivel {
			http.NotFound(w, r)
			return
		}
	}
	return recurso, id, aba, rec, true
}

// presencaSinal registra que a pessoa está no registro e devolve a faixa com quem mais está.
// É chamada pela própria tela, a cada 15 segundos.
func (h *handlers) presencaSinal(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	recurso, id, aba, rec, ok := h.presencaPedido(w, r, q.Get)
	if !ok {
		return
	}
	a, _ := authFrom(r.Context())
	modo := presenca.Ver
	if q.Get("modo") == "editar" && a.sess.Principal.Pode(rec.editar) {
		modo = presenca.Editar
	}
	eu := presenca.Pessoa{Tipo: string(a.sess.Conta.Tipo), ID: a.sess.Conta.ID, Nome: nomeDaConta(a.sess.Conta.Nome, a.sess.Conta.Email)}
	escopo := escopoPresenca(r)
	h.presenca.Sinal(escopo, recurso, id, aba, eu, modo)

	frase, alerta := frasePresenca(h.presenca.Outros(escopo, recurso, id, aba), eu, modo, h.fuso(r))
	w.Header().Set("Cache-Control", "no-store")
	render(w, r, views.PresencaFaixa(views.PresencaData{Recurso: recurso, ID: id, Modo: string(modo), Frase: frase, Alerta: alerta}))
}

// presencaSair tira a janela do registro quando a tela é fechada.
func (h *handlers) presencaSair(w http.ResponseWriter, r *http.Request) {
	naoAuditar(r) // não é uma ação do usuário sobre os dados
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	recurso, id, aba, _, ok := h.presencaPedido(w, r, r.PostFormValue)
	if !ok {
		return
	}
	h.presenca.Sair(escopoPresenca(r), recurso, id, aba)
	w.WriteHeader(http.StatusNoContent)
}

func nomeDaConta(nome, email string) string {
	if strings.TrimSpace(nome) != "" {
		return nome
	}
	return email
}

// frasePresenca monta o aviso: "Este registro está sendo editado por A (desde 14:35) e visualizado por
// B (desde 14:30)." Devolve também se há alguém editando (o aviso fica em destaque).
func frasePresenca(outros []presenca.Presenca, eu presenca.Pessoa, meuModo presenca.Modo, loc *time.Location) (string, bool) {
	var editando, vendo []string
	for _, p := range outros {
		nome := p.Pessoa.Nome
		if p.Pessoa.ID == eu.ID && p.Pessoa.Tipo == eu.Tipo {
			nome = "você, em outra janela"
		}
		texto := fmt.Sprintf("%s (desde %s)", nome, p.Desde.In(loc).Format("15:04"))
		if p.Modo == presenca.Editar {
			editando = append(editando, texto)
		} else {
			vendo = append(vendo, texto)
		}
	}
	var partes []string
	if len(editando) > 0 {
		partes = append(partes, "editado por "+listaEmPortugues(editando))
	}
	if len(vendo) > 0 {
		partes = append(partes, "visualizado por "+listaEmPortugues(vendo))
	}
	if len(partes) == 0 {
		return "", false
	}
	frase := "Este registro está sendo " + strings.Join(partes, " e ") + "."
	if len(editando) > 0 && meuModo == presenca.Editar {
		frase += " Cuidado: se você salvar, pode sobrescrever o que a outra pessoa alterou."
	}
	return frase, len(editando) > 0
}

// listaEmPortugues junta itens como "A, B e C".
func listaEmPortugues(itens []string) string {
	switch len(itens) {
	case 0:
		return ""
	case 1:
		return itens[0]
	}
	return strings.Join(itens[:len(itens)-1], ", ") + " e " + itens[len(itens)-1]
}
