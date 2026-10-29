package tenant

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

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
