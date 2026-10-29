package views

// ---- Guia e Novidades ----

type GuiaItem struct{ Slug, Titulo, Resumo, Trecho string }

type GuiaListaData struct {
	Q     string
	Itens []GuiaItem
}

type GuiaArtigoData struct {
	Slug   string
	Titulo string
	HTML   string
	Outros []GuiaItem
}

// VersaoView é uma versão na página de Novidades, com os itens já agrupados por tipo.
type VersaoView struct {
	Versao     string
	Titulo     string
	Data       string
	Nova       bool // ainda não vista por quem está olhando
	Novos      []string
	Melhorados []string
	Corrigidos []string
}

type NovidadesData struct {
	Versoes       []VersaoView
	PodeRegistrar bool
}

type VersaoFormData struct {
	Versao    string
	Titulo    string
	Data      string
	Novo      string
	Melhorado string
	Corrigido string
	Erro      string
	CSRF      string
}

// versaoTexto é a versão mostrada no rodapé do menu.
func versaoTexto(l Layout) string {
	if l.Versao != "" {
		return l.Versao
	}
	return Version
}
