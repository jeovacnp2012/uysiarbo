titulo: Lixeira e exclusão
resumo: Como funciona excluir, restaurar e excluir definitivamente.
ordem: 12
permissao: conf_lixeira
palavras: excluir, exclusão, lixeira, restaurar, apagar, definitiva, force delete, vínculo, deleted_at
---
# Lixeira e exclusão

No UYSI ARBO **nada é apagado de verdade** por engano: excluir um registro o manda para a **Lixeira**.

## Excluir
Nas fichas de **cidade, bairro, quarteirão, local** e **grupo de locais** há o botão **Excluir** (para quem pode editar). O registro some das listas, do mapa e da API, mas continua guardado, com a data e quem excluiu.

Se o registro ainda tem **dados ativos ligados a ele**, a exclusão é recusada com um aviso do que está ligado:
- uma **cidade** só pode ser excluída sem bairros ativos;
- um **bairro**, sem quarteirões nem locais ativos;
- um **quarteirão**, sem locais ativos.

Exclua ou mova primeiro o que está dentro. O nome de um registro excluído **pode ser usado de novo**.

## Restaurar
Em **Administração → Lixeira** (permissão **Lixeira**), escolha o tipo, e use **Restaurar**. Se o registro pai também estiver na lixeira (um local cujo bairro foi excluído, por exemplo), restaure o pai primeiro; se já existir outro registro ativo com o mesmo nome, o sistema avisa.

## Excluir definitivamente (force delete)
Só quem tem a permissão **Excluir definitivamente** vê o botão, e ele só vale para o que **já está na lixeira**. Apaga de verdade e não tem volta, por isso:
- é **recusada, com aviso**, se houver **vínculo com qualquer outro registro**, inclusive registros que também foram excluídos (por exemplo, um bairro que teve quarteirões ou locais, mesmo excluídos; um local que está em algum grupo de locais ou tem acesso concedido a uma pessoa);
- fica registrada na **Auditoria**.

Para chegar a apagar um bairro de vez, apague antes, de baixo para cima, o que está ligado a ele (locais → quarteirões → bairro).

## Importação e API
Desfazer uma importação também usa a lixeira. A API não exclui nada; ela marca `ativo: false` ou mostra `excluido_em` para quem sincroniza (veja o artigo da API).
