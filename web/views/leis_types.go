package views

// ---- Leis municipais ----

type LeiLinha struct {
	ID         int64
	Numero     string
	Ano        int
	Nome       string
	Publicacao string // dd/mm/aaaa
	Ativo      bool
	Trechos    int
}

type LeiTrechoLinha struct {
	ID     int64
	Artigo string
	Texto  string
}

type LeiFormData struct {
	ID         int64 // 0 = lei nova
	Numero     string
	Ano        string
	Nome       string
	Publicacao string // aaaa-mm-dd
	Ementa     string
	Preambulo  string
	Link       string
	Ativo      bool
	Trechos    []LeiTrechoLinha
	// Trecho em edição ou novo, com o erro (se houver), para repopular o formulário de trecho.
	NovoArtigo string
	NovoTexto  string
	ErroTrecho string
	Erro       string
	CSRF       string
}

func (d LeiFormData) Novo() bool { return d.ID == 0 }
