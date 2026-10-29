package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"uysiarbo/internal/authz"
)

// Tipo diz de onde vem a conta: a plataforma (master ou não) ou um usuário de um cliente.
type Tipo string

const (
	TipoPlataforma Tipo = "plataforma"
	TipoUsuario    Tipo = "usuario"
)

// Conta é uma pessoa que pode entrar no sistema.
type Conta struct {
	ID              string
	Tipo            Tipo
	Nome            string
	Email           string
	SenhaHash       string
	Ativo           bool
	DeveTrocarSenha bool
	BloqueadoAte    time.Time // zero se não está bloqueada
	Tentativas      int       // falhas seguidas de senha
	NovidadeVista   int64     // última versão das Novidades que a pessoa já viu
	PapelID         int64     // só para usuário de cliente
	Master          bool      // só para conta da plataforma: nenhuma regra de permissão se aplica
}

var (
	ErrNaoEncontrada  = errors.New("não encontrado")
	ErrCredenciais    = errors.New("e-mail ou senha incorretos")
	ErrSessaoInvalida = errors.New("sessão inválida ou expirada")
	ErrSenhaAtual     = errors.New("a senha atual não confere")
)

// ErroCredenciais é o ErrCredenciais com informação para a auditoria: Conta é preenchida quando o
// e-mail existe (nunca mostre isso a quem tentou entrar) e Bloqueou diz que esta falha acionou o
// bloqueio. errors.Is(err, ErrCredenciais) continua valendo.
type ErroCredenciais struct {
	Conta    *Conta
	Bloqueou bool
}

func (ErroCredenciais) Error() string      { return ErrCredenciais.Error() }
func (ErroCredenciais) Is(alvo error) bool { return alvo == ErrCredenciais }

// ErrBloqueada indica excesso de tentativas erradas.
type ErrBloqueada struct {
	Ate   time.Time
	Conta Conta
}

func (e ErrBloqueada) Error() string {
	return fmt.Sprintf("acesso bloqueado por excesso de tentativas até %s", e.Ate.Format("15:04"))
}

// SenhaFracaError explica por que a senha escolhida não serve.
type SenhaFracaError struct{ Motivo string }

func (e SenhaFracaError) Error() string { return e.Motivo }

// ContaStore guarda as contas de um tipo (a plataforma ou um cliente).
type ContaStore interface {
	Tipo() Tipo
	PorEmail(ctx context.Context, email string) (Conta, error)
	PorID(ctx context.Context, id string) (Conta, error)
	RegistrarFalha(ctx context.Context, id string, limite int, bloqueio time.Duration) error
	RegistrarSucesso(ctx context.Context, id string) error
	DefinirSenha(ctx context.Context, id, hash string, deveTrocar bool) error
	Permissoes(ctx context.Context, c Conta) ([]string, error)
	// MarcarNovidadeVista guarda que a pessoa já viu as Novidades até esta versão.
	MarcarNovidadeVista(ctx context.Context, id string, versaoID int64) error
}

// SessaoRegistro é uma sessão como guardada no banco (só o hash do token).
type SessaoRegistro struct {
	Tipo    Tipo
	ContaID string
	Expira  time.Time
}

type SessaoStore interface {
	Criar(ctx context.Context, hash []byte, tipo Tipo, contaID string, expira time.Time, ip, userAgent string) error
	Buscar(ctx context.Context, hash []byte) (SessaoRegistro, error)
	Renovar(ctx context.Context, hash []byte, expira time.Time) error
	Remover(ctx context.Context, hash []byte) error
	// RemoverDaConta apaga as sessões da conta, menos a de hash "manter" (pode ser nil).
	RemoverDaConta(ctx context.Context, tipo Tipo, contaID string, manter []byte) error
}

// Sessao é uma pessoa autenticada, com o que ela pode fazer.
type Sessao struct {
	Conta     Conta
	Principal authz.Principal
	tokenHash []byte
}

// Service autentica pessoas e gerencia sessões e senhas num escopo (o console ou um cliente).
type Service struct {
	Contas   []ContaStore // tentadas na ordem ao entrar
	Sessoes  SessaoStore
	Agora    func() time.Time
	Limite   int           // falhas seguidas até bloquear (padrão 5)
	Bloqueio time.Duration // duração do bloqueio (padrão 15 min)
	Duracao  time.Duration // validade da sessão, renovada com o uso (padrão 12 h)
}

func (s *Service) agora() time.Time {
	if s.Agora != nil {
		return s.Agora()
	}
	return time.Now()
}

