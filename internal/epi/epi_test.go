package epi

import (
	"testing"
	"time"
)

func dia(a, m, d int) time.Time { return time.Date(a, time.Month(m), d, 15, 30, 0, 0, time.UTC) }

func TestSINAN(t *testing.T) {
	casos := []struct {
		d           time.Time
		ano, numero int
		inicio      string
	}{
		{dia(2026, 1, 1), 2025, 53, "2025-12-28"}, // 1/1/2026 é quinta: a semana ainda é de 2025
		{dia(2026, 1, 4), 2026, 1, "2026-01-04"},
		{dia(2026, 1, 10), 2026, 1, "2026-01-04"},
		{dia(2026, 1, 11), 2026, 2, "2026-01-11"},
		{dia(2026, 10, 29), 2026, 43, "2026-10-25"},
		{dia(2026, 12, 31), 2026, 52, "2026-12-27"},
		{dia(2025, 1, 1), 2025, 1, "2024-12-29"}, // quarta: tem 4 dias em janeiro
		{dia(2024, 12, 29), 2025, 1, "2024-12-29"},
		{dia(2023, 1, 1), 2023, 1, "2023-01-01"}, // domingo
		{dia(2021, 1, 3), 2021, 1, "2021-01-03"},
		{dia(2021, 1, 2), 2020, 53, "2020-12-27"},
	}
	for _, c := range casos {
		s := Calcular(c.d, SINAN)
		if s.Ano != c.ano || s.Numero != c.numero || s.Inicio.Format("2006-01-02") != c.inicio {
			t.Errorf("%s: %d/%d início %s, queria %d/%d início %s", c.d.Format("2006-01-02"), s.Ano, s.Numero, s.Inicio.Format("2006-01-02"), c.ano, c.numero, c.inicio)
		}
		if s.Inicio.Weekday() != time.Sunday || s.Fim.Sub(s.Inicio).Hours() != 144 {
			t.Errorf("%s: a semana deveria ir de domingo a sábado: %v a %v", c.d.Format("2006-01-02"), s.Inicio, s.Fim)
		}
	}
}

func TestISO(t *testing.T) {
	casos := []struct {
		d           time.Time
		ano, numero int
		inicio      string
	}{
		{dia(2026, 1, 1), 2026, 1, "2025-12-29"},
		{dia(2026, 10, 29), 2026, 44, "2026-10-26"},
		{dia(2021, 1, 3), 2020, 53, "2020-12-28"},
		{dia(2024, 12, 30), 2025, 1, "2024-12-30"},
		{dia(2026, 12, 31), 2026, 53, "2026-12-28"},
	}
	for _, c := range casos {
		s := Calcular(c.d, ISO)
		if s.Ano != c.ano || s.Numero != c.numero || s.Inicio.Format("2006-01-02") != c.inicio {
			t.Errorf("%s: %d/%d início %s, queria %d/%d início %s", c.d.Format("2006-01-02"), s.Ano, s.Numero, s.Inicio.Format("2006-01-02"), c.ano, c.numero, c.inicio)
		}
		if s.Inicio.Weekday() != time.Monday {
			t.Errorf("a semana ISO começa na segunda: %v", s.Inicio)
		}
	}
}

func TestValido(t *testing.T) {
	if !Valido("iso") || !Valido("sinan") || Valido("") || Valido("outro") {
		t.Error("Valido")
	}
	// Tipo desconhecido vale SINAN.
	if Calcular(dia(2026, 1, 4), "x").Numero != 1 || Calcular(dia(2026, 1, 4), "x").Inicio.Weekday() != time.Sunday {
		t.Error("o padrão deveria ser SINAN")
	}
}
