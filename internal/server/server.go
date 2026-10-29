// Package server monta o roteador HTTP da aplicação.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/geocode"
	"uysiarbo/internal/httpx"
	"uysiarbo/internal/presenca"
	"uysiarbo/internal/tenant"
	"uysiarbo/web"
	"uysiarbo/web/views"
)

// Pinger verifica se uma dependência (o banco) está respondendo.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps reúne o que o servidor precisa para atender.
type Deps struct {
	Log      *slog.Logger
	DB       Pinger
	Catalog  Catalog          // catálogo (schema public)
	Resolver *tenant.Resolver // nil = sem clientes: tudo é console (usado em testes simples)
	Pools    *tenant.Pools    // conexões por schema de cliente

	// Login. Sem Public, as páginas não exigem login (usado só em testes simples).
	Public        *sqlc.Queries
	Captcha       Captcha // nil = sem captcha
	RecaptchaSite string  // chave pública do reCAPTCHA (vai para a tela de entrar)
	Producao      bool    // cookies sempre Secure

	// Mapa base (blocos de imagem). Vazio = OpenStreetMap.
	MapTileURL     string
	MapAttribution string

	// Localização automática de endereços (CEP e coordenadas). nil = desligada.
	Geocoder geocode.Geocoder

	// Tenants permite ao console criar clientes e masters (nil = telas do console desligadas).
	Tenants *tenant.Service
}

type handlers struct {
	log     *slog.Logger
	cat     Catalog
	pools   *tenant.Pools
	pub     *sqlc.Queries
	captcha Captcha
	siteKey string
	prod    bool

	mapaTiles, mapaCredito string
	geocoder               geocode.Geocoder

	tenants  *tenant.Service
	resolver *tenant.Resolver

	vmu    sync.Mutex
	vatual versaoAtual

	presenca *presenca.Registro
}

