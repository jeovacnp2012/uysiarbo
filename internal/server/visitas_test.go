package server

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"uysiarbo/internal/db/sqlccliente"
)

// pngMinimo é um PNG válido de 1x1 pixel.
var pngMinimo, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")

// enviarFotos faz o POST multipart de fotos da visita.
func (n *navegador) enviarFotos(caminho string, arquivos map[string][]byte) *httptest.ResponseRecorder {
	n.t.Helper()
	if n.cookies[csrfCookie] == "" {
		n.get("/entrar")
	}
	var corpo bytes.Buffer
	mw := multipart.NewWriter(&corpo)
	_ = mw.WriteField("csrf", n.cookies[csrfCookie])
	for nome, dados := range arquivos {
		fw, _ := mw.CreateFormFile("foto", nome)
		_, _ = fw.Write(dados)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, caminho, io.NopCloser(&corpo))
	req.Host = n.host
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for k, v := range n.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	rec := httptest.NewRecorder()
	n.h.ServeHTTP(rec, req)
	return rec
}

type cenarioDeVisita struct {
	local, ciclo int64
	hoje, ontem  string
}

// cenarioVisita cria um local e um ciclo (com LIT e PE) em volta de hoje.
func (m *mundo) cenarioVisita(t *testing.T, ana *navegador) cenarioDeVisita {
	t.Helper()
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Rua das Palmeiras")
	q := m.criarQuarteirao(t, b, "1")
	loc := umLocal(t, m, ana, b, q, "numero", "10")
	agora := time.Now().UTC()
	ini, fim := agora.AddDate(0, 0, -60).Format("2006-01-02"), agora.AddDate(0, 0, 60).Format("2006-01-02")
	quer(t, ana.post("/ciclos", formCiclo("01 Ciclo", ini, fim, url.Values{"atividade": {"1", "3"}})), http.StatusSeeOther)
	cs, _ := m.cliQ.ListCiclos(t.Context())
	if len(cs) != 1 {
		t.Fatalf("ciclos = %d", len(cs))
	}
	return cenarioDeVisita{local: loc, ciclo: cs[0].ID, hoje: agora.Format("2006-01-02"), ontem: agora.AddDate(0, 0, -1).Format("2006-01-02")}
}

func (c cenarioDeVisita) form(agente string, extras ...string) url.Values {
	v := url.Values{
		"local": {s64(c.local)}, "ciclo": {s64(c.ciclo)}, "atividade": {"1"}, "agente": {agente},
		"data": {c.ontem}, "hora": {"08:30"}, "tipo": {"normal"}, "pendencia": {"nenhuma"},
	}
	for i := 0; i+1 < len(extras); i += 2 {
		v.Set(extras[i], extras[i+1])
	}
	return v
}