func (s *Service) limite() int {
	if s.Limite > 0 {
		return s.Limite
	}
	return 5
}

func (s *Service) bloqueio() time.Duration {
	if s.Bloqueio > 0 {
		return s.Bloqueio
	}
	return 15 * time.Minute
}

func (s *Service) duracao() time.Duration {
	if s.Duracao > 0 {
		return s.Duracao
	}
	return 12 * time.Hour
}

func (s *Service) loja(t Tipo) (ContaStore, error) {
	for _, c := range s.Contas {
		if c.Tipo() == t {
			return c, nil
		}
	}
	return nil, fmt.Errorf("tipo de conta %q não atendido neste escopo", t)
}

// hashFalso serve para gastar o mesmo tempo quando o e-mail não existe, para não revelar quais
// e-mails têm conta pelo tempo de resposta.
var hashFalso, _ = Hash("senha-que-nunca-sera-usada")

// Entrar confere e-mail e senha e abre uma sessão. Devolve o token (vai no cookie; o banco guarda só
// o hash). Erros de credencial são sempre ErrCredenciais, sem dizer se o e-mail existe.
func (s *Service) Entrar(ctx context.Context, email, senha, ip, userAgent string) (string, *Sessao, error) {
	email = strings.TrimSpace(email)
	var conta Conta
	var loja ContaStore
	for _, l := range s.Contas {
		c, err := l.PorEmail(ctx, email)
		if err == nil {
			conta, loja = c, l
			break
		}
		if !errors.Is(err, ErrNaoEncontrada) {
			return "", nil, err
		}
	}
	if loja == nil {
		_, _ = Confere(senha, hashFalso)
		return "", nil, ErroCredenciais{}
	}

	agora := s.agora()
	if conta.BloqueadoAte.After(agora) {
		return "", nil, ErrBloqueada{Ate: conta.BloqueadoAte, Conta: conta}
	}
	ok, err := Confere(senha, conta.SenhaHash)
	if err != nil {
		return "", nil, err
	}
	if !ok || !conta.Ativo {
		bloqueou := false
		if conta.Ativo {
			if err := loja.RegistrarFalha(ctx, conta.ID, s.limite(), s.bloqueio()); err != nil {
				return "", nil, err
			}
			bloqueou = conta.Tentativas+1 >= s.limite()
		}
		return "", nil, ErroCredenciais{Conta: &conta, Bloqueou: bloqueou}
	}
	if err := loja.RegistrarSucesso(ctx, conta.ID); err != nil {
		return "", nil, err
	}

	token, hash, err := novoToken()
	if err != nil {
		return "", nil, err
	}
	if err := s.Sessoes.Criar(ctx, hash, conta.Tipo, conta.ID, agora.Add(s.duracao()), ip, userAgent); err != nil {
		return "", nil, err
	}
	sess, err := s.montar(ctx, conta, hash)
	return token, sess, err
}

// Autenticar devolve a sessão do token, ou ErrSessaoInvalida. Renova a validade com o uso.
func (s *Service) Autenticar(ctx context.Context, token string) (*Sessao, error) {
	hash, ok := hashDoToken(token)
	if !ok {
		return nil, ErrSessaoInvalida
	}
	reg, err := s.Sessoes.Buscar(ctx, hash)
	if err != nil {
		if errors.Is(err, ErrNaoEncontrada) {
			return nil, ErrSessaoInvalida
		}
		return nil, err
	}
	agora := s.agora()
	if !reg.Expira.After(agora) {
		_ = s.Sessoes.Remover(ctx, hash)
		return nil, ErrSessaoInvalida
	}
	loja, err := s.loja(reg.Tipo)
	if err != nil {
		return nil, ErrSessaoInvalida
	}
	conta, err := loja.PorID(ctx, reg.ContaID)
	if err != nil || !conta.Ativo {
		_ = s.Sessoes.Remover(ctx, hash)
		return nil, ErrSessaoInvalida
	}
	// Renova só quando já gastou mais de 5 minutos, para não gravar a cada requisição.
	if novo := agora.Add(s.duracao()); novo.Sub(reg.Expira) > 5*time.Minute {
		_ = s.Sessoes.Renovar(ctx, hash, novo)
	}
	return s.montar(ctx, conta, hash)
}

