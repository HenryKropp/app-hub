# 034 - MCP: publicar app com tela sem ficar no escuro (F0 + F1)

**Status:** Accepted

## Context
Em 2026-11-11, uma mudança pronta do app "baixar-relatorios" levou 6 versões e uns 20 passos para entrar no ar pelo Claude. A suspeita era o servidor cair por falta de memória (F0).

A investigação (só leitura no servidor, com o OK do Henry) mostrou outra coisa:
- O Ziv ficou ligado sem parar desde 2026-11-09 22h13. Nada foi morto por falta de memória e sobravam ~490 MB.
- Nos 4 horários de "ECONNREFUSED" que o Claude viu (13h06, 13h44, 14h03 e 14h32 UTC), nenhum pedido chegou ao servidor. A falha está no caminho entre o PC e a internet.
- As 4 primeiras publicações quebraram na hora, com um erro claro no código (`Path(__file__).parents[4]`). O Ziv guardou o log, mas não mostrou ao Claude. Ainda esperou 90 s por um app que já tinha morrido e respondeu "não respondeu na porta, confira 0.0.0.0". Cada tentativa levou de 2 a 5 minutos, e o Claude reenviou a mesma versão 7 quatro vezes.

## Decision
- **Morte rápida:** enquanto espera o app responder, o Ziv também olha se o programa ainda está vivo. Se ele parou, a falha sai em segundos: "O programa parou com erro (código N) X s depois de ligar…", ou "terminou sozinho…" quando o código é 0. A dica do 0.0.0.0 só aparece quando o programa está vivo e não abriu a porta.
- **Log na falha:** a publicação guarda as últimas 60 linhas. Os valores secretos do próprio app (Configurações e cofre, incluindo a senha do banco) viram `••••` antes de guardar. A tela de logs do site usa a mesma limpeza.
- **Etapas:** enquanto publica, a implantação diz a etapa (cópia de segurança, montando, banco, ligando, esperando responder, trocando) e há quanto tempo. Isso fica só na memória, e a API mostra `stage`, `stage_seconds` e `seconds`.
- **MCP:**
  - `publicar_app_com_tela` devolve, ao falhar: o motivo, se a versão anterior está de fato no ar (com o número dela), as últimas 50 linhas do log e a regra "leia o log e corrija antes de mandar outra versão". Enquanto publica, devolve a etapa e o tempo.
  - `ver_app` ganha `no_ar`, `situacao_no_ar`, `versao_no_ar` e `ultima_publicacao`.
  - Ferramenta nova `ver_log_app`, só de leitura, para quem vê a parte técnica. Ela mostra o fim do log da versão ligada e, se a última publicação falhou, o log dela. A rota `GET /api/apps/{id}/shared/logs` entrou na lista de rotas liberadas para o Claude.

## Rationale
O custo real eram as tentativas no escuro. Com o motivo e o log na própria resposta, a correção sai na primeira volta. A rede do PC não se resolve no Ziv.

## Trade-offs
**Pros:**
- Uma falha na partida leva segundos, não 90 s.
- O Claude vê a mesma coisa que quem cuida vê na parte técnica.

**Cons:**
- O log pode mostrar segredos que não estão nas Configurações. A limpeza por padrão (`password=`, `token:`, URL com senha) cobre parte; o resto depende do app.
- A etapa se perde se o Ziv reiniciar no meio. Nesse caso a implantação já é tratada como sobra.

## Alternatives Considered
- **Separar o app com tela do Ziv em outra VM (F0):** descartado. A memória não era o problema.
- **Guardar a etapa no banco:** descartado. Exige uma migração para um dado que só vale enquanto publica.