func TestVisitaRegistrarEditarEValidar(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)

	quer(t, ana.get("/visitas"), http.StatusOK, "Nenhuma visita encontrada")
	quer(t, ana.get("/visitas/nova"), http.StatusNotFound)
	quer(t, ana.get("/visitas/nova?local="+s64(c.local)), http.StatusOK, "Registrar visita", "Encontrou depósitos", "Pendência", "01 Ciclo")

	// Validações: nada é gravado.
	erros := map[string]url.Values{
		"data da visita é inválida":             c.form(agente, "data", "31/02/2026"),
		"não pode estar no futuro":              c.form(agente, "data", time.Now().UTC().AddDate(0, 0, 3).Format("2006-01-02")),
		"fora do período":                       c.form(agente, "data", time.Now().UTC().AddDate(0, 0, -120).Format("2006-01-02")),
		"não faz parte do ciclo":                c.form(agente, "atividade", "2"),
		"Escolha o agente":                      c.form(""),
		"hora de entrada é inválida":            c.form(agente, "hora", "25:99"),
		"Escolha o tipo":                        c.form(agente, "tipo", "outro"),
		"Escolha a pendência":                   c.form(agente, "pendencia", "talvez"),
		"com pendência não pode ter depósitos":  c.form(agente, "pendencia", "fechado", "achou", "1"),
		"passar da soma dos depósitos":          c.form(agente, "achou", "1", "a1", "2", "b", "1", "total_eliminado", "4"),
		"A1: informe":                           c.form(agente, "achou", "1", "a1", "-3"),
		"Larvicida L1":                          c.form(agente, "tratamento", "1", "l1_gramas", "abc"),
		"latitude e a longitude juntas":         c.form(agente, "lat", "-13.8"),
		"Longitude":                             c.form(agente, "lat", "-13.8", "lon", "300"),
		"assinatura precisa ser uma imagem PNG": c.form(agente, "assinatura", "javascript:alert(1)"),
	}
	for trecho, f := range erros {
		quer(t, ana.post("/visitas", f), http.StatusBadRequest, trecho)
	}
	if vs, _ := m.cliQ.ListVisitasRecentes(t.Context(), listarVisitas()); len(vs) != 0 {
		t.Fatalf("as recusadas não gravam: %d", len(vs))
	}

	// Visita completa.
	assinatura := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngMinimo)
	rec := ana.post("/visitas", c.form(agente,
		"achou", "1", "a1", "2", "a2", "1", "b", "0", "c", "3", "d1", "1", "d2", "0", "e", "1", "total_eliminado", "5",
		"amostra", "1", "tratamento", "1", "l1_gramas", "12,5", "l1_tratados", "4", "l2_gramas", "3", "l2_tratados", "1",
		"adulticida_tipo", "Cipermetrina", "adulticida_cargas", "2", "trat_eliminados", "2", "bri_volume", "1.5",
		"lat", "-13.8247", "lon", "-56.0813", "precisao", "8,5", "anotacao", "Morador presente.", "assinatura", assinatura))
	quer(t, rec, http.StatusSeeOther)
	if !strings.HasPrefix(local(rec), "/visitas/") || !strings.HasSuffix(local(rec), "?aviso=salva") {
		t.Fatalf("destino: %q", local(rec))
	}
	vs, _ := m.cliQ.ListVisitasRecentes(t.Context(), listarVisitas())
	if len(vs) != 1 {
		t.Fatalf("visitas = %d", len(vs))
	}
	id := s64(vs[0].ID)
	v, _ := m.cliQ.GetVisita(t.Context(), vs[0].ID)
	if !v.AchouDeposito || v.DepA1.Int32 != 2 || v.DepC.Int32 != 3 || v.TotalEliminado.Int32 != 5 || !v.ColetouAmostra ||
		textoNumerico(v.L1Gramas) != "12.5" || v.L1Tratados.Int32 != 4 || v.AdulticidaTipo != "Cipermetrina" || textoNumerico(v.BriVolume) != "1.5" ||
		v.Latitude.Float64 != -13.8247 || v.PrecisaoGps.Float64 != 8.5 || v.Anotacao != "Morador presente." || v.Assinatura != assinatura {
		t.Fatalf("visita gravada: %+v", v)
	}
	if !v.Semana.Valid || v.Semana.Int16 < 1 || v.Semana.Int16 > 53 {
		t.Errorf("a semana epidemiológica deveria ser calculada: %+v", v.Semana)
	}
	if v.HoraEntrada.Microseconds != int64(8*3600+30*60)*1e6 {
		t.Errorf("hora = %d", v.HoraEntrada.Microseconds)
	}

	quer(t, ana.get("/visitas"), http.StatusOK, "Rua das Palmeiras", "01 Ciclo", "LIT")
	quer(t, ana.get("/visitas?local="+s64(c.local)), http.StatusOK, "Mostrando as visitas do local")
	quer(t, ana.get("/visitas/"+id+"?aviso=salva"), http.StatusOK, "Visita salva.", "Cipermetrina", "Morador presente.", "Assinatura", "SE ")
	quer(t, ana.get("/locais/"+s64(c.local)), http.StatusOK, "Registrar visita")

	// Editar: sem mudança não gera auditoria; mudando, sim. Pendência apaga o que não vale mais (precisa limpar os marcadores).
	mesma := c.form(agente,
		"achou", "1", "a1", "2", "a2", "1", "b", "0", "c", "3", "d1", "1", "d2", "0", "e", "1", "total_eliminado", "5",
		"amostra", "1", "tratamento", "1", "l1_gramas", "12,5", "l1_tratados", "4", "l2_gramas", "3", "l2_tratados", "1",
		"adulticida_tipo", "Cipermetrina", "adulticida_cargas", "2", "trat_eliminados", "2", "bri_volume", "1.5",
		"lat", "-13.8247", "lon", "-56.0813", "precisao", "8,5", "anotacao", "Morador presente.", "assinatura", assinatura)
	quer(t, ana.post("/visitas/"+id, mesma), http.StatusSeeOther)
	mesma.Set("anotacao", "Morador ausente na volta.")
	quer(t, ana.post("/visitas/"+id, mesma), http.StatusSeeOther)
	quer(t, ana.post("/visitas/"+id, c.form(agente, "pendencia", "recusado")), http.StatusSeeOther)
	v, _ = m.cliQ.GetVisita(t.Context(), vs[0].ID)
	if v.Pendencia != "recusado" || v.AchouDeposito || v.ColetouAmostra || v.FezTratamento || v.DepA1.Valid || v.L1Gramas.Valid {
		t.Errorf("com pendência a visita não guarda depósitos, amostra nem tratamento: %+v", v)
	}
	quer(t, ana.post("/visitas/999999", c.form(agente)), http.StatusNotFound)

	ev := m.eventos(t)
	if achar(ev, "visita.criar") == nil {
		t.Error("faltou visita.criar na auditoria")
	}
	n := 0
	for _, e := range ev {
		if e.Acao == "visita.editar" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("visita.editar registrado %d vezes, queria 2 (a edição sem mudança não conta)", n)
	}
	for _, e := range ev {
		if strings.Contains(string(e.Depois), "base64") {
			t.Error("a imagem da assinatura não pode ir para a auditoria")
		}
	}
}

