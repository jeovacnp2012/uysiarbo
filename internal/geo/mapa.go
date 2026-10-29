package geo

import (
	"context"
	"encoding/json"
)

// Escopo repete, para o mapa, os limites do papel: o mesmo que vale nas listas.
type Escopo struct {
	Somente        *int64 // só os bairros em que este usuário é responsável (nil = todos)
	SoPE           bool   // só pontos estratégicos
	VerDesativados bool
}

// Caixa é o retângulo visível do mapa (oeste, sul, leste, norte), em graus.
type Caixa struct{ O, S, L, N float64 }

// LimiteDeLocais é quantos pontos um pedido do mapa devolve, no máximo.
const LimiteDeLocais = 5000

// Colecao é um GeoJSON FeatureCollection pronto para o MapLibre.
type Colecao struct {
	Type     string       `json:"type"`
	Features []FeicaoMapa `json:"features"`
	Truncado bool         `json:"truncado,omitempty"`
	SemPonto int          `json:"sem_ponto,omitempty"` // locais do filtro sem latitude/longitude
}

// FeicaoMapa é uma feição de saída (o nome Feicao já é do leitor de GeoJSON).
type FeicaoMapa struct {
	Type       string          `json:"type"`
	ID         int64           `json:"id"`
	Properties map[string]any  `json:"properties"`
	Geometry   json.RawMessage `json:"geometry"`
}

func novaColecao() Colecao { return Colecao{Type: "FeatureCollection", Features: []FeicaoMapa{}} }

// BairrosGeoJSON devolve os contornos dos bairros (simplificados) que a pessoa pode ver.
func BairrosGeoJSON(ctx context.Context, db DBTX, e Escopo, bairroID int64) (Colecao, error) {
	rows, err := db.Query(ctx, `
SELECT b.id, b.nome, ST_AsGeoJSON(ST_SimplifyPreserveTopology(g.geom, 0.00003), 6)
FROM bairro_geometrias g JOIN bairros b ON b.id = g.bairro_id
WHERE ($1::bigint IS NULL OR usuario_ve_bairro($1::bigint, b.id))
  AND ($2::bigint = 0 OR b.id = $2::bigint)
ORDER BY b.id`, e.Somente, bairroID)
	if err != nil {
		return Colecao{}, err
	}
	defer rows.Close()
	col := novaColecao()
	for rows.Next() {
		var id int64
		var nome, geom string
		if err := rows.Scan(&id, &nome, &geom); err != nil {
			return col, err
		}
		col.Features = append(col.Features, FeicaoMapa{Type: "Feature", ID: id, Properties: map[string]any{"id": id, "nome": nome}, Geometry: json.RawMessage(geom)})
	}
	return col, rows.Err()
}

// QuarteiroesGeoJSON devolve os contornos dos quarteirões ativos dentro da caixa.
func QuarteiroesGeoJSON(ctx context.Context, db DBTX, e Escopo, caixa Caixa, bairroID int64) (Colecao, error) {
	rows, err := db.Query(ctx, `
SELECT q.id, q.nome, b.id, b.nome, ST_AsGeoJSON(g.geom, 6)
FROM quarteirao_geometrias g
JOIN quarteiroes q ON q.id = g.quarteirao_id AND q.ativo
JOIN bairros b ON b.id = q.bairro_id
WHERE g.geom && ST_MakeEnvelope($1, $2, $3, $4, 4326)
  AND ($5::bigint IS NULL OR usuario_ve_quarteirao($5::bigint, q.id, b.id))
  AND ($6::bigint = 0 OR b.id = $6::bigint)
ORDER BY q.id LIMIT $7`, caixa.O, caixa.S, caixa.L, caixa.N, e.Somente, bairroID, LimiteDeLocais+1)
	if err != nil {
		return Colecao{}, err
	}
	defer rows.Close()
	col := novaColecao()
	for rows.Next() {
		var id, bid int64
		var nome, bnome, geom string
		if err := rows.Scan(&id, &nome, &bid, &bnome, &geom); err != nil {
			return col, err
		}
		if len(col.Features) == LimiteDeLocais {
			col.Truncado = true
			break
		}
		col.Features = append(col.Features, FeicaoMapa{Type: "Feature", ID: id, Properties: map[string]any{"id": id, "nome": nome, "bairro_id": bid, "bairro": bnome}, Geometry: json.RawMessage(geom)})
	}
	return col, rows.Err()
}

// FiltroDeLocais são as escolhas da tela do mapa para os pontos.
type FiltroDeLocais struct {
	BairroID int64  // 0 = todos
	GrupoID  int64  // 0 = todos
	Tipo     string // "" = todos
	LocalID  int64  // 0 = todos (usado para focar um local)
}

const condicoesDeLocais = `
  AND ($1::bigint IS NULL OR usuario_ve_local($1::bigint, l.id, l.bairro_id, l.quarteirao_id))
  AND (NOT $2::boolean OR l.tipo = 'ponto_estrategico')
  AND ($3::boolean OR l.ativo)
  AND ($4::bigint = 0 OR l.bairro_id = $4::bigint)
  AND ($5::text = '' OR l.tipo = $5::text)
  AND ($6::bigint = 0 OR EXISTS (SELECT 1 FROM local_grupo_itens i WHERE i.grupo_id = $6::bigint AND i.local_id = l.id))
  AND ($7::bigint = 0 OR l.id = $7::bigint)`

// LocaisGeoJSON devolve os locais com coordenadas dentro da caixa (no máximo LimiteDeLocais).
// SemPonto conta os locais do mesmo filtro que não aparecem por não terem latitude e longitude.
func LocaisGeoJSON(ctx context.Context, db DBTX, e Escopo, caixa Caixa, f FiltroDeLocais) (Colecao, error) {
	col := novaColecao()
	if err := db.QueryRow(ctx, `SELECT count(*)::int FROM locais l WHERE l.latitude IS NULL`+condicoesDeLocais,
		e.Somente, e.SoPE, e.VerDesativados, f.BairroID, f.Tipo, f.GrupoID, f.LocalID).Scan(&col.SemPonto); err != nil {
		return col, err
	}
	rows, err := db.Query(ctx, `
SELECT l.id, l.tipo, l.endereco || ', ' || l.numero, l.ativo, b.nome, l.longitude, l.latitude
FROM locais l JOIN bairros b ON b.id = l.bairro_id
WHERE l.latitude IS NOT NULL
  AND ST_SetSRID(ST_MakePoint(l.longitude, l.latitude), 4326) && ST_MakeEnvelope($8, $9, $10, $11, 4326)`+condicoesDeLocais+`
ORDER BY l.id LIMIT $12`,
		e.Somente, e.SoPE, e.VerDesativados, f.BairroID, f.Tipo, f.GrupoID, f.LocalID, caixa.O, caixa.S, caixa.L, caixa.N, LimiteDeLocais+1)
	if err != nil {
		return col, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var tipo, endereco, bairro string
		var ativo bool
		var lon, lat float64
		if err := rows.Scan(&id, &tipo, &endereco, &ativo, &bairro, &lon, &lat); err != nil {
			return col, err
		}
		if len(col.Features) == LimiteDeLocais {
			col.Truncado = true
			break
		}
		ponto, _ := json.Marshal(map[string]any{"type": "Point", "coordinates": []float64{lon, lat}})
		col.Features = append(col.Features, FeicaoMapa{
			Type: "Feature", ID: id, Geometry: ponto,
			Properties: map[string]any{"id": id, "tipo": tipo, "endereco": endereco, "ativo": ativo, "bairro": bairro},
		})
	}
	return col, rows.Err()
}
