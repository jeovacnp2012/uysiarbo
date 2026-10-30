titulo: Histórico de mobilidade
resumo: Os envios feitos pelos aplicativos dos tablets: quando chegaram, de qual aparelho e o resultado de cada lote.
ordem: 16
permissao: historico_mobilidade
palavras: mobilidade, tablet, aplicativo, sincronização, sincronismo, envio, lote, conflito, rejeitada, aparelho, histórico
---
# Histórico de mobilidade

Cada vez que um aplicativo (tablet) envia visitas ao sistema, o envio fica registrado. A tela **Histórico de mobilidade** (menu Administração) mostra esses envios, do mais recente para o mais antigo.

## O que aparece
- **Recebido em**: quando o sistema recebeu o lote, no horário da sua cidade. Abaixo, o **relógio do aparelho** na hora do envio, quando ele informou.
- **Aparelho** e **Credencial**: de que tablet veio e com qual credencial da API ele se identificou.
- **Visitas**: quantas visitas vieram no lote. Delas, quantas foram **criadas**, quantas **já existiam** (reenvio, sem duplicar), quantas deram **conflito** e quantas foram **rejeitadas**; e quantos **tubitos** (amostras) vieram junto.

## Conflito e rejeição
- **Conflito**: o sistema já tinha aquela visita com dados diferentes dos que o aparelho mandou. O que está no sistema prevalece e nada é alterado.
- **Rejeitada**: a visita não passou nas regras (ciclo fechado, data fora do período, depósitos inconsistentes etc.). O aparelho recebe o motivo de cada uma na resposta do envio.

## Filtros
Os filtros pesquisam na hora: **aparelho**, **credencial**, **período** (pelo dia, no horário da cidade) e **só com conflito ou rejeição**, para achar os envios que pedem atenção.

## Histórico de cada visita
Na ficha de uma visita, quem tem acesso à **Auditoria** vê a seção **Histórico de alterações**: quem registrou, editou, desativou ou excluiu a visita e os tubitos dela, e quando. Visitas que vieram do aplicativo aparecem com a credencial como autora.
