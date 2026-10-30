package server

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"uysiarbo/internal/db/sqlccliente"
)

func (n *navegador) enviarAnexos(caminho string, arquivos map[string][]byte) *httptest.ResponseRecorder {
	n.t.Helper()
	if n.cookies[csrfCookie] == "" {
		n.get("/entrar")
	}
	var corpo bytes.Buffer
	mw := multipart.NewWriter(&corpo)
	_ = mw.WriteField("csrf", n.cookies[csrfCookie])
	for nome, dados := range arquivos {
		fw, _ := mw.CreateFormFile("arquivo", nome)
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

func TestFichaDeCasoComentaEncaminhaEAnexa(t *testing.T) {
	registrarTipoDeCaso(tipoDeCaso{
		Slug: "teste", Rotulo: "Caso de teste", Ver: "visita_visualizar", Editar: "visita_anotacao",
		Existe:  func(_ context.Context, _ *sqlccliente.Queries, id int64) bool { return id == 7 },
		Caminho: func(id int64) string { return "/visitas/" + strconv.FormatInt(id, 10) },
	})
	m := novoMundo(t)
	ana := m.comAna(t)
	quer(t, ana.post("/casos/inexistente/7/comentario", url.Values{"texto": {"x"}}), http.StatusNotFound)
	quer(t, ana.post("/casos/teste/8/comentario", url.Values{"texto": {"x"}}), http.StatusNotFound)

	rec := ana.post("/casos/teste/7/comentario", url.Values{"texto": {"Visitei o local e o dono não estava"}})
	quer(t, rec, http.StatusSeeOther)
	rec = ana.post("/casos/teste/7/comentario", url.Values{"texto": {"   "}})
	quer(t, rec, http.StatusSeeOther)
	if !strings.Contains(local(rec), "caso_erro=") {
		t.Fatalf("comentário vazio deveria voltar com erro: %s", local(rec))
	}

	agente := m.idUsuario(t, emailAna)
	quer(t, ana.post("/casos/teste/7/encaminhar", url.Values{"destino": {agente}, "texto": {"Seguir com a notificação"}}), http.StatusSeeOther)
	rec = ana.post("/casos/teste/7/encaminhar", url.Values{"destino": {"999999"}})
	if !strings.Contains(local(rec), "caso_erro=") {
		t.Fatalf("destino inválido deveria voltar com erro: %s", local(rec))
	}

	quer(t, ana.enviarAnexos("/casos/teste/7/anexos", map[string][]byte{"foto.png": pngMinimo}), http.StatusSeeOther)
	rec = ana.enviarAnexos("/casos/teste/7/anexos", map[string][]byte{"nota.txt": []byte("texto puro")})
	if !strings.Contains(local(rec), "caso_erro=") {
		t.Fatalf("arquivo de texto deveria ser recusado: %s", local(rec))
	}
	anexos, _ := m.cliQ.ListCasoAnexos(t.Context(), sqlccliente.ListCasoAnexosParams{Tipo: "teste", CasoID: 7})
	if len(anexos) != 1 {
		t.Fatalf("anexos = %d, esperava 1", len(anexos))
	}
	ver := ana.get("/casos/teste/7/anexos/" + s64(anexos[0].ID))
	if ver.Code != http.StatusOK || ver.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("download do anexo: %d %s", ver.Code, ver.Header().Get("Content-Type"))
	}

	eventos, _ := m.cliQ.ListCasoEventos(t.Context(), sqlccliente.ListCasoEventosParams{Tipo: "teste", CasoID: 7})
	var tipos []string
	for _, e := range eventos {
		tipos = append(tipos, e.Evento)
	}
	if strings.Join(tipos, ",") != "anexo,encaminhamento,comentario" {
		t.Fatalf("linha do tempo = %v", tipos)
	}
	resp, err := m.cliQ.GetCasoResponsavel(t.Context(), sqlccliente.GetCasoResponsavelParams{Tipo: "teste", CasoID: 7})
	if err != nil || resp.Nome == "" {
		t.Fatalf("responsável atual: %v %q", err, resp.Nome)
	}

	quer(t, ana.post("/casos/teste/7/anexos/"+s64(anexos[0].ID)+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/casos/teste/7/anexos/"+s64(anexos[0].ID)), http.StatusNotFound)
}
