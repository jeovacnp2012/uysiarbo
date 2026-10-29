// Package geo lê e valida GeoJSON de polígonos (limites de bairros e quarteirões) e grava no PostGIS.
// A leitura é feita em Go (formato, tipos, faixa das coordenadas); a validade topológica fica com o PostGIS.
package geo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// TamanhoMaximo limita o texto aceito (um bairro grande detalhado cabe com folga).
const TamanhoMaximo = 4 << 20

// MaximoDeFeicoes limita quantos polígonos entram numa importação de quarteirões.
const MaximoDeFeicoes = 2000

// Feicao é um polígono lido do GeoJSON, com o nome que veio nas propriedades (se veio).
type Feicao struct {
	Nome      string
	Geometria []byte // GeoJSON de Polygon ou MultiPolygon
}

type bruto struct {
	Type       string            `json:"type"`
	Features   []json.RawMessage `json:"features"`
	Geometry   json.RawMessage   `json:"geometry"`
	Properties map[string]any    `json:"properties"`
	Coords     json.RawMessage   `json:"coordinates"`
}

// chavesDeNome são as propriedades onde procuramos o nome do quarteirão, na ordem.
var chavesDeNome = []string{"nome", "name", "quarteirao", "quadra", "id", "codigo"}

// Ler aceita uma Geometry, uma Feature ou uma FeatureCollection e devolve só os polígonos.
func Ler(texto string) ([]Feicao, error) {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return nil, errors.New("cole o conteúdo do arquivo GeoJSON")
	}
	if len(texto) > TamanhoMaximo {
		return nil, fmt.Errorf("o GeoJSON é grande demais (máximo %d MB)", TamanhoMaximo>>20)
	}
	var b bruto
	if err := json.Unmarshal([]byte(texto), &b); err != nil {
		return nil, errors.New("o texto não é um GeoJSON válido (JSON mal formado)")
	}
	var out []Feicao
	switch b.Type {
	case "FeatureCollection":
		if len(b.Features) > MaximoDeFeicoes {
			return nil, fmt.Errorf("polígonos demais de uma vez: o limite é %d", MaximoDeFeicoes)
		}
		for i, f := range b.Features {
			fe, err := daFeicao(f)
			if err != nil {
				return nil, fmt.Errorf("item %d: %w", i+1, err)
			}
			out = append(out, fe)
		}
	case "Feature":
		fe, err := daFeicao([]byte(texto))
		if err != nil {
			return nil, err
		}
		out = append(out, fe)
	case "Polygon", "MultiPolygon":
		fe, err := daGeometria([]byte(texto), "")
		if err != nil {
			return nil, err
		}
		out = append(out, fe)
	case "":
		return nil, errors.New("o GeoJSON não tem o campo \"type\"")
	default:
		return nil, fmt.Errorf("tipo %q não é aceito: envie Polygon, MultiPolygon, Feature ou FeatureCollection", b.Type)
	}
	if len(out) == 0 {
		return nil, errors.New("o GeoJSON não tem nenhum polígono")
	}
	return out, nil
}

func daFeicao(raw []byte) (Feicao, error) {
	var f bruto
	if err := json.Unmarshal(raw, &f); err != nil || f.Type != "Feature" {
		return Feicao{}, errors.New("não é uma Feature de GeoJSON")
	}
	if len(f.Geometry) == 0 || bytes.Equal(bytes.TrimSpace(f.Geometry), []byte("null")) {
		return Feicao{}, errors.New("a feição não tem geometria")
	}
	return daGeometria(f.Geometry, nomeDas(f.Properties))
}

func nomeDas(p map[string]any) string {
	for _, k := range chavesDeNome {
		for chave, v := range p {
			if !strings.EqualFold(chave, k) {
				continue
			}
			switch x := v.(type) {
			case string:
				if s := strings.TrimSpace(x); s != "" {
					return s
				}
			case float64:
				if x == float64(int64(x)) {
					return fmt.Sprintf("%d", int64(x))
				}
			}
		}
	}
	return ""
}

func daGeometria(raw []byte, nome string) (Feicao, error) {
	var g bruto
	if err := json.Unmarshal(raw, &g); err != nil {
		return Feicao{}, errors.New("geometria mal formada")
	}
	if g.Type != "Polygon" && g.Type != "MultiPolygon" {
		return Feicao{}, fmt.Errorf("a geometria %q não é aceita: só Polygon e MultiPolygon", g.Type)
	}
	var coords any
	if err := json.Unmarshal(g.Coords, &coords); err != nil {
		return Feicao{}, errors.New("coordenadas mal formadas")
	}
	profundidade := 2 // Polygon: anéis > pontos > [x,y]
	if g.Type == "MultiPolygon" {
		profundidade = 3
	}
	if err := conferirCoordenadas(coords, profundidade); err != nil {
		return Feicao{}, err
	}
	return Feicao{Nome: nome, Geometria: raw}, nil
}

// conferirCoordenadas confere a estrutura (listas aninhadas até [x, y]) e a faixa de longitude/latitude.
func conferirCoordenadas(v any, nivel int) error {
	lista, ok := v.([]any)
	if !ok {
		return errors.New("coordenadas fora do formato do GeoJSON")
	}
	if nivel == 0 {
		if len(lista) < 2 {
			return errors.New("ponto sem longitude e latitude")
		}
		x, okx := lista[0].(float64)
		y, oky := lista[1].(float64)
		if !okx || !oky {
			return errors.New("coordenada não numérica")
		}
		if x < -180 || x > 180 || y < -90 || y > 90 {
			return fmt.Errorf("coordenada fora do mundo (%.6f, %.6f): o GeoJSON deve estar em longitude/latitude (WGS 84)", x, y)
		}
		return nil
	}
	if len(lista) == 0 {
		return errors.New("polígono vazio")
	}
	for _, item := range lista {
		if err := conferirCoordenadas(item, nivel-1); err != nil {
			return err
		}
	}
	return nil
}
