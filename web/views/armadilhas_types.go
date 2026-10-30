package views

import "net/url"

// ArmadilhaCfg diferencia a ovitrampa (contagem de ovos) da EDL (manutenção com nível da água).
type ArmadilhaCfg struct {
	Slug    string // "ovitrampa" (presença, Lixeira)
	Caminho string // "/ovitrampas"
	Plural  string // "Ovitrampas"
	Rotulo  string // "Ovitrampa"
	Ovos    bool   // registros têm quantidade de ovos (ovitrampa) em vez de nível da água (EDL)
	RegNome string // "Contagem de ovos" / "Manutenção"
}

type ArmadilhaLinha struct {
	ID             int64
	IDTexto        string
	Status         string
	StatusRotulo   string
	Data           string
	Endereco       string
	Bairro         string
	Agente         string
	UltimoRegistro string
}

type ArmadilhasFiltro struct{ Busca, Status, Bairro, De, Ate string }

type ArmadilhasData struct {
	Cfg        ArmadilhaCfg
	Linhas     []ArmadilhaLinha
	Filtro     ArmadilhasFiltro
	Status     []CidadeOpcao
	Bairros    []CidadeOpcao
	Pag        Pagination
	Query      url.Values
	Ordem, Dir string
	PodeNova   bool
}

type ArmadilhaEscolherLocal struct {
	Cfg    ArmadilhaCfg
	Busca  string
	Buscou bool
	Locais []LocalVinculo
}

// ArmadilhaFormData: dados editáveis (nova ou existente). Data no formato do datetime-local (aaaa-mm-ddThh:mm).
type ArmadilhaFormData struct {
	Cfg           ArmadilhaCfg
	ID            int64 // 0 = nova
	LocalID       int64
	LocalEndereco string
	Data          string
	Agente        string
	Agentes       []CidadeOpcao
	Latitude      string
	Longitude     string
	Anotacao      string
	Erro          string
	CSRF          string
}

func (d ArmadilhaFormData) Novo() bool { return d.ID == 0 }

type ArmadilhaRegistroLinha struct {
	ID    int64
	Texto string // "200 ovos" ou nível da água
	Ovos  string
	Nivel string
	Data  string
	Por   string
	Nota  string
}

type ArmadilhaFichaData struct {
	Cfg            ArmadilhaCfg
	ID             int64
	IDTexto        string
	Status         string
	StatusRotulo   string
	Data           string
	Agente         string
	Coordenadas    string
	Anotacao       string
	Retirada       string
	RetiradoPor    string
	LocalID        int64
	LocalEndereco  string
	Bairro         string
	Dados          ArmadilhaFormData
	Registros      []ArmadilhaRegistroLinha
	Niveis         []CidadeOpcao
	StatusOpcoes   []CidadeOpcao
	Usuarios       []CidadeOpcao
	AgoraLocal     string
	PodeEditar     bool
	PodeStatus     bool
	PodeRegistrar  bool
	PodeRegEditar  bool
	PodeRegExcluir bool
	Aviso          string
	Erro           string
	CSRF           string
}
