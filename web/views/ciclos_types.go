package views

// ---- Ciclos ----

type CicloLinha struct {
	ID         int64
	Nome       string
	Inicio     string // dd/mm/aaaa
	Final      string
	Fechado    bool
	Publico    bool
	Atividades string // siglas, separadas por vírgula
}

type CiclosData struct {
	Linhas     []CicloLinha
	PodeEditar bool
}

// AtividadeOpcao é uma atividade do catálogo, marcada ou não.
type AtividadeOpcao struct {
	ID       int16
	Sigla    string
	Nome     string
	Marcada  bool // habilitada no ciclo
	NoPainel bool // aparece no painel público
}

type CicloOpcao struct {
	ID      int64
	Nome    string
	Marcado bool
}

type CicloFormData struct {
	ID                 int64 // 0 = ciclo novo
	Nome               string
	Inicio             string // aaaa-mm-dd (campo de data)
	Final              string
	Fechado            bool
	PermiteForaPeriodo bool
	PainelPublico      bool
	PainelTexto        string
	Atividades         []AtividadeOpcao
	Sincronizar        []CicloOpcao
	Aviso              string // sobreposição de datas, não impede salvar
	PodeEditar         bool
	Erro               string
	CSRF               string
}

func (d CicloFormData) Novo() bool { return d.ID == 0 }
