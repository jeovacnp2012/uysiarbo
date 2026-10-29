package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/db/sqlccliente"
)

func tempo(t pgtype.Timestamptz) time.Time {
	if t.Valid {
		return t.Time
	}
	return time.Time{}
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func uuidString(u pgtype.UUID) string {
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	return u, u.Scan(s)
}

func traduz(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNaoEncontrada
	}
	return err
}

// ---- Plataforma (schema public) ----

// PlataformaContas guarda as contas da plataforma. Com SoMaster, só os masters contam (é o que vale
// no endereço de um cliente: lá o master entra, os outros usuários da plataforma não).
type PlataformaContas struct {
	Q        *sqlc.Queries
	SoMaster bool
}

func (PlataformaContas) Tipo() Tipo { return TipoPlataforma }

func (p PlataformaContas) conta(u sqlc.PlataformaUsuario) (Conta, error) {
	if p.SoMaster && !u.Master {
		return Conta{}, ErrNaoEncontrada
	}
	return Conta{
		ID: uuidString(u.ID), Tipo: TipoPlataforma, Nome: u.Nome, Email: u.Email, SenhaHash: u.SenhaHash,
		Ativo: u.Ativo, DeveTrocarSenha: u.DeveTrocarSenha, BloqueadoAte: tempo(u.BloqueadoAte), Master: u.Master,
	}, nil
}

func (p PlataformaContas) PorEmail(ctx context.Context, email string) (Conta, error) {
	u, err := p.Q.GetPlataformaUsuarioByEmail(ctx, email)
	if err != nil {
		return Conta{}, traduz(err)
	}
	return p.conta(u)
}

func (p PlataformaContas) PorID(ctx context.Context, id string) (Conta, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return Conta{}, ErrNaoEncontrada
	}
	u, err := p.Q.GetPlataformaUsuarioByID(ctx, uid)
	if err != nil {
		return Conta{}, traduz(err)
	}
	return p.conta(u)
}

func (p PlataformaContas) RegistrarFalha(ctx context.Context, id string, limite int, bloqueio time.Duration) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}
	return p.Q.RegistrarFalhaPlataforma(ctx, sqlc.RegistrarFalhaPlataformaParams{
		Limite: int32(limite), BloqueioSegundos: int32(bloqueio.Seconds()), ID: uid,
	})
}

func (p PlataformaContas) RegistrarSucesso(ctx context.Context, id string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}
	return p.Q.RegistrarSucessoPlataforma(ctx, uid)
}

func (p PlataformaContas) DefinirSenha(ctx context.Context, id, hash string, deveTrocar bool) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}
	_, err = p.Q.SetPlataformaUsuarioSenha(ctx, sqlc.SetPlataformaUsuarioSenhaParams{ID: uid, SenhaHash: hash, DeveTrocarSenha: deveTrocar})
	return err
}

// Permissoes: o master não depende de permissões.
func (PlataformaContas) Permissoes(context.Context, Conta) ([]string, error) { return nil, nil }

// PlataformaSessoes guarda as sessões do console.
type PlataformaSessoes struct{ Q *sqlc.Queries }

func (s PlataformaSessoes) Criar(ctx context.Context, hash []byte, _ Tipo, contaID string, expira time.Time, ip, ua string) error {
	uid, err := parseUUID(contaID)
	if err != nil {
		return err
	}
	return s.Q.CreateSessaoPlataforma(ctx, sqlc.CreateSessaoPlataformaParams{TokenHash: hash, UsuarioID: uid, ExpiraEm: ts(expira), Ip: ip, UserAgent: ua})
}

func (s PlataformaSessoes) Buscar(ctx context.Context, hash []byte) (SessaoRegistro, error) {
	r, err := s.Q.GetSessaoPlataforma(ctx, hash)
	if err != nil {
		return SessaoRegistro{}, traduz(err)
	}
	return SessaoRegistro{Tipo: TipoPlataforma, ContaID: uuidString(r.UsuarioID), Expira: r.ExpiraEm.Time}, nil
}

func (s PlataformaSessoes) Renovar(ctx context.Context, hash []byte, expira time.Time) error {
	return s.Q.TouchSessaoPlataforma(ctx, sqlc.TouchSessaoPlataformaParams{TokenHash: hash, ExpiraEm: ts(expira)})
}

func (s PlataformaSessoes) Remover(ctx context.Context, hash []byte) error {
	return s.Q.DeleteSessaoPlataforma(ctx, hash)
}

func (s PlataformaSessoes) RemoverDaConta(ctx context.Context, _ Tipo, contaID string, manter []byte) error {
	uid, err := parseUUID(contaID)
	if err != nil {
		return err
	}
	return s.Q.DeleteSessoesDoUsuarioPlataforma(ctx, sqlc.DeleteSessoesDoUsuarioPlataformaParams{UsuarioID: uid, TokenHash: manter})
}

// ---- Cliente (schema c_<slug>) ----

// ClienteContas guarda os usuários de um cliente. Q deve usar o pool do schema do cliente.
type ClienteContas struct{ Q *sqlccliente.Queries }

