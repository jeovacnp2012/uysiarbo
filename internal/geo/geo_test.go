package geo

import (
	"strings"
	"testing"
)

const quadrado = `{"type":"Polygon","coordinates":[[[-56.09,-13.83],[-56.07,-13.83],[-56.07,-13.81],[-56.09,-13.81],[-56.09,-13.83]]]}`

func TestLerAceitaGeometriaFeatureEColecao(t *testing.T) {
	feat := `{"type":"Feature","properties":{"Nome":" Q1 "},"geometry":` + quadrado + `}`
	colecao := `{"type":"FeatureCollection","features":[` + feat + `,{"type":"Feature","properties":{"quadra":7},"geometry":` + strings.Replace(quadrado, "Polygon", "MultiPolygon", 1) + `}]}`
	multi := `{"type":"MultiPolygon","coordinates":[[[[-56.09,-13.83],[-56.07,-13.83],[-56.07,-13.81],[-56.09,-13.83]]]]}`

	if fs, err := Ler(quadrado); err != nil || len(fs) != 1 || fs[0].Nome != "" {
		t.Errorf("geometria: %+v %v", fs, err)
	}
	if fs, err := Ler(multi); err != nil || len(fs) != 1 {
		t.Errorf("multipolígono: %+v %v", fs, err)
	}
	if fs, err := Ler(feat); err != nil || len(fs) != 1 || fs[0].Nome != "Q1" {
		t.Errorf("feature: %+v %v", fs, err)
	}
	// a segunda feição do exemplo declara MultiPolygon com coordenadas de Polygon: tem de ser recusada
	if _, err := Ler(colecao); err == nil {
		t.Error("MultiPolygon com a profundidade de um Polygon deveria ser recusado")
	}
	boa := `{"type":"FeatureCollection","features":[` + feat + `,{"type":"Feature","properties":{"quadra":7},"geometry":` + quadrado + `}]}`
	fs, err := Ler(boa)
	if err != nil || len(fs) != 2 || fs[0].Nome != "Q1" || fs[1].Nome != "7" {
		t.Errorf("coleção: %+v %v", fs, err)
	}
}

func TestLerRecusaEntradasRuins(t *testing.T) {
	casos := map[string]string{
		"vazio":            "  ",
		"json ruim":        "{oi",
		"sem tipo":         `{"coordinates":[]}`,
		"ponto":            `{"type":"Point","coordinates":[-56,-13]}`,
		"linha":            `{"type":"Feature","properties":{},"geometry":{"type":"LineString","coordinates":[[0,0],[1,1]]}}`,
		"sem geometria":    `{"type":"Feature","properties":{},"geometry":null}`,
		"fora do mundo":    `{"type":"Polygon","coordinates":[[[-5600000,-13],[1,1],[2,2],[-5600000,-13]]]}`,
		"utm (metros)":     `{"type":"Polygon","coordinates":[[[600000,8500000],[600100,8500000],[600100,8500100],[600000,8500000]]]}`,
		"coleção vazia":    `{"type":"FeatureCollection","features":[]}`,
		"anel vazio":       `{"type":"Polygon","coordinates":[[]]}`,
		"coord não número": `{"type":"Polygon","coordinates":[[["a","b"],[1,1],[2,2],["a","b"]]]}`,
		"plano demais":     `{"type":"Polygon","coordinates":[[-56,-13],[-55,-13]]}`,
	}
	for nome, texto := range casos {
		if _, err := Ler(texto); err == nil {
			t.Errorf("%s: deveria ser recusado", nome)
		}
	}
}

func TestLerLimitaTamanhoEQuantidade(t *testing.T) {
	if _, err := Ler(`{"type":"Feature","properties":{"nome":"` + strings.Repeat("x", TamanhoMaximo) + `"},"geometry":` + quadrado + `}`); err == nil {
		t.Error("texto maior que o limite deveria ser recusado")
	}
	feat := `{"type":"Feature","properties":{"nome":"x"},"geometry":` + quadrado + `}`
	var b strings.Builder
	b.WriteString(`{"type":"FeatureCollection","features":[`)
	for i := 0; i <= MaximoDeFeicoes; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(feat)
	}
	b.WriteString("]}")
	if _, err := Ler(b.String()); err == nil {
		t.Error("feições demais deveriam ser recusadas")
	}
}
