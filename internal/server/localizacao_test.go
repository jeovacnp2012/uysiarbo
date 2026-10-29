package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/geocode"
)

// geocodeFalso responde sem internet e guarda o que lhe pediram.
type geocodeFalso struct {
	mu        sync.Mutex
	cep       geocode.Endereco
	cepErr    error
	pos       geocode.Posicao
	posErr    error
	consultas []geocode.Consulta
}

func (g *geocodeFalso) PorCEP(_ context.Context, cep string) (geocode.Endereco, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.cep, g.cepErr
}

func (g *geocodeFalso) Localizar(_ context.Context, c geocode.Consulta) (geocode.Posicao, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.consultas = append(g.consultas, c)
	return g.pos, g.posErr
}

func jsonDe(t *testing.T, corpo string) respostaLocalizacao {
	t.Helper()
	var r respostaLocalizacao
	if err := json.Unmarshal([]byte(corpo), &r); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, corpo)
	}
	return r
}

func TestBuscarEnderecoPeloCEP(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Jardim São José")
	m.geo.cep = geocode.Endereco{CEP: "78450-000", Logradouro: "Avenida das Palmeiras", Bairro: "JARDIM SAO JOSE", Cidade: "Nova Mutum", UF: "MT"}

	rec := ana.get("/locais/cep?cep=78450-000")
	r := jsonDe(t, rec.Body.String())
	if rec.Code != http.StatusOK || !r.Encontrado || r.Logradouro != "Avenida das Palmeiras" || r.Cidade != "Nova Mutum" || r.UF != "MT" {
		t.Fatalf("cep: %d %+v", rec.Code, r)
	}
	if r.BairroID != b {
		t.Errorf("o bairro cadastrado (com acento e caixa diferentes) deveria ser sugerido: %d, queria %d", r.BairroID, b)
	}

	m.geo.cep.Bairro = "Bairro Que Não Existe"
	if r := jsonDe(t, ana.get("/locais/cep?cep=78450-000").Body.String()); r.BairroID != 0 || !r.Encontrado {
		t.Errorf("bairro desconhecido: %+v", r)
	}
	m.geo.cepErr = geocode.ErrNaoEncontrado
	if r := jsonDe(t, ana.get("/locais/cep?cep=1").Body.String()); r.Encontrado || !strings.Contains(r.Mensagem, "CEP não encontrado") {
		t.Errorf("CEP desconhecido: %+v", r)
	}
	m.geo.cepErr = geocode.ErrIndisponivel
	rec = ana.get("/locais/cep?cep=78450-000")
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(jsonDe(t, rec.Body.String()).Mensagem, "não respondeu") {
		t.Errorf("serviço fora: %d %s", rec.Code, rec.Body.String())
	}
	m.geo.cepErr = geocode.ErrDesligado
	if r := jsonDe(t, ana.get("/locais/cep?cep=78450-000").Body.String()); !r.Desligado {
		t.Errorf("desligado: %+v", r)
	}
	quer(t, m.comCarla(t).get("/locais/cep?cep=78450-000"), http.StatusForbidden)
}