func (ClienteContas) Tipo() Tipo { return TipoUsuario }

func conta(u sqlccliente.Usuario) Conta {
	return Conta{
		ID: strconv.FormatInt(u.ID, 10), Tipo: TipoUsuario, Nome: u.Nome, Email: u.Email, SenhaHash: u.SenhaHash,
		Ativo: u.Ativo, DeveTrocarSenha: u.DeveTrocarSenha, BloqueadoAte: tempo(u.BloqueadoAte), PapelID: u.PapelID,
	}
}

func (c ClienteContas) PorEmail(ctx context.Context, email string) (Conta, error) {
	u, err := c.Q.GetUsuarioByEmail(ctx, email)
	if err != nil {
		return Conta{}, traduz(err)
	}
	return conta(u), nil
}

func (c ClienteContas) PorID(ctx context.Context, id string) (Conta, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return Conta{}, ErrNaoEncontrada
	}
	u, err := c.Q.GetUsuarioByID(ctx, n)
	if err != nil {
		return Conta{}, traduz(err)
	}
	return conta(u), nil
}

func (c ClienteContas) RegistrarFalha(ctx context.Context, id string, limite int, bloqueio time.Duration) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return err
	}
	return c.Q.RegistrarFalhaUsuario(ctx, sqlccliente.RegistrarFalhaUsuarioParams{Limite: int32(limite), BloqueioSegundos: int32(bloqueio.Seconds()), ID: n})
}

func (c ClienteContas) RegistrarSucesso(ctx context.Context, id string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return err
	}
	return c.Q.RegistrarSucessoUsuario(ctx, n)
}

func (c ClienteContas) DefinirSenha(ctx context.Context, id, hash string, deveTrocar bool) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return err
	}
	return c.Q.SetUsuarioSenha(ctx, sqlccliente.SetUsuarioSenhaParams{ID: n, SenhaHash: hash, DeveTrocarSenha: deveTrocar})
}

func (c ClienteContas) Permissoes(ctx context.Context, co Conta) ([]string, error) {
	return c.Q.ListPermissoesDoPapel(ctx, co.PapelID)
}

// ClienteSessoes guarda as sessões num cliente: de usuários do cliente e do master.
type ClienteSessoes struct{ Q *sqlccliente.Queries }

func (s ClienteSessoes) Criar(ctx context.Context, hash []byte, tipo Tipo, contaID string, expira time.Time, ip, ua string) error {
	if tipo == TipoPlataforma {
		uid, err := parseUUID(contaID)
		if err != nil {
			return err
		}
		return s.Q.CreateSessaoMaster(ctx, sqlccliente.CreateSessaoMasterParams{TokenHash: hash, MasterID: uid, ExpiraEm: ts(expira), Ip: ip, UserAgent: ua})
	}
	n, err := strconv.ParseInt(contaID, 10, 64)
	if err != nil {
		return err
	}
	return s.Q.CreateSessaoUsuario(ctx, sqlccliente.CreateSessaoUsuarioParams{TokenHash: hash, UsuarioID: pgtype.Int8{Int64: n, Valid: true}, ExpiraEm: ts(expira), Ip: ip, UserAgent: ua})
}

func (s ClienteSessoes) Buscar(ctx context.Context, hash []byte) (SessaoRegistro, error) {
	r, err := s.Q.GetSessao(ctx, hash)
	if err != nil {
		return SessaoRegistro{}, traduz(err)
	}
	if r.MasterID.Valid {
		return SessaoRegistro{Tipo: TipoPlataforma, ContaID: uuidString(r.MasterID), Expira: r.ExpiraEm.Time}, nil
	}
	return SessaoRegistro{Tipo: TipoUsuario, ContaID: strconv.FormatInt(r.UsuarioID.Int64, 10), Expira: r.ExpiraEm.Time}, nil
}

func (s ClienteSessoes) Renovar(ctx context.Context, hash []byte, expira time.Time) error {
	return s.Q.TouchSessao(ctx, sqlccliente.TouchSessaoParams{TokenHash: hash, ExpiraEm: ts(expira)})
}

func (s ClienteSessoes) Remover(ctx context.Context, hash []byte) error {
	return s.Q.DeleteSessao(ctx, hash)
}

func (s ClienteSessoes) RemoverDaConta(ctx context.Context, tipo Tipo, contaID string, manter []byte) error {
	if tipo == TipoPlataforma {
		uid, err := parseUUID(contaID)
		if err != nil {
			return err
		}
		return s.Q.DeleteSessoesDoMaster(ctx, sqlccliente.DeleteSessoesDoMasterParams{MasterID: uid, TokenHash: manter})
	}
	n, err := strconv.ParseInt(contaID, 10, 64)
	if err != nil {
		return err
	}
	return s.Q.DeleteSessoesDoUsuario(ctx, sqlccliente.DeleteSessoesDoUsuarioParams{UsuarioID: pgtype.Int8{Int64: n, Valid: true}, TokenHash: manter})
}