func listarVisitas() sqlccliente.ListVisitasRecentesParams {
	return sqlccliente.ListVisitasRecentesParams{VerDesativadas: true}
}

func TestVisitaCicloFechadoEPendencia(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)
	quer(t, ana.post("/visitas", c.form(agente)), http.StatusSeeOther)
	vs, _ := m.cliQ.ListVisitasRecentes(t.Context(), listarVisitas())
	id := s64(vs[0].ID)

	// Quem edita, mas não altera a pendência nem mexe em ciclo fechado.
	edu := m.usuarioCom(t, "Edu Lima", "edu@nm.gov.br", "visita_visualizar", "visita_editar", "visita_adicionar")
	quer(t, edu.post("/visitas/"+id, c.form(agente, "pendencia", "fechado")), http.StatusSeeOther) // mantém a pendência de antes
	if v, _ := m.cliQ.GetVisita(t.Context(), vs[0].ID); v.Pendencia != "nenhuma" {
		t.Errorf("sem a permissão a pendência não muda: %s", v.Pendencia)
	}

	// Ciclo fechado: só com a permissão própria.
	quer(t, ana.post("/ciclos/"+s64(c.ciclo), formCiclo("01 Ciclo", time.Now().UTC().AddDate(0, 0, -60).Format("2006-01-02"),
		time.Now().UTC().AddDate(0, 0, 60).Format("2006-01-02"), url.Values{"atividade": {"1", "3"}, "fechado": {"1"}})), http.StatusSeeOther)
	quer(t, edu.post("/visitas/"+id, c.form(agente, "anotacao", "x")), http.StatusBadRequest, "está fechado")
	quer(t, edu.post("/visitas", c.form(agente)), http.StatusBadRequest, "está fechado")
	quer(t, ana.post("/visitas/"+id, c.form(agente, "anotacao", "ajuste do coordenador")), http.StatusSeeOther)
}

