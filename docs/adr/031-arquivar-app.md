# 031 - Arquivar app

**Status:** Accepted

## Context
Até aqui não havia como tirar um app do Ziv. Um app que ninguém usa mais
continuava no catálogo, na tela inicial, na busca, no Claude e, se tinha versão
no servidor, ocupando uma das vagas de app com tela da empresa (ADR 025).
Apagar de uma vez é arriscado: some o histórico, as execuções, o cofre e os
dados do app com tela, sem volta. O Henry aprovou (2026-11-09) um arquivamento
com prazo: o app sai de tudo, pode voltar por 30 dias e depois é apagado de vez.

## Decision
- **Quem arquiva:** o botão "Arquivar", no pé da página do app, aparece só para
  quem controla o acesso do app (`can.access`): o responsável, a reserva, o
  líder da área ou quem gerencia as áreas. O mesmo vale para trazer de volta.
  Ele pede confirmação numa janela que explica o que acontece.
- **O que o arquivamento faz** (`POST /api/apps/{id}/archive`):
  - grava `archived_at` e `archived_by` no app (migração 024);
  - o app some do catálogo, da tela inicial (favoritos, recentes, novidades da
    equipe, avisos), da busca, dos "Outros apps" e do Claude, que lê as mesmas
    listas;
  - a ficha e todas as rotas do app respondem 410 com "Este app foi arquivado.
    Quem cuida dele pode trazê-lo de volta em Apps › Arquivados.";
  - não roda mais: `executionBlock` recusa qualquer execução, inclusive as que
    já estavam na fila do Agente, as agendadas e as pedidas pelo Claude;
  - os agendamentos ficam pausados com o motivo "O app foi arquivado" e somem
    de "Meus agendamentos";
  - a versão no servidor é parada como no botão "Parar" (caixa desligada;
    imagem, banco e cópias guardados) e o endereço do app passa a responder
    "não encontrado". O app arquivado não conta nas vagas de app com tela;
  - fica uma linha no histórico da ficha ("Arquivou o app"); a chamada também
    entra no registro de auditoria, como toda chamada que muda algo.
- **Arquivados:** em Apps, um link "Arquivados (N)" aparece quando há apps
  arquivados que a pessoa poderia trazer de volta. A lista mostra quem arquivou,
  quando, e quantos dias faltam para a exclusão definitiva.
- **Trazer de volta** (`POST /api/apps/{id}/unarchive`): o app volta ao
  catálogo como estava, mas os agendamentos e a versão no servidor continuam
  desligados. A tela diz "ligue de novo se precisar".
- **Exclusão definitiva:** a cada hora, cada empresa apaga os apps arquivados há
  mais de 30 dias. Antes, limpa a versão no servidor pelo mesmo caminho de
  "Deixar de compartilhar" e da faxina dele (caixas, imagens, rede, banco,
  cópias e dados), tira o app dos favoritos e grava uma linha de auditoria. O
  resto (execuções, versões, cofre, acessos, agendamentos) sai junto com o app.
  **A pasta do app no computador nunca é mexida.**
- **Registrar de novo:** a análise de pastas mostra que a pasta é de um app
  arquivado e aponta para Arquivados; publicar essa pasta é recusado (409
  `app_archived`). Depois da exclusão definitiva, a pasta pode ser registrada
  como um app novo.

## Rationale
Arquivar com prazo dá o alívio de "sumir com o app" sem o risco de perder algo
por engano. Parar a versão no servidor, em vez de deixar de compartilhar, deixa
a volta completa (banco e cópias intactos). Recusar o registro da mesma pasta
evita que o app arquivado volte sem ninguém perceber.

## Trade-offs
**Pros:**
- Um clique tira o app de todo lugar; outro, dentro de 30 dias, devolve.
- A limpeza do servidor reaproveita o código que já apaga apps deixados de
  compartilhar.

**Cons:**
- Durante os 30 dias, o banco e as cópias do app com tela continuam ocupando
  disco.
- Quem só usa o app não vê os arquivados; se precisar dele de volta, pede ao
  responsável.
- As execuções antigas do app continuam no histórico de quem rodou até a
  exclusão definitiva.
- Sem o servidor de apps ligado (sem Podman ou sem `HUB_SECRET_KEY`), um app
  arquivado que tem versão no servidor não é apagado: a faxina tenta de novo a
  cada hora até conseguir limpar o servidor.
- As cópias de código que o próprio Ziv guarda no servidor (apps enviados pelo
  navegador ou pelo Claude) ficam no disco depois da exclusão, como as pastas
  dos outros apps.

## Alternatives Considered
- **Apagar na hora, com confirmação:** simples, mas sem volta para um engano.
- **Deixar de compartilhar ao arquivar:** libera o servidor na hora, mas a
  volta exigiria publicar de novo.
- **Arquivar sem prazo:** os arquivados se acumulariam para sempre, com o banco
  e as cópias no disco.
