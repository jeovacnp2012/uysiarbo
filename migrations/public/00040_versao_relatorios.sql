-- +goose Up
-- Versão 1.0.037: motor de relatórios e Rastro GPS com posição do local.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.037', 'Relatórios em tela, PDF e planilha', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova área Relatórios no menu: escolha o relatório, ajuste os filtros e veja na tela ou baixe em PDF, planilha (XLSX) ou CSV. O primeiro relatório é o de Visitas.', 1),
    ('melhorado', 'Rastro GPS ganhou a opção de mostrar a posição do local quando a visita não tem GPS, útil para as visitas importadas.', 2),
    ('corrigido', 'Os filtros do Rastro GPS não se sobrepõem mais quando o nome do agente é comprido.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.037';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.037';
