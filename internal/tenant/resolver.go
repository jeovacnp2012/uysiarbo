package tenant

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"uysiarbo/internal/db/sqlc"
)

// Kind diz a que um endereço (host) pertence.
type Kind int

const (
	Unknown Kind = iota // endereço que não é nosso
	Admin               // console da plataforma, sem cliente
	Client              // endereço de um cliente
)

// Resolver descobre o cliente de uma requisição pelo endereço. Guarda o resultado por pouco tempo
// em memória, para não consultar o banco a cada requisição.
type Resolver struct {
	Pool       *pgxpool.Pool
	BaseDomain string // "<subdominio>.<BaseDomain>" é o endereço de um cliente
	AdminHost  string
	TTL        time.Duration // padrão 30 s

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	cliente sqlc.Cliente
	kind    Kind
	em      time.Time
}

// NormalizeHost tira a porta, o ponto final e as maiúsculas.
func NormalizeHost(host string) string {
	if i := strings.LastIndex(host, ":"); i >= 0 && !strings.Contains(host[i:], "]") {
		host = host[:i]
	}
	return strings.ToLower(strings.TrimSuffix(host, "."))
}

// Lookup resolve o endereço. Um endereço desconhecido não é erro: volta Unknown.
func (r *Resolver) Lookup(ctx context.Context, host string) (sqlc.Cliente, Kind, error) {
	host = NormalizeHost(host)
	if host == strings.ToLower(r.AdminHost) {
		return sqlc.Cliente{}, Admin, nil
	}

	ttl := r.TTL
	if ttl == 0 {
		ttl = 30 * time.Second
	}
	r.mu.Lock()
	if c, ok := r.cache[host]; ok && time.Since(c.em) < ttl {
		r.mu.Unlock()
		return c.cliente, c.kind, nil
	}
	r.mu.Unlock()

	cliente, kind, err := r.consultar(ctx, host)
	if err != nil {
		return sqlc.Cliente{}, Unknown, err
	}
	r.mu.Lock()
	if r.cache == nil {
		r.cache = map[string]cached{}
	}
	r.cache[host] = cached{cliente: cliente, kind: kind, em: time.Now()}
	r.mu.Unlock()
	return cliente, kind, nil
}

func (r *Resolver) consultar(ctx context.Context, host string) (sqlc.Cliente, Kind, error) {
	q := sqlc.New(r.Pool)

	if sub, ok := strings.CutSuffix(host, "."+strings.ToLower(r.BaseDomain)); ok && sub != "" && !strings.Contains(sub, ".") {
		c, err := q.GetClienteBySubdominio(ctx, sub)
		if err == nil {
			return c, Client, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Cliente{}, Unknown, err
		}
	}
	c, err := q.GetClienteByDominio(ctx, pgtype.Text{String: host, Valid: true})
	if err == nil {
		return c, Client, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Cliente{}, Unknown, nil
	}
	return sqlc.Cliente{}, Unknown, err
}

// Host devolve o endereço completo de um cliente.
func (r *Resolver) Host(c sqlc.Cliente) string {
	if c.Dominio.Valid {
		return c.Dominio.String
	}
	return c.Subdominio + "." + r.BaseDomain
}

type ctxKey struct{}

// WithCliente guarda o cliente da requisição no contexto.
func WithCliente(ctx context.Context, c sqlc.Cliente) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// FromContext devolve o cliente da requisição; ok é falso no console da plataforma.
func FromContext(ctx context.Context) (sqlc.Cliente, bool) {
	c, ok := ctx.Value(ctxKey{}).(sqlc.Cliente)
	return c, ok
}