func New(d Deps) http.Handler {
	views.AssetVersion = web.Version()
	h := &handlers{log: d.Log, cat: d.Catalog, pools: d.Pools, pub: d.Public, captcha: d.Captcha, siteKey: d.RecaptchaSite, prod: d.Producao, tenants: d.Tenants, resolver: d.Resolver}
	if h.captcha == nil {
		h.captcha = SemCaptcha{}
	}
	h.mapaTiles, h.mapaCredito = d.MapTileURL, d.MapAttribution
	if h.geocoder = d.Geocoder; h.geocoder == nil {
		h.geocoder = geocode.Desligado{}
	}
	if h.mapaTiles == "" {
		h.mapaTiles, h.mapaCredito = "https://tile.openstreetmap.org/{z}/{x}/{y}.png", "© contribuidores do OpenStreetMap"
	}
	h.presenca = &presenca.Registro{}

	r := chi.NewRouter()
	r.Use(securityHeaders, requestID, middleware.RealIP, accessLog(d.Log), recoverer(d.Log))
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(tenancy(d.Log, d.Resolver), csrf, h.autenticacao, h.auditoriaGenerica)

	r.Get("/healthz", health)
	r.Get("/readyz", ready(d.Log, d.DB))

	r.Handle("/static/*", http.StripPrefix("/static/", staticHandler()))

	r.Get("/entrar", h.entrarForm)
	r.Post("/entrar", h.entrarPost)
	r.Post("/sair", h.sair)
	r.Get("/trocar-senha", h.trocarSenhaForm)
	r.Post("/trocar-senha", h.trocarSenhaPost)

	r.Get("/papeis", h.exigir("conf_papel", h.papeisLista))
	r.Get("/papeis/novo", h.exigir("conf_papel", h.papelNovoForm))
	r.Post("/papeis", h.exigir("conf_papel", h.papelSalvar))
	r.Get("/papeis/{id}", h.exigir("conf_papel", h.papelEditarForm))
	r.Post("/papeis/{id}", h.exigir("conf_papel", h.papelSalvar))

	r.Get("/usuarios", h.exigir("conf_usuario", h.usuariosLista))
	r.Get("/usuarios/novo", h.exigir("conf_usuario", h.usuarioNovoForm))
	r.Post("/usuarios", h.exigir("conf_usuario", h.usuarioCriar))
	r.Get("/usuarios/{id}", h.exigir("conf_usuario", h.usuarioEditarForm))
	r.Post("/usuarios/{id}", h.exigir("conf_usuario", h.usuarioSalvar))
	r.Post("/usuarios/{id}/resetar-senha", h.exigir("conf_usuario", h.usuarioResetarSenha))

	r.Get("/auditoria", h.auditoriaPagina)
	r.Get("/presenca", h.presencaSinal)
	r.Post("/presenca/sair", h.presencaSair)

	r.Get("/cidades", h.exigir("bairro_visualizar", h.cidadesLista))
	r.Get("/cidades/nova", h.exigir("bairro_adicionar", h.cidadeNovaForm))
	r.Post("/cidades", h.exigir("bairro_adicionar", h.cidadeSalvar))
	r.Get("/cidades/{id}", h.exigir("bairro_visualizar", h.cidadeEditarForm))
	r.Post("/cidades/{id}", h.exigir("bairro_adicionar", h.cidadeSalvar))

	r.Get("/mapa", h.mapaPagina)
	r.Get("/mapa/bairros.geojson", h.exigir("bairro_visualizar", h.mapaBairros))
	r.Get("/mapa/quarteiroes.geojson", h.exigir("bairro_visualizar", h.mapaQuarteiroes))
	r.Get("/mapa/locais.geojson", h.exigir("local_visualizar", h.mapaLocais))

	r.Get("/grupos", h.exigir("local_grupo_visualizar", h.gruposLista))
	r.Get("/grupos/novo", h.exigir("local_grupo_gerenciar", h.grupoNovoForm))
	r.Get("/grupos/quarteiroes", h.exigir("local_grupo_gerenciar", h.grupoQuarteiroes))
	r.Post("/grupos", h.exigir("local_grupo_gerenciar", h.grupoSalvar))
	r.Get("/grupos/{id}", h.exigir("local_grupo_visualizar", h.grupoFicha))
	r.Post("/grupos/{id}", h.exigir("local_grupo_gerenciar", h.grupoSalvar))
	r.Get("/grupos/{id}/exportar.csv", h.exigir("local_grupo_visualizar", h.grupoExportar))
	r.Post("/grupos/{id}/excluir", h.exigir("local_grupo_gerenciar", h.grupoExcluir))
	r.Post("/grupos/{id}/clonar", h.exigir("local_grupo_gerenciar", h.grupoClonar))
	r.Post("/grupos/{id}/limpar", h.exigir("local_grupo_gerenciar", h.grupoLimpar))
	r.Post("/grupos/{id}/locais", h.exigir("local_grupo_gerenciar", h.grupoAdicionar))
	r.Post("/grupos/{id}/locais/{lid}/remover", h.exigir("local_grupo_gerenciar", h.grupoRemoverLocal))

	r.Get("/locais", h.exigir("local_visualizar", h.locaisLista))
	r.Get("/locais/novo", h.exigir("local_adicionar", h.localNovoForm))
	r.Get("/locais/quarteiroes", h.exigir("local_visualizar", h.localQuarteiroes))
	r.Post("/locais", h.exigir("local_adicionar", h.localSalvar))
	r.Get("/locais/cep", h.exigir("local_adicionar", h.localCEP))
	r.Get("/locais/geocodificar", h.exigir("local_adicionar", h.localGeocodificar))
	r.Get("/locais/lote", h.exigir("local_adicionar", h.loteForm))
	r.Post("/locais/lote", h.exigir("local_adicionar", h.loteSalvar))
	r.Get("/locais/{id}", h.exigir("local_visualizar", h.localEditarForm))
	r.Get("/locais/{id}/qr.png", h.exigir("local_visualizar", h.localQR))
	r.Post("/locais/{id}/ativo", h.exigir("local_desativar", h.localAtivo))
	r.Post("/locais/{id}/grupo", h.exigir("local_grupo_gerenciar", h.localNoGrupo))
	r.Post("/locais/{id}/chave", h.exigir("local_adicionar", h.localNovaChave))
	r.Post("/locais/{id}", h.exigir("local_adicionar", h.localSalvar))

	r.Get("/bairros", h.exigir("bairro_visualizar", h.bairrosLista))
	r.Get("/bairros/novo", h.exigir("bairro_adicionar", h.bairroNovoForm))
	r.Post("/bairros", h.exigir("bairro_adicionar", h.bairroSalvar))
	r.Get("/bairros/{id}", h.exigir("bairro_visualizar", h.bairroEditarForm))
	r.Post("/bairros/{id}", h.exigir("bairro_adicionar", h.bairroSalvar))
	r.Post("/bairros/{id}/limite", h.exigir("bairro_adicionar", h.bairroLimite))
	r.Post("/bairros/{id}/quarteiroes", h.exigir("bairro_adicionar", h.quarteiraoCriar))
	r.Post("/bairros/{id}/quarteiroes/limites", h.exigir("bairro_adicionar", h.quarteiraoLimites))
	r.Post("/bairros/{id}/quarteiroes/{qid}/ativo", h.exigir("bairro_adicionar", h.quarteiraoAtivo))

	r.Get("/guia", h.guiaLista)
	r.Get("/guia/{slug}", h.guiaArtigo)
	r.Get("/novidades", h.novidades)
	r.Get("/novidades/nova", h.somenteMaster(h.versaoNovaForm))
	r.Post("/novidades", h.somenteMaster(h.versaoCriar))

	r.Get("/clientes", h.somenteMaster(h.clientesLista))
	r.Get("/clientes/novo", h.somenteMaster(h.clienteNovoForm))
	r.Post("/clientes", h.somenteMaster(h.clienteCriar))
	r.Post("/clientes/{slug}/status", h.somenteMaster(h.clienteStatus))
	r.Get("/clientes/{slug}/editar", h.somenteMaster(h.clienteEditarForm))
	r.Post("/clientes/{slug}/editar", h.somenteMaster(h.clienteEditarSalvar))
	r.Get("/plataforma/usuarios", h.somenteMaster(h.mastersLista))
	r.Get("/plataforma/usuarios/novo", h.somenteMaster(h.masterNovoForm))
	r.Post("/plataforma/usuarios", h.somenteMaster(h.masterCriar))
	r.Post("/plataforma/usuarios/{id}/resetar-senha", h.somenteMaster(h.masterResetarSenha))

	r.Get("/", h.home)
	r.Get("/componentes", h.components)
	r.Get("/componentes/tabela", h.componentsTable)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, http.StatusNotFound, "not_found", "Página ou recurso não encontrado.")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Método não permitido.")
	})
	return r
}

func staticHandler() http.Handler {
	files := http.FileServerFS(web.Static())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		files.ServeHTTP(w, r)
	})
}

// render escreve um componente templ como HTML (status 200).
func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	renderStatus(w, r, http.StatusOK, c)
}

// renderStatus escreve um componente templ como HTML com o status dado.
func renderStatus(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := c.Render(r.Context(), w); err != nil {
		slog.Error("renderizar página", "erro", err, "request_id", r.Header.Get("X-Request-Id"))
	}
}

// health indica apenas que o processo está de pé.
func health(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ready indica que a aplicação consegue atender: o banco precisa responder.
func ready(log *slog.Logger, db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			log.Error("banco indisponível", "erro", err, "request_id", r.Header.Get("X-Request-Id"))
			httpx.Error(w, r, http.StatusServiceUnavailable, "db_unavailable", "Banco de dados indisponível.")
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "db": "ok"})
	}
}
