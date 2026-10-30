titulo: Comunicações — WhatsApp e e-mail
resumo: A fila de mensagens, o que significa cada situação, como reenviar e acompanhar a linha do tempo.
ordem: 22
permissao: comunicacao_visualizar
palavras: comunicação, mensagem, whatsapp, e-mail, email, fila, reenviar, tentativa, entregue, visualizada, notificação
---
# Comunicações — WhatsApp e e-mail

O menu **Comunicações** guarda todas as mensagens que o sistema envia ou vai enviar, por WhatsApp ou e-mail, com o histórico de cada uma.

## Situações
- **Na fila:** aguardando o envio.
- **Enviada:** entregue ao provedor de envio.
- **Entregue** e **Visualizada:** a mensagem chegou e foi lida.
- **Falhou:** todas as tentativas deram erro (são 3, com espera crescente entre elas).
- **Cancelada:** descartada por alguém.

## Nova comunicação
Quem tem a permissão **Reenviar** pode criar uma mensagem em **Nova comunicação**: escolha o canal, informe o telefone com DDD (ou o e-mail), o texto e, no e-mail, o assunto. O telefone é guardado só com números e com o código 55 do Brasil.

## Enviar, reenviar e cancelar
- **Enviar agora:** tenta o envio de uma mensagem na fila.
- **Processar fila:** na lista, tenta enviar todas as que já estão no horário (até 50 por vez).
- **Reenviar:** uma mensagem que falhou ou foi cancelada volta para a fila e é enviada de novo.
- **Cancelar:** vale para mensagem na fila ou que falhou.
- **Marcar como entregue/visualizada:** registra o acompanhamento de uma mensagem já enviada.

## Envio automático
O **e-mail** sai pelo servidor SMTP configurado no servidor do sistema (variáveis `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM` e `SMTP_SECURITY`). Com o SMTP ligado, o sistema olha a fila a cada minuto, em todos os clientes, e envia sozinho o que já está no horário; mensagem com erro tenta de novo até 3 vezes.

O **WhatsApp** ainda não tem provedor: as mensagens desse canal esperam na fila, sem gastar tentativas, e o sistema avisa. Sem SMTP configurado, o mesmo vale para os e-mails.

## Linha do tempo
Cada mensagem mostra tudo o que aconteceu com ela: criação, tentativas, falhas, envio, entrega, leitura e quem fez cada ação. Mensagens não são apagadas; para parar um envio, cancele.
