titulo: Relatórios
resumo: Gere relatórios por filtros, veja na tela e baixe em PDF, planilha ou CSV.
ordem: 19
permissao: relatorio_visita
palavras: relatório, relatorios, pdf, planilha, xlsx, csv, visitas, exportar, imprimir
---
# Relatórios

Abra **Relatórios** no menu e escolha um relatório. Cada um mostra só os relatórios que o seu papel libera.

1. Ajuste os filtros (ciclo, atividade, agente, bairro, período, pendência...). Vazio significa "todos".
2. **Gerar relatório** mostra o resultado na tela (as primeiras 200 linhas, com os totais).
3. **Baixar PDF**, **Baixar planilha** ou **Baixar CSV** trazem todas as linhas, com os filtros usados no cabeçalho, o cliente e a data e hora de geração (no fuso da sua cidade).

Cada relatório sai com até 20.000 linhas; se passar disso, o arquivo avisa que foi cortado e é preciso estreitar os filtros. Cada download fica registrado na Auditoria. Quem só vê as próprias visitas recebe só as dela.

## Relatório de Visitas
Lista as visitas com data, endereço, agente, ciclo, semana, atividade, pendência e se teve depósito, amostra e tratamento.

## Os demais relatórios
- **Local x visita:** os locais ativos com a quantidade de visitas no ciclo, a última visita e quantas tiveram depósito; filtre por "Foi visitado" para achar os locais que ficaram sem visita.
- **Acompanhamento de produção:** por agente, os dias trabalhados, visitas, locais, pendências (fechadas, recusadas, tentativas), depósitos, amostras e tratamentos.
- **Serviço antivetorial:** por atividade, os depósitos encontrados e eliminados, o larvicida (L1 e L2, em gramas e depósitos tratados) e as cargas de adulticida.
- **Dia trabalhado:** por agente e dia, as visitas, os locais e o horário da primeira e da última visita.
- **Laboratório: totais:** por bairro, os tubitos, em análise, concluídos, positivos e os totais de aegypti, albopictus e negativo.
- **Animais: totais:** por bairro, os moradores, cães, gatos e roedores cadastrados nos locais.
- **BRI:** as visitas com borrifação residual e o volume usado.
- **Locais com cadastro incompleto:** locais sem coordenada, sem CEP e, nos residenciais, sem proprietário ou sem número de moradores.

- **Campanha de vacinação:** por bairro, os atendimentos da campanha antirrábica, cães e gatos vacinados e não vacinados, recusas e casas fechadas; filtre por período e bairro.
- **LIRAa:** por bairro, os quarteirões programados e encerrados na agenda, os imóveis pesquisados, pendências, depósitos, positivos, recipientes positivos, IIP e Breteau (da atividade LIRAa no ciclo escolhido).
- **Locais por criticidade:** locais com depósito encontrado em 2 ou mais visitas nos últimos 12 meses, com os de tubitos positivos primeiro; serve de apoio ao sobrevoo e ao bloqueio.
- **Pesquisa entomológica / tratamento em papel:** folha para imprimir, com os locais por quarteirão e colunas em branco (depósitos A1 a E, tubito, larvicida) para preencher a mão quando não há tablet; depois os dados são lançados em Visitas.

As colunas e fórmulas (IIP, Breteau, criticidade) foram inferidas do Geo Controle e devem ser conferidas com a área técnica.
