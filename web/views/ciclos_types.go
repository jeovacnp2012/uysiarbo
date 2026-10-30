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
	Semanas    string // "SE 1/2026 a SE 9/2026"
}

type CiclosData struct {
	Linhas       []CicloLinha
	PodeEditar   bool
	TipoSemana   string // "sinan" ou "iso"
	RotuloSemana string
	CSRF         string
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
	Semanas            string // semanas epidemiológicas que o ciclo cobre
	PodeEditar         bool
	Erro               string
	CSRF               string
}

func (d CicloFormData) Novo() bool { return d.ID == 0 }