func (s *Service) montar(ctx context.Context, c Conta, hash []byte) (*Sessao, error) {
	sess := &Sessao{Conta: c, tokenHash: hash}
	if c.Tipo == TipoPlataforma {
		// Só o master tem poder total; outra conta da plataforma não recebe nada por padrão.
		sess.Principal = authz.Com()
		if c.Master {
			sess.Principal = authz.Master()
		}
		return sess, nil
	}
	loja, err := s.loja(c.Tipo)
	if err != nil {
		return nil, err
	}
	perms, err := loja.Permissoes(ctx, c)
	if err != nil {
		return nil, err
	}
	sess.Principal = authz.Com(perms...)
	return sess, nil
}

// Sair encerra a sessão do token.
func (s *Service) Sair(ctx context.Context, token string) error {
	hash, ok := hashDoToken(token)
	if !ok {
		return nil
	}
	return s.Sessoes.Remover(ctx, hash)
}

// TrocarSenha define a senha escolhida pela própria pessoa. Exige a senha atual (no primeiro acesso
// depois de um reset, é a senha temporária), encerra as outras sessões e tira a obrigação de trocar.
func (s *Service) TrocarSenha(ctx context.Context, sess *Sessao, atual, nova string) error {
	if ok, err := Confere(atual, sess.Conta.SenhaHash); err != nil {
		return err
	} else if !ok {
		return ErrSenhaAtual
	}
	if atual == nova {
		return SenhaFracaError{"A nova senha precisa ser diferente da atual."}
	}
	if err := ValidarSenha(nova, sess.Conta.Email); err != nil {
		return err
	}
	hash, err := Hash(nova)
	if err != nil {
		return err
	}
	loja, err := s.loja(sess.Conta.Tipo)
	if err != nil {
		return err
	}
	if err := loja.DefinirSenha(ctx, sess.Conta.ID, hash, false); err != nil {
		return err
	}
	sess.Conta.SenhaHash, sess.Conta.DeveTrocarSenha = hash, false
	return s.Sessoes.RemoverDaConta(ctx, sess.Conta.Tipo, sess.Conta.ID, sess.tokenHash)
}

// ResetarSenha gera uma senha forte temporária para a conta e a devolve UMA vez, para quem resetou
// repassar. A senha antiga deixa de valer, todas as sessões da conta caem, o bloqueio é removido e
// a troca no próximo login passa a ser obrigatória. Quem chama confere a permissão (conf_usuario).
func (s *Service) ResetarSenha(ctx context.Context, tipo Tipo, contaID string) (string, error) {
	loja, err := s.loja(tipo)
	if err != nil {
		return "", err
	}
	if _, err := loja.PorID(ctx, contaID); err != nil {
		return "", err
	}
	senha, err := Gerar(16)
	if err != nil {
		return "", err
	}
	hash, err := Hash(senha)
	if err != nil {
		return "", err
	}
	if err := loja.DefinirSenha(ctx, contaID, hash, true); err != nil {
		return "", err
	}
	if err := s.Sessoes.RemoverDaConta(ctx, tipo, contaID, nil); err != nil {
		return "", err
	}
	return senha, nil
}

// ValidarSenha aplica a política de senhas escolhidas pelas pessoas.
func ValidarSenha(senha, email string) error {
	runas := []rune(senha)
	switch {
	case len(runas) < 10:
		return SenhaFracaError{"A senha precisa ter pelo menos 10 caracteres."}
	case len(runas) > 128:
		return SenhaFracaError{"A senha pode ter no máximo 128 caracteres."}
	}
	var letra, digito bool
	for _, r := range runas {
		letra = letra || unicode.IsLetter(r)
		digito = digito || unicode.IsDigit(r)
	}
	if !letra || !digito {
		return SenhaFracaError{"A senha precisa ter letras e números."}
	}
	if strings.EqualFold(senha, email) || strings.EqualFold(senha, strings.SplitN(email, "@", 2)[0]) {
		return SenhaFracaError{"A senha não pode ser igual ao e-mail."}
	}
	return nil
}

func novoToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(token))
	return token, h[:], nil
}

func hashDoToken(token string) ([]byte, bool) {
	if len(token) != 43 {
		return nil, false
	}
	h := sha256.Sum256([]byte(token))
	return h[:], true
}

// MarcarNovidadeVista registra que a pessoa viu as Novidades até a versão dada (nunca volta atrás).
func (s *Service) MarcarNovidadeVista(ctx context.Context, sess *Sessao, versaoID int64) error {
	if versaoID <= sess.Conta.NovidadeVista {
		return nil
	}
	loja, err := s.loja(sess.Conta.Tipo)
	if err != nil {
		return err
	}
	if err := loja.MarcarNovidadeVista(ctx, sess.Conta.ID, versaoID); err != nil {
		return err
	}
	sess.Conta.NovidadeVista = versaoID
	return nil
}
