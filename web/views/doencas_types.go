package views

// DoencaLinha é uma doença/agravo na lista.
type DoencaLinha struct {
	ID         int64
	Nome       string
	Entrevista string
	Ativo      bool
}

type DoencaFormData struct {
	ID          int64 // 0 = nova
	Nome        string
	Ativo       bool
	Entrevistas []CidadeOpcao
	Erro        string
	CSRF        string
}

func (d DoencaFormData) Novo() bool { return d.ID == 0 }

type EntrevistaLinha struct {
	ID        int64
	Codigo    string
	Titulo    string
	Ativo     bool
	Perguntas int
	Doencas   int
}

type EntrevistaFormData struct {
	ID         int64 // 0 = nova
	Codigo     string
	Titulo     string
	Ativo      bool
	CamposJSON string // lista de perguntas, no formato do editor
	Erro       string
	CSRF       string
}

func (d EntrevistaFormData) Novo() bool { return d.ID == 0 }