func TestLocalizarEnderecoNoMapa(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Centro")
	m.geo.pos = geocode.Posicao{Latitude: -13.82, Longitude: -56.08, Precisao: "endereco"}
	pedido := "/locais/geocodificar?endereco=" + url.QueryEscape("Rua das Palmeiras") + "&numero=120&bairro_id=" + s64(b)

	r := jsonDe(t, ana.get(pedido).Body.String())
	if !r.Encontrado || r.Latitude == nil || *r.Latitude != -13.82 || *r.Longitude != -56.08 || r.Precisao != "endereco" || r.Dentro != nil {
		t.Fatalf("localizar: %+v", r)
	}
	// A consulta leva rua, número, bairro, cidade e UF do cadastro (e nada de dados pessoais).
	c := m.geo.consultas[len(m.geo.consultas)-1]
	if c.Logradouro != "Rua das Palmeiras" || c.Numero != "120" || c.Bairro != "Centro" || c.Cidade != "Nova Mutum" || c.UF != "MT" {
		t.Errorf("consulta: %+v", c)
	}

	// Com o limite do bairro gravado, avisa se o ponto cai dentro ou fora.
	quer(t, ana.post("/bairros/"+s64(b)+"/limite", url.Values{"acao": {"salvar"}, "geojson": {poligonoBairro}}), http.StatusSeeOther)
	if r := jsonDe(t, ana.get(pedido).Body.String()); r.Dentro == nil || !*r.Dentro {
		t.Errorf("o ponto está dentro do bairro: %+v", r)
	}
	m.geo.pos = geocode.Posicao{Latitude: -10, Longitude: -50, Precisao: "rua"}
	if r := jsonDe(t, ana.get(pedido).Body.String()); r.Dentro == nil || *r.Dentro || r.Precisao != "rua" {
		t.Errorf("o ponto está fora do bairro: %+v", r)
	}

	// Pedidos incompletos ou sem resultado.
	if r := jsonDe(t, ana.get("/locais/geocodificar?endereco=Rua&numero=1").Body.String()); r.Encontrado || !strings.Contains(r.Mensagem, "Escolha o bairro") {
		t.Errorf("sem bairro: %+v", r)
	}
	if r := jsonDe(t, ana.get("/locais/geocodificar?numero=1&bairro_id="+s64(b)).Body.String()); r.Encontrado || !strings.Contains(r.Mensagem, "rua ou avenida") {
		t.Errorf("sem rua: %+v", r)
	}
	m.geo.posErr = geocode.ErrNaoEncontrado
	if r := jsonDe(t, ana.get(pedido).Body.String()); r.Encontrado || !strings.Contains(r.Mensagem, "Não achei") {
		t.Errorf("não achou: %+v", r)
	}
	m.geo.posErr = geocode.ErrIndisponivel
	if rec := ana.get(pedido); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("serviço fora: %d", rec.Code)
	}
	quer(t, m.comCarla(t).get(pedido), http.StatusForbidden)
}

func TestLocalizarRespeitaEscopoDoBairro(t *testing.T) {
	m := novoMundo(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	a, b := m.criarBairro(t, cid, "Bairro A"), m.criarBairro(t, cid, "Bairro B")
	m.geo.pos = geocode.Posicao{Latitude: -13.82, Longitude: -56.08, Precisao: "endereco"}
	pd, _ := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Campo"})
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(pd.ID, "local_visualizar", "local_adicionar", "local_meubairro"))
	dora := m.novoUsuarioLogado(t, "Dora Reis", "dora@nm.gov.br", pd.ID)
	did, _ := m.cliQ.GetUsuarioByEmail(t.Context(), "dora@nm.gov.br")
	_ = m.cliQ.AddResponsaveisAoBairro(t.Context(), addRespParams(a, did.ID))
	quer(t, dora.get("/locais/geocodificar?endereco=Rua&numero=1&bairro_id="+s64(a)), http.StatusOK)
	quer(t, dora.get("/locais/geocodificar?endereco=Rua&numero=1&bairro_id="+s64(b)), http.StatusNotFound)
}

func TestBotoesDeLocalizacaoNoFormulario(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	quer(t, ana.get("/locais/novo"), http.StatusOK, `id="btn-cep"`, `id="btn-localizar"`, "Buscar pelo CEP", "Localizar no mapa", "/static/js/localizacao.js", `id="geo-status"`)
	if rec := ana.get("/static/js/localizacao.js"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "/locais/geocodificar") {
		t.Errorf("localizacao.js: %d", rec.Code)
	}
	// Desligada, os botões somem.
	if (&handlers{geocoder: geocode.Desligado{}}).localizacaoLigada() {
		t.Error("com o provedor desligado, os botões não deveriam aparecer")
	}
	if !(&handlers{geocoder: &geocodeFalso{}}).localizacaoLigada() {
		t.Error("com um provedor, os botões deveriam aparecer")
	}
}

func TestChaveDeNome(t *testing.T) {
	for entrada, esperado := range map[string]string{"Jardim São José": "jardim sao jose", "  CENTRO  ": "centro", "Vila  Ação": "vila acao"} {
		if chaveDeNome(entrada) != esperado {
			t.Errorf("chaveDeNome(%q) = %q, queria %q", entrada, chaveDeNome(entrada), esperado)
		}
	}
}
