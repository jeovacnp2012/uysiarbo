package geo

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX é o que o pool e a transação do pgx têm em comum.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ErrGeometriaInvalida: o PostGIS não conseguiu transformar o texto num polígono utilizável.
var ErrGeometriaInvalida = errors.New("polígono inválido")

// Resumo descreve o limite gravado, para mostrar na ficha.
type Resumo struct {
	GeoJSON   string
	AreaHa    float64 // hectares
	Latitude  float64 // um ponto dentro do polígono
	Longitude float64
}

// normalizada é a expressão que corrige o polígono e o guarda como MultiPolygon em WGS 84.
// ST_MakeValid conserta laços e anéis abertos; ST_CollectionExtract(…, 3) descarta o que não for polígono.
const normalizada = `ST_Multi(ST_CollectionExtract(ST_MakeValid(ST_SetSRID(ST_GeomFromGeoJSON($2::text), 4326)), 3))`

func gravar(ctx context.Context, db DBTX, tabela, coluna string, id int64, geojson []byte) error {
	// tabela e coluna vêm só do código (constantes abaixo), nunca de entrada de usuário.
	tag, err := db.Exec(ctx, `
INSERT INTO `+tabela+` (`+coluna+`, geom)
SELECT $1::bigint, g FROM (SELECT `+normalizada+` AS g) x
WHERE NOT ST_IsEmpty(g)
ON CONFLICT (`+coluna+`) DO UPDATE SET geom = EXCLUDED.geom, atualizado_em = now()`, id, string(geojson))
	if err != nil {
		var pe *pgconn.PgError
		// o PostGIS recusa GeoJSON impossível com erro interno (XX), de dados (22) ou da regra de validade (23514)
		if errors.As(err, &pe) && (strings.HasPrefix(pe.Code, "XX") || strings.HasPrefix(pe.Code, "22") || pe.Code == "23514") {
			return ErrGeometriaInvalida
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrGeometriaInvalida
	}
	return nil
}

// GravarBairro cria ou substitui o limite do bairro.
func GravarBairro(ctx context.Context, db DBTX, bairroID int64, geojson []byte) error {
	return gravar(ctx, db, "bairro_geometrias", "bairro_id", bairroID, geojson)
}

// GravarQuarteirao cria ou substitui o limite do quarteirão.
func GravarQuarteirao(ctx context.Context, db DBTX, quarteiraoID int64, geojson []byte) error {
	return gravar(ctx, db, "quarteirao_geometrias", "quarteirao_id", quarteiraoID, geojson)
}

const resumoSQL = `SELECT ST_AsGeoJSON(geom, 6), ST_Area(geom::geography) / 10000.0,
       ST_Y(ST_PointOnSurface(geom)), ST_X(ST_PointOnSurface(geom))
FROM `

// BairroResumo devolve o limite do bairro; ok=false se ele não tem limite.
func BairroResumo(ctx context.Context, db DBTX, bairroID int64) (Resumo, bool, error) {
	var r Resumo
	err := db.QueryRow(ctx, resumoSQL+`bairro_geometrias WHERE bairro_id = $1`, bairroID).Scan(&r.GeoJSON, &r.AreaHa, &r.Latitude, &r.Longitude)
	if errors.Is(err, pgx.ErrNoRows) {
		return Resumo{}, false, nil
	}
	return r, err == nil, err
}

// RemoverBairro apaga o limite; devolve se havia algo para apagar.
func RemoverBairro(ctx context.Context, db DBTX, bairroID int64) (bool, error) {
	tag, err := db.Exec(ctx, `DELETE FROM bairro_geometrias WHERE bairro_id = $1`, bairroID)
	return tag.RowsAffected() > 0, err
}

// QuarteiroesComLimite devolve, dos quarteirões do bairro, o conjunto dos que têm limite gravado.
func QuarteiroesComLimite(ctx context.Context, db DBTX, bairroID int64) (map[int64]bool, error) {
	rows, err := db.Query(ctx, `
SELECT g.quarteirao_id FROM quarteirao_geometrias g
JOIN quarteiroes q ON q.id = g.quarteirao_id WHERE q.bairro_id = $1`, bairroID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// PontoNoBairro diz se o ponto cai dentro do limite gravado do bairro; tem=false se o bairro não tem limite.
func PontoNoBairro(ctx context.Context, db DBTX, bairroID int64, lon, lat float64) (dentro, tem bool, err error) {
	err = db.QueryRow(ctx, `SELECT ST_Contains(geom, ST_SetSRID(ST_MakePoint($2, $3), 4326)) FROM bairro_geometrias WHERE bairro_id = $1`,
		bairroID, lon, lat).Scan(&dentro)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	return dentro, err == nil, err
}
