package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDiariaDeTrabalho(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	base := "/zoonose/campanha/diaria"

	quer(t, ana.get(base), http.StatusOK)
	quer(t, ana.get(base+"/nova"), http.StatusOK)
	quer(t, ana.get(base+"/999999"), http.StatusNotFound)
	quer(t, ana.post(base, url.Values{"data": {"ontem"}}), http.StatusBadRequest)

	rec := ana.post(base, url.Values{"data": {time.Now().Format("2006-01-02")}, "anotacao": {"dia de teste"}})
	quer(t, rec, http.StatusSeeOther)
	caminho := strings.SplitN(local(rec), "?", 2)[0]
	if !strings.HasPrefix(caminho, base+"/") {
		t.Fatalf("redirecionou para %q", local(rec))
	}
	quer(t, ana.get(caminho), http.StatusOK, "dia de teste", "Aguardando", "Temperaturas")

	// equipe
	quer(t, ana.post(caminho+"/equipe", url.Values{"usuario": {"999999"}}), http.StatusBadRequest)
	quer(t, ana.post(caminho+"/equipe", url.Values{"usuario": {m.idUsuario(t, emailAna)}, "lider": {"1"}}), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusOK, "líder")

	// registro: valores inválidos e válidos
	quer(t, ana.post(caminho+"/registro", url.Values{"ini_doses": {"-1"}}), http.StatusBadRequest)
	quer(t, ana.post(caminho+"/registro", url.Values{"estoque_08:00": {"999"}}), http.StatusBadRequest)
	quer(t, ana.post(caminho+"/registro", url.Values{"ini_doses": {"50"}, "estoque_08:00": {"4,5"}, "uso_08:00": {"5"}, "recebimento": {"07:49"}}), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusOK, `value="50"`, `value="4.5"`, "07:49")

	// andamento: só inicia aguardando, só finaliza executando
	quer(t, ana.post(caminho+"/finalizar", url.Values{}), http.StatusBadRequest)
	quer(t, ana.post(caminho+"/iniciar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(caminho+"/iniciar", url.Values{}), http.StatusBadRequest)

	// atendimento ligado à diária aparece na ficha
	agora := time.Now().UTC().Format(formatoHoraLocal)
	id := strings.TrimPrefix(caminho, base+"/")
	quer(t, ana.post("/zoonose/campanha", url.Values{"local": {s64(c.local)}, "data": {agora}, "situacao": {"atendido"}, "vac_cachorros": {"3"}, "diaria": {id}}), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusOK, "Vacinados: 3")
	// diária com atendimento não vai à lixeira
	quer(t, ana.post(caminho+"/finalizar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusOK, "Finalizado")
	quer(t, ana.post(caminho+"/reabrir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(caminho+"/cancelar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(caminho+"/reativar", url.Values{}), http.StatusSeeOther)
}

func TestDiariaConsomeEstoque(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	rec := ana.post("/estoque/unidade", url.Values{"nome": {"Almoxarifado"}})
	idUn := strings.TrimPrefix(strings.SplitN(local(rec), "?", 2)[0], "/estoque/unidade/")
	mk := func(nome, categoria, qtd string) string {
		rec := ana.post("/estoque/produto", url.Values{"unidade": {idUn}, "nome": {nome}, "tipo": {"insumo"}, "medida": {"un"}, "saida": {"campo"},
			"selecionavel": {"1"}, "categoria": {categoria}, "ativo": {"1"}})
		quer(t, rec, http.StatusSeeOther)
		idP := strings.TrimPrefix(strings.SplitN(local(rec), "?", 2)[0], "/estoque/produto/")
		rec = ana.post("/estoque/lote", url.Values{"produto": {idP}, "numero": {"L1"}, "quantidade": {qtd}, "validade": {"2030-01-01"}})
		quer(t, rec, http.StatusSeeOther)
		return strings.TrimPrefix(strings.SplitN(local(rec), "?", 2)[0], "/estoque/lote/")
	}
	vac := mk("Vacina antirrábica", "vacina", "10")
	ser := mk("Seringa 1 mL", "seringa", "100")

	base := "/zoonose/campanha/diaria"
	rec = ana.post(base, url.Values{"data": {time.Now().Format("2006-01-02")}})
	quer(t, rec, http.StatusSeeOther)
	d := strings.SplitN(local(rec), "?", 2)[0]
	quer(t, ana.get(d), http.StatusOK, "Estoque da diária", "Vacina antirrábica", "Seringa 1 mL")
	quer(t, ana.post(d+"/estoque", url.Values{"vacina_lote": {ser}}), http.StatusBadRequest, "lote de vacina")
	quer(t, ana.post(d+"/estoque", url.Values{"vacina_lote": {vac}, "seringa_lote": {ser}}), http.StatusSeeOther)
	quer(t, ana.post(d+"/registro", url.Values{"fim_seringas_usadas": {"20"}, "fim_seringas_perdidas": {"2"}, "fim_seringas_defeito": {"1"}, "fim_frascos_vazios": {"4"}}), http.StatusSeeOther)
	quer(t, ana.post(d+"/iniciar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(d+"/finalizar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/estoque/lote/"+vac), http.StatusOK, "6")
	quer(t, ana.get("/estoque/lote/"+ser), http.StatusOK, "77")
	quer(t, ana.get(d), http.StatusOK, "Baixado", "23 un", "4 un")
	quer(t, ana.post(d+"/reabrir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/estoque/lote/"+vac), http.StatusOK, "10")
	quer(t, ana.get("/estoque/lote/"+ser), http.StatusOK, "100")

	// saldo insuficiente recusa a finalização
	quer(t, ana.post(d+"/registro", url.Values{"fim_frascos_vazios": {"11"}}), http.StatusSeeOther)
	quer(t, ana.post(d+"/finalizar", url.Values{}), http.StatusBadRequest, "saldo")
}
