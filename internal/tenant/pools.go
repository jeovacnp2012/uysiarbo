package tenant

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"uysiarbo/internal/db"
)

// Pools mantém um pool de conexões por schema de cliente. Toda conexão de um pool nasce com
// search_path = "<schema>, public" (definido na conexão, não por consulta): o código do cliente A
// nunca enxerga as tabelas do cliente B, mesmo que alguém esqueça um filtro.
type Pools struct {
	URL      string
	MaxConns int32 // por cliente; padrão 4

	mu    sync.Mutex
	pools map[string]*pgxpool.Pool
}

func (p *Pools) Get(ctx context.Context, schema string) (*pgxpool.Pool, error) {
	if !db.SchemaClienteRe.MatchString(schema) {
		return nil, fmt.Errorf("nome de schema inválido: %q", schema)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if pool, ok := p.pools[schema]; ok {
		return pool, nil
	}

	cfg, err := pgxpool.ParseConfig(p.URL)
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = pgx.Identifier{schema}.Sanitize() + ", public"
	cfg.MaxConns = p.MaxConns
	if cfg.MaxConns == 0 {
		cfg.MaxConns = 4
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if p.pools == nil {
		p.pools = map[string]*pgxpool.Pool{}
	}
	p.pools[schema] = pool
	return pool, nil
}

func (p *Pools) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, pool := range p.pools {
		pool.Close()
	}
	p.pools = nil
}
