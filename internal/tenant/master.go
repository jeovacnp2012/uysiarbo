package tenant

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/auth"
	"uysiarbo/internal/db/sqlc"
)

var ErrMasterJaExiste = errors.New("já existe um usuário com esse e-mail")

// CriarMaster cria um usuário master da plataforma com uma senha forte gerada. A senha em texto
// puro é devolvida UMA vez, para quem criou entregar; só o hash fica guardado, e a pessoa é obrigada
// a escolher a própria senha no primeiro login.
func (s *Service) CriarMaster(ctx context.Context, nome, email string) (sqlc.PlataformaUsuario, string, error) {
	nome, email = strings.TrimSpace(nome), strings.TrimSpace(email)
	if nome == "" || !strings.Contains(email, "@") {
		return sqlc.PlataformaUsuario{}, "", errors.New("informe nome e um e-mail válido")
	}
	q := sqlc.New(s.Pool)
	if _, err := q.GetPlataformaUsuarioByEmail(ctx, email); err == nil {
		return sqlc.PlataformaUsuario{}, "", ErrMasterJaExiste
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.PlataformaUsuario{}, "", err
	}

	senha, hash, err := novaSenha()
	if err != nil {
		return sqlc.PlataformaUsuario{}, "", err
	}
	u, err := q.CreatePlataformaUsuario(ctx, sqlc.CreatePlataformaUsuarioParams{
		Nome: nome, Email: email, SenhaHash: hash, Master: true, DeveTrocarSenha: true,
	})
	if err != nil {
		return sqlc.PlataformaUsuario{}, "", fmt.Errorf("criar master: %w", err)
	}
	s.auditar(ctx, audit.Evento{
		Acao: "master.criar", Entidade: "master", EntidadeID: u.Email, Rotulo: u.Nome,
		Depois: map[string]any{"nome": u.Nome, "email": u.Email, "master": true, "troca_obrigatoria": true},
	})
	return u, senha, nil
}

// ResetarSenhaPlataforma gera uma nova senha forte para o usuário da plataforma, devolvida uma vez.
// A senha antiga deixa de valer e a troca no próximo login passa a ser obrigatória.
func (s *Service) ResetarSenhaPlataforma(ctx context.Context, email string) (sqlc.PlataformaUsuario, string, error) {
	q := sqlc.New(s.Pool)
	u, err := q.GetPlataformaUsuarioByEmail(ctx, strings.TrimSpace(email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.PlataformaUsuario{}, "", errors.New("usuário não encontrado")
		}
		return sqlc.PlataformaUsuario{}, "", err
	}
	senha, hash, err := novaSenha()
	if err != nil {
		return sqlc.PlataformaUsuario{}, "", err
	}
	u, err = q.SetPlataformaUsuarioSenha(ctx, sqlc.SetPlataformaUsuarioSenhaParams{ID: u.ID, SenhaHash: hash, DeveTrocarSenha: true})
	if err == nil {
		err = s.EncerrarSessoes(ctx, uuidTexto(u.ID.Bytes))
	}
	if err == nil {
		s.auditar(ctx, audit.Evento{
			Acao: "master.resetar_senha", Entidade: "master", EntidadeID: u.Email, Rotulo: u.Nome,
			Detalhes: map[string]any{"troca_obrigatoria": true},
		})
	}
	return u, senha, err
}

func novaSenha() (senha, hash string, err error) {
	if senha, err = auth.Gerar(16); err != nil {
		return "", "", err
	}
	if hash, err = auth.Hash(senha); err != nil {
		return "", "", err
	}
	return senha, hash, nil
}

// EncerrarSessoes derruba todas as sessões abertas de uma conta da plataforma: a do console e as que
// ela abriu dentro de cada cliente. Use depois de resetar a senha.
func (s *Service) EncerrarSessoes(ctx context.Context, contaID string) error {
	if _, err := s.Pool.Exec(ctx, `DELETE FROM sessoes_plataforma WHERE usuario_id = $1::uuid`, contaID); err != nil {
		return err
	}
	clientes, err := sqlc.New(s.Pool).ListClientes(ctx)
	if err != nil {
		return err
	}
	for _, c := range clientes {
		if c.Status == "provisionando" || c.Status == "cancelado" {
			continue
		}
		sql := "DELETE FROM " + pgx.Identifier{c.SchemaNome, "sessoes"}.Sanitize() + " WHERE master_id = $1::uuid"
		if _, err := s.Pool.Exec(ctx, sql, contaID); err != nil {
			return fmt.Errorf("encerrar sessões em %s: %w", c.Slug, err)
		}
	}
	return nil
}

func uuidTexto(b [16]byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
