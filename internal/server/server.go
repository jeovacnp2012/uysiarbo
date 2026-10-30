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

	// Importação por API de outro sistema: por segurança só acessa a internet pública; true libera a rede interna (testes).
	ImportPermitirRedePrivada bool

	// Chave que assina os tokens da API (mínimo 32 bytes). Vazia = a API fica desligada.
	APIKey []byte

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

	mapaTiles, mapaCredito    string
	geocoder                  geocode.Geocoder
	api                       *apiEstado
	imp                       *motorImp
	importPermitirRedePrivada bool

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
	h.imp = &motorImp{}
	h.importPermitirRedePrivada = d.ImportPermitirRedePrivada
	h.api = &apiEstado{chave: d.APIKey, tocadoEm: map[int64]time.Time{}}

	r := chi.NewRouter()
	r.Use(securityHeaders, requestID, middleware.RealIP, accessLog(d.Log), recoverer(d.Log))
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(tenancy(d.Log, d.Resolver), csrf, h.autenticacao, h.auditoriaGenerica, h.exportarLista)

	if len(d.APIKey) >= 32 {
		r.Post("/api/v1/token", h.apiToken)
		r.Get("/api/v1/openapi.yaml", openAPI)
		r.Get("/api/v1/bairros", h.apiAuth("bairros:read", h.apiBairrosLista))
		r.Get("/api/v1/bairros/{id}", h.apiAuth("bairros:read", h.apiBairroObter))
		r.Post("/api/v1/bairros", h.apiAuth("bairros:write", h.idempotente(h.apiBairroCriar)))
		r.Patch("/api/v1/bairros/{id}", h.apiAuth("bairros:write", h.idempotente(h.apiBairroAlterar)))
		r.Get("/api/v1/quarteiroes", h.apiAuth("quarteiroes:read", h.apiQuarteiroesLista))
		r.Get("/api/v1/quarteiroes/{id}", h.apiAuth("quarteiroes:read", h.apiQuarteiraoObter))
		r.Post("/api/v1/quarteiroes", h.apiAuth("quarteiroes:write", h.idempotente(h.apiQuarteiraoCriar)))
		r.Patch("/api/v1/quarteiroes/{id}", h.apiAuth("quarteiroes:write", h.idempotente(h.apiQuarteiraoAlterar)))
		r.Get("/api/v1/locais", h.apiAuth("locais:read", h.apiLocaisLista))
		r.Get("/api/v1/locais/{id}", h.apiAuth("locais:read", h.apiLocalObter))
		r.Post("/api/v1/locais", h.apiAuth("locais:write", h.idempotente(h.apiLocalCriar)))
		r.Patch("/api/v1/locais/{id}", h.apiAuth("locais:write", h.idempotente(h.apiLocalAlterar)))
		r.Post("/api/v1/importacoes", h.apiAuth("importacoes:write", h.idempotente(h.apiImportacaoCriar)))
		r.Get("/api/v1/importacoes/{id}", h.apiAuth("importacoes:read", h.apiImportacaoObter))
		r.Get("/api/v1/importacoes/{id}/linhas", h.apiAuth("importacoes:read", h.apiImportacaoLinhas))
		r.Post("/api/v1/importacoes/{id}/executar", h.apiAuth("importacoes:write", h.idempotente(h.apiImportacaoExecutar)))
		r.Post("/api/v1/importacoes/{id}/descartar", h.apiAuth("importacoes:write", h.idempotente(h.apiImportacaoDescartar)))
		r.Post("/api/v1/sync", h.apiAuth("visitas:write", h.apiSync))
		r.Get("/api/v1/visitas", h.apiAuth("visitas:read", h.apiVisitasLista))
		r.Get("/api/v1/cidades", h.apiAuth("cidades:read", h.apiCidadesLista))
		r.Get("/api/v1/cidades/{id}", h.apiAuth("cidades:read", h.apiCidadeObter))
		r.Post("/api/v1/cidades", h.apiAuth("cidades:write", h.idempotente(h.apiCidadeCriar)))
		r.Patch("/api/v1/cidades/{id}", h.apiAuth("cidades:write", h.idempotente(h.apiCidadeAlterar)))
	}

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

	r.Get("/reconhecimentos", h.exigir("reconhecimento_visualizar", h.reconhecimentosLista))
	r.Get("/reconhecimentos/novo", h.exigir("reconhecimento_adicionar", h.reconhecimentoNovo))
	r.Post("/reconhecimentos", h.exigir("reconhecimento_adicionar", h.reconhecimentoSalvar))
	r.Get("/reconhecimentos/{id}", h.exigir("reconhecimento_visualizar", h.reconhecimentoFicha))
	r.Post("/reconhecimentos/{id}", h.exigir("reconhecimento_adicionar", h.reconhecimentoSalvar))
	r.Post("/reconhecimentos/{id}/excluir", h.exigir("reconhecimento_adicionar", h.reconhecimentoExcluir))
	r.Get("/resumosemanal", h.exigir("resumosemanal_visualizar", h.resumosLista))
	r.Post("/resumosemanal", h.exigir("resumosemanal_adicionar", h.resumoGerar))
	r.Get("/resumosemanal/{id}", h.exigir("resumosemanal_visualizar", h.resumoVer))
	r.Post("/resumosemanal/{id}/excluir", h.exigir("resumosemanal_adicionar", h.resumoExcluir))
	r.Get("/ciclos", h.exigir("param_ciclo", h.ciclosLista))
	r.Get("/ciclos/novo", h.exigir("param_ciclo", h.cicloNovoForm))
	r.Post("/ciclos", h.exigir("param_ciclo", h.cicloSalvar))
	r.Post("/ciclos/semana", h.exigir("param_ciclo", h.ciclosSemanaSalvar))
	r.Get("/visitas", h.exigir("visita_visualizar", h.visitasLista))
	r.Get("/visitas/nova", h.exigir("visita_adicionar", h.visitaNovaForm))
	r.Post("/visitas", h.exigir("visita_adicionar", h.visitaSalvar))
	r.Get("/visitas/{id}", h.exigir("visita_visualizar", h.visitaFicha))
	r.Post("/visitas/{id}", h.exigir("visita_editar", h.visitaSalvar))
	r.Post("/visitas/{id}/fotos", h.exigir("visita_editar", h.visitaFotoEnviar))
	r.Get("/visitas/{id}/fotos/{fid}", h.exigir("visita_visualizar", h.visitaFotoVer))
	r.Post("/visitas/{id}/fotos/{fid}/excluir", h.exigir("visita_editar", h.visitaFotoExcluir))
	r.Post("/visitas/{id}/anotacoes", h.exigir("visita_anotacao", h.visitaAnotar))
	// ficha de caso (9.1): andamento comum a todo tipo de caso; a permissão vem do tipo
	r.Get("/investigacoes", h.exigir("investigacao_visualizar", h.investigacoesLista))
	r.Get("/investigacoes/nova", h.exigir("investigacao_nova", h.investigacaoNova))
	r.Post("/investigacoes", h.exigir("investigacao_nova", h.investigacaoCriar))
	r.Get("/investigacoes/{id}", h.exigir("investigacao_visualizar", h.investigacaoFicha))
	r.Post("/investigacoes/{id}", h.exigir("investigacao_editar", h.investigacaoSalvar))
	r.Post("/investigacoes/{id}/local", h.exigir("investigacao_local", h.investigacaoLocal))
	r.Post("/investigacoes/{id}/entrevista", h.exigir("investigacao_entrevista", h.investigacaoEntrevista))
	r.Post("/investigacoes/{id}/atender", h.exigir("investigacao_atender", h.investigacaoAtender))
	r.Post("/investigacoes/{id}/resultado", h.exigir("investigacao_resultado", h.investigacaoResultado))
	r.Post("/investigacoes/{id}/encerrar", h.exigir("investigacao_encerrar", h.investigacaoEncerrar))
	r.Post("/investigacoes/{id}/reabrir", h.exigir("investigacao_reabrir", h.investigacaoReabrir))
	r.Post("/investigacoes/{id}/excluir", h.exigir("investigacao_desativar", h.investigacaoExcluir))
	r.Post("/casos/{tipo}/{id}/comentario", h.casoRota(true, h.casoComentar))
	r.Post("/casos/{tipo}/{id}/encaminhar", h.casoRota(true, h.casoEncaminhar))
	r.Post("/casos/{tipo}/{id}/anexos", h.casoRota(true, h.casoAnexar))
	r.Get("/casos/{tipo}/{id}/anexos/{aid}", h.casoRota(false, h.casoAnexoVer))
	r.Post("/casos/{tipo}/{id}/anexos/{aid}/excluir", h.casoRota(true, h.casoAnexoExcluir))
	r.Post("/visitas/{id}/excluir", h.exigir("visita_desativar", h.visitaExcluir))
	r.Post("/visitas/{id}/ativo", h.exigir("visita_desativar", h.visitaAtivo))
	r.Post("/visitas/{id}/amostras", h.exigir("visita_editar", h.amostraCriar))
	r.Post("/visitas/{id}/amostras/{aid}", h.exigir("visita_editar", h.amostraColeta))
	r.Post("/visitas/{id}/amostras/{aid}/resultado", h.exigir("visita_laboratorio_adicionar", h.amostraResultado))
	r.Post("/visitas/{id}/amostras/{aid}/excluir", h.exigir("visita_editar", h.amostraExcluir))
	r.Get("/dashboard", h.exigir("visita_visualizar", h.dashboard))
	r.Get("/relatorios", h.relatoriosIndice)
	r.Get("/relatorios/{slug}", h.relatorioGerar)
	r.Get("/bi", h.exigir("bi_visualizar", h.biPagina))
	r.Get("/laboratorio", h.exigir("visita_laboratorio_visualizar", h.laboratorioLista))
	r.Get("/mobilidade/historico", h.exigir("historico_mobilidade", h.mobilidadeHistorico))
	r.Post("/visitas/lote", h.exigir("visita_desativar", h.visitasLote))
	r.Get("/doencas", h.exigir("param_doenca", h.doencasLista))
	r.Get("/doencas/nova", h.exigir("param_doenca", h.doencaForm))
	r.Post("/doencas", h.exigir("param_doenca", h.doencaSalvar))
	r.Get("/doencas/{id}", h.exigir("param_doenca", h.doencaForm))
	r.Post("/doencas/{id}", h.exigir("param_doenca", h.doencaSalvar))
	r.Get("/entrevistas", h.exigir("param_doenca", h.entrevistasLista))
	r.Get("/entrevistas/nova", h.exigir("param_doenca", h.entrevistaForm))
	r.Post("/entrevistas", h.exigir("param_doenca", h.entrevistaSalvar))
	r.Get("/entrevistas/{id}", h.exigir("param_doenca", h.entrevistaForm))
	r.Post("/entrevistas/{id}", h.exigir("param_doenca", h.entrevistaSalvar))
	r.Get("/leis", h.exigir("param_leis", h.leisLista))
	r.Get("/leis/nova", h.exigir("param_leis", h.leiNovaForm))
	r.Post("/leis", h.exigir("param_leis", h.leiSalvar))
	r.Get("/leis/{id}", h.exigir("param_leis", h.leiEditarForm))
	r.Post("/leis/{id}", h.exigir("param_leis", h.leiSalvar))
	r.Post("/leis/{id}/excluir", h.exigir("param_leis", h.leiExcluir))
	r.Post("/leis/{id}/trechos", h.exigir("param_leis", h.trechoSalvar))
	r.Post("/leis/{id}/trechos/{tid}", h.exigir("param_leis", h.trechoSalvar))
	r.Post("/leis/{id}/trechos/{tid}/excluir", h.exigir("param_leis", h.trechoExcluir))
	r.Get("/parametros/geral", h.exigir("param_geral", h.parametrosForm(grupoGeral)))
	r.Post("/parametros/geral", h.exigir("param_geral", h.parametrosSalvar(grupoGeral)))
	r.Get("/parametros/mobilidade", h.exigir("param_mobilidade", h.parametrosForm(grupoMobilidade)))
	r.Post("/parametros/mobilidade", h.exigir("param_mobilidade", h.parametrosSalvar(grupoMobilidade)))
	r.Get("/ciclos/{id}", h.exigir("param_ciclo", h.cicloEditarForm))
	r.Post("/ciclos/{id}", h.exigir("param_ciclo", h.cicloSalvar))
	r.Post("/ciclos/{id}/excluir", h.exigir("param_ciclo", h.cicloExcluir))
	r.Get("/cidades", h.exigir("bairro_visualizar", h.cidadesLista))
	r.Get("/cidades/nova", h.exigir("bairro_adicionar", h.cidadeNovaForm))
	r.Post("/cidades", h.exigir("bairro_adicionar", h.cidadeSalvar))
	r.Get("/cidades/{id}", h.exigir("bairro_visualizar", h.cidadeEditarForm))
	r.Post("/cidades/{id}", h.exigir("bairro_adicionar", h.cidadeSalvar))

	r.Get("/mapa", h.mapaPagina)
	r.Get("/mapa/rastro", h.exigir("mapa_coleta", h.rastroPagina))
	r.Get("/mapa/rastro.geojson", h.exigir("mapa_coleta", h.rastroGeoJSON))
	r.Get("/mapa/bairros.geojson", h.exigir("bairro_visualizar", h.mapaBairros))
	r.Get("/mapa/quarteiroes.geojson", h.exigir("bairro_visualizar", h.mapaQuarteiroes))
	r.Get("/mapa/locais.geojson", h.exigir("local_visualizar", h.mapaLocais))

	r.Get("/lixeira", h.exigir("conf_lixeira", h.lixeiraPagina))
	r.Post("/lixeira/{tipo}/{id}/restaurar", h.exigir("conf_lixeira", h.lixeiraRestaurar))
	r.Post("/lixeira/{tipo}/{id}/excluir-definitivo", h.exigir("conf_exclusao_definitiva", h.lixeiraExcluirDefinitivo))
	r.Post("/cidades/{id}/excluir", h.exigir("bairro_adicionar", h.cidadeExcluir))
	r.Post("/bairros/{id}/excluir", h.exigir("bairro_adicionar", h.bairroExcluir))
	r.Post("/bairros/{id}/quarteiroes/{qid}/excluir", h.exigir("bairro_adicionar", h.quarteiraoExcluir))
	r.Post("/locais/{id}/excluir", h.exigir("local_adicionar", h.localExcluir))

	r.Get("/importacoes", h.exigir("conf_importacao", h.importacoesLista))
	r.Get("/importacoes/nova", h.exigir("conf_importacao", h.importacaoNova))
	r.Get("/importacoes/modelo/{arquivo}", h.exigir("conf_importacao", h.importacaoModelo))
	r.Post("/importacoes", h.exigir("conf_importacao", h.importacaoEnviar))
	r.Get("/importacoes/mapeamentos", h.exigir("conf_importacao", h.importacaoMapeamentos))
	r.Post("/importacoes/mapeamentos/{id}/excluir", h.exigir("conf_importacao", h.importacaoMapeamentoExcluir))
	r.Get("/importacoes/{id}", h.exigir("conf_importacao", h.importacaoFicha))
	r.Get("/importacoes/{id}/mapear", h.exigir("conf_importacao", h.importacaoMapearForm))
	r.Post("/importacoes/{id}/mapear", h.exigir("conf_importacao", h.importacaoMapearSalvar))
	r.Get("/importacoes/{id}/progresso", h.exigir("conf_importacao", h.importacaoProgresso))
	r.Get("/importacoes/{id}/relatorio.csv", h.exigir("conf_importacao", h.importacaoRelatorio))
	r.Get("/importacoes/{id}/erros.csv", h.exigir("conf_importacao", h.importacaoErros))
	r.Post("/importacoes/{id}/executar", h.exigir("conf_importacao", h.importacaoExecutar))
	r.Post("/importacoes/{id}/retomar", h.exigir("conf_importacao", h.importacaoRetomar))
	r.Post("/importacoes/{id}/interromper", h.exigir("conf_importacao", h.importacaoInterromper))
	r.Post("/importacoes/{id}/cancelar", h.exigir("conf_importacao", h.importacaoCancelar))
	r.Post("/importacoes/{id}/desfazer", h.exigir("conf_importacao", h.importacaoDesfazer))

	r.Get("/parametros/api", h.exigir("conf_api", h.apiListaPagina))
	r.Get("/parametros/api/nova", h.exigir("conf_api", h.apiNovaForm))
	r.Post("/parametros/api", h.exigir("conf_api", h.apiCriar))
	r.Get("/parametros/api/{id}", h.exigir("conf_api", h.apiFicha))
	r.Post("/parametros/api/{id}", h.exigir("conf_api", h.apiSalvar))
	r.Post("/parametros/api/{id}/rotacionar", h.exigir("conf_api", h.apiRotacionar))
	r.Post("/parametros/api/{id}/ativo", h.exigir("conf_api", h.apiAtivo))

	r.Get("/parametros/acesso", h.exigir("acesso_local", h.acessoPagina))
	r.Get("/parametros/acesso/quarteiroes", h.exigir("acesso_local", h.grupoQuarteiroes))
	r.Post("/parametros/acesso", h.exigir("acesso_local", h.acessoCriar))
	r.Post("/parametros/acesso/metodo", h.exigir("acesso_local", h.acessoMetodo))
	r.Post("/parametros/acesso/{id}/remover", h.exigir("acesso_local", h.acessoRemover))
	r.Post("/locais/{id}/acesso", h.exigir("acesso_local", h.localAcessoCriar))

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