func TestVisitaFotosAnotacoesEExclusao(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)
	quer(t, ana.post("/visitas", c.form(agente)), http.StatusSeeOther)
	vs, _ := m.cliQ.ListVisitasRecentes(t.Context(), listarVisitas())
	vid := vs[0].ID
	id := s64(vid)

	quer(t, ana.get("/visitas/"+id), http.StatusOK, "Nenhuma foto", "Anotações internas", "Excluir visita")

	// Fotos: só imagens de verdade.
	quer(t, ana.enviarFotos("/visitas/"+id+"/fotos", map[string][]byte{"x.png": []byte("<script>alert(1)</script>")}), http.StatusBadRequest, "não é JPEG, PNG nem WebP")
	quer(t, ana.enviarFotos("/visitas/"+id+"/fotos", map[string][]byte{"grande.png": append(append([]byte{}, pngMinimo...), make([]byte, 6<<20)...)}), http.StatusBadRequest, "passa de 5 MB")
	rec := ana.enviarFotos("/visitas/"+id+"/fotos", map[string][]byte{"../fachada.png": pngMinimo})
	quer(t, rec, http.StatusSeeOther)
	fotos, _ := m.cliQ.ListFotosDaVisita(t.Context(), vid)
	if len(fotos) != 1 || fotos[0].Nome != "fachada.png" || fotos[0].TipoMidia != "image/png" {
		t.Fatalf("fotos = %+v", fotos)
	}
	fid := s64(fotos[0].ID)
	img := ana.get("/visitas/" + id + "/fotos/" + fid)
	quer(t, img, http.StatusOK)
	if img.Header().Get("Content-Type") != "image/png" || img.Header().Get("X-Content-Type-Options") != "nosniff" || !bytes.Equal(img.Body.Bytes(), pngMinimo) {
		t.Errorf("cabeçalhos/corpo da foto: %v", img.Header())
	}
	quer(t, ana.get("/visitas/"+id), http.StatusOK, "fachada.png", "Excluir foto")
	quer(t, ana.get("/visitas/"+id+"/fotos/9999"), http.StatusNotFound)
	quer(t, ana.post("/visitas/"+id+"/fotos/"+fid+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/visitas/"+id+"/fotos/"+fid), http.StatusNotFound)

	// Anotações internas ficam em ordem e não se editam.
	quer(t, ana.post("/visitas/"+id+"/anotacoes", url.Values{"texto": {" "}}), http.StatusBadRequest, "1 a 2000")
	quer(t, ana.post("/visitas/"+id+"/anotacoes", url.Values{"texto": {"Voltar na quinta."}}), http.StatusSeeOther)
	quer(t, ana.get("/visitas/"+id), http.StatusOK, "Voltar na quinta.", "Ana")

	// Excluir: vai para a Lixeira; o local, com a visita ativa, não pode ser excluído.
	quer(t, ana.post("/locais/"+s64(c.local)+"/excluir", url.Values{}), http.StatusConflict, "1 visita ativa")
	quer(t, ana.post("/ciclos/"+s64(c.ciclo)+"/excluir", url.Values{}), http.StatusConflict, "1 visita ativa")
	quer(t, ana.post("/visitas/"+id+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/visitas/"+id), http.StatusNotFound)
	quer(t, ana.get("/visitas"), http.StatusOK, "Nenhuma visita encontrada")
	quer(t, ana.get("/lixeira?tipo=visita"), http.StatusOK, "Visita "+id)
	// Na lixeira, o local e o ciclo ainda têm vínculo (a visita excluída conta).
	quer(t, ana.post("/locais/"+s64(c.local)+"/excluir", url.Values{}), http.StatusSeeOther)

	ev := m.eventos(t)
	for _, a := range []string{"visita.criar", "visita.foto_adicionar", "visita.foto_excluir", "visita.anotacao", "visita.excluir"} {
		if achar(ev, a) == nil {
			t.Errorf("faltou %s na auditoria", a)
		}
	}
}

func TestVisitasRespeitamPermissoesEEscopo(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)
	quer(t, ana.post("/visitas", c.form(agente)), http.StatusSeeOther)
	vs, _ := m.cliQ.ListVisitasRecentes(t.Context(), listarVisitas())
	id := s64(vs[0].ID)

	// Quem só consulta não registra, não edita, não anota, não exclui.
	carla := m.comCarla(t)
	quer(t, carla.get("/visitas"), http.StatusOK, "Rua das Palmeiras")
	quer(t, carla.get("/visitas/"+id), http.StatusOK, "Visita "+id)
	quer(t, carla.get("/visitas/nova?local="+s64(c.local)), http.StatusForbidden)
	quer(t, carla.post("/visitas", c.form(agente)), http.StatusForbidden)
	quer(t, carla.post("/visitas/"+id, c.form(agente)), http.StatusForbidden)
	quer(t, carla.post("/visitas/"+id+"/anotacoes", url.Values{"texto": {"x"}}), http.StatusForbidden)
	quer(t, carla.post("/visitas/"+id+"/excluir", url.Values{}), http.StatusForbidden)
	quer(t, carla.enviarFotos("/visitas/"+id+"/fotos", map[string][]byte{"a.png": pngMinimo}), http.StatusForbidden)

	// "Apenas as minhas": a visita da Ana não aparece para quem só vê as próprias.
	pa, _ := m.cliQ.CreatePapel(t.Context(), papelParams("Só as minhas"))
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(pa.ID, "visita_visualizar", "visita_visualizar_minha"))
	rui := m.novoUsuarioLogado(t, "Rui Reis", "rui@nm.gov.br", pa.ID)
	quer(t, rui.get("/visitas/"+id), http.StatusNotFound)
	if strings.Contains(rui.get("/visitas").Body.String(), "href=\"/visitas/"+id+"\"") {
		t.Error("quem vê só as minhas não deveria ver a visita da Ana")
	}

	quer(t, ana.get("/visitas/abc"), http.StatusNotFound)
}

func TestFichaDoLocalMostraReincidenciaEHistorico(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)
	ficha := "/locais/" + s64(c.local)

	// Sem visitas, a ficha não mostra a seção.
	if strings.Contains(ana.get(ficha).Body.String(), `id="reincidencia"`) {
		t.Error("sem visitas não deveria haver a seção de reincidência")
	}

	// Uma visita com depósito: aparece no histórico, ainda sem reincidência.
	quer(t, ana.post("/visitas", c.form(agente, "achou", "1", "a1", "1", "total_eliminado", "1")), http.StatusSeeOther)
	quer(t, ana.get(ficha), http.StatusOK, "Visitas e reincidência", "Sem reincidência", "1 com depósito encontrado", "01 Ciclo")

	// A segunda visita com depósito, nos 12 meses, torna o local reincidente.
	quer(t, ana.post("/visitas", c.form(agente, "data", c.hoje, "achou", "1", "b", "1", "total_eliminado", "1")), http.StatusSeeOther)
	quer(t, ana.get(ficha), http.StatusOK, "Reincidente", "2 com depósito encontrado", "2 visita(s)")

	// Visita sem depósito não conta para a reincidência.
	quer(t, ana.post("/visitas", c.form(agente, "data", c.hoje)), http.StatusSeeOther)
	quer(t, ana.get(ficha), http.StatusOK, "3 visita(s)", "2 com depósito encontrado")
}
