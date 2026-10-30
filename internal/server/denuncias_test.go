package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"uysiarbo/internal/db/sqlccliente"
)

func TestDenunciaFluxoCompleto(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)

	base := url.Values{"tipo": {"Foco de mosquito"}, "origem": {"telefone"}, "data": {"2026-10-20"}, "titulo": {"Pneus com água"}, "endereco": {"Rua B, 20"}}
	com := func(extra map[string]string) url.Values {
		v := url.Values{}
		for k, x := range base {
			v[k] = x
		}
		for k, x := range extra {
			v.Set(k, x)
		}
		return v
	}

	// validações
	quer(t, ana.post("/denuncias", com(map[string]string{"tipo": "Inventado"})), http.StatusBadRequest)
	quer(t, ana.post("/denuncias", com(map[string]string{"titulo": " "})), http.StatusBadRequest)
	quer(t, ana.post("/denuncias", com(map[string]string{"cpf": "123"})), http.StatusBadRequest)
	quer(t, ana.post("/denuncias", com(map[string]string{"endereco": ""})), http.StatusBadRequest)

	// anônima: não guarda dados do cidadão, mesmo que venham no formulário
	rec := ana.post("/denuncias", com(map[string]string{"anonima": "1", "nome": "Fulano", "telefone": "6599999", "cpf": "52998224725"}))
	quer(t, rec, http.StatusSeeOther)
	rows, err := m.cliQ.ListDenuncias(t.Context(), listaTodasDenuncias())
	if err != nil || len(rows) != 1 {
		t.Fatalf("denúncias = %d, %v", len(rows), err)
	}
	d, _ := m.cliQ.GetDenunciaQualquer(t.Context(), rows[0].ID)
	if !d.Anonima || d.CidadaoNome != "" || d.CidadaoTelefone != "" || d.CidadaoCpf != "" {
		t.Fatalf("anônima guardou dados do cidadão: %+v", d)
	}

	// identificada, com telefone repetido mostra a anterior
	rec = ana.post("/denuncias", com(map[string]string{"nome": "Maria", "telefone": "65988887777", "cpf": "529.982.247-25"}))
	quer(t, rec, http.StatusSeeOther)
	caminho := local(rec)
	if !strings.HasPrefix(caminho, "/denuncias/") {
		t.Fatalf("redirecionou para %q", caminho)
	}
	quer(t, ana.get("/denuncias"), http.StatusOK)
	quer(t, ana.get("/denuncias/nova"), http.StatusOK)
	corpo := ana.get(caminho)
	quer(t, corpo, http.StatusOK)
	if !strings.Contains(corpo.Body.String(), "Pneus com água") {
		t.Fatal("a ficha deveria mostrar o título")
	}

	// andamento: não encerra sem atender; cancelar exige motivo
	rec = ana.post(caminho+"/encerrar", url.Values{})
	if !strings.Contains(local(rec), "caso_erro=") {
		t.Fatalf("encerrar sem atender deveria voltar com erro: %s", local(rec))
	}
	rec = ana.post(caminho+"/cancelar", url.Values{"texto": {" "}})
	if !strings.Contains(local(rec), "caso_erro=") {
		t.Fatalf("cancelar sem motivo deveria voltar com erro: %s", local(rec))
	}
	quer(t, ana.post(caminho+"/atender", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(caminho+"/encerrar", url.Values{"texto": {"Foco eliminado"}}), http.StatusSeeOther)
	quer(t, ana.post(caminho, com(map[string]string{"titulo": "Outro"})), http.StatusConflict)
	quer(t, ana.post(caminho+"/reabrir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(caminho, com(map[string]string{"titulo": "Pneus e vasos", "nome": "Maria", "telefone": "65988887777"})), http.StatusSeeOther)

	// vínculos inexistentes
	quer(t, ana.post(caminho+"/local", url.Values{"local": {"999999"}}), http.StatusNotFound)
	rec = ana.post(caminho+"/visita", url.Values{"visita": {"999999"}})
	if !strings.Contains(local(rec), "caso_erro=") {
		t.Fatalf("visita inexistente deveria voltar com erro: %s", local(rec))
	}

	// cancelar com motivo e excluir (lógica)
	quer(t, ana.post(caminho+"/cancelar", url.Values{"texto": {"Duplicada"}}), http.StatusSeeOther)
	quer(t, ana.post(caminho+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusNotFound)
}

func listaTodasDenuncias() sqlccliente.ListDenunciasParams {
	return sqlccliente.ListDenunciasParams{Limite: 100}
}
