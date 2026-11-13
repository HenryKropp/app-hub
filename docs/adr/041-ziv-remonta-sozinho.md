# ADR 041: O Ziv remonta sozinho quando a receita muda

**Data:** 2026-11-13
**Status:** Aceito

## Contexto

O ADR 040 ensinou a receita automática a trazer o navegador. O app de relatórios (BTG/BBA) já estava publicado com a receita antiga, sem navegador e com 256 MB. Para receber a receita nova, ele precisava ser montado de novo. Mas a página do app esconde o Publicar quando a versão no ar já é a mais nova, porque o código não mudou. O Henry: "você tem que atualizar o app sozinho". Uma melhoria do Ziv não deve depender de alguém clicar.

## Decisão

- Cada publicação já guarda a receita com que foi montada (`shared_deployments.recipe`). O supervisor compara essa receita com a que o Ziv gera hoje para o mesmo app.
- Se forem diferentes e o código no ar ainda for o último (a versão publicada é a mais nova), o Ziv publica de novo sozinho, do jeito de sempre. A caixa antiga fica no ar até a nova responder. Se a nova falhar, nada muda para quem usa.
- Não remonta: app com Dockerfile próprio; app com código novo esperando publicação (essa decisão é da pessoa); pasta que o Ziv não consegue ler; app parado, dormindo ou com erro. Um app dormindo é conferido quando volta a rodar.
- Uma remontagem por vez no servidor inteiro, entre todas as empresas (`Slots.TryRefresh`), e no máximo uma por rodada do supervisor (30 s).
- Cada versão no ar é conferida uma vez por início do Ziv. Uma remontagem que falha não é repetida até o próximo início, para não ficar montando sem parar.
- A publicação fica sem autor (`created_by` vazio). O histórico do app mostra "O Ziv remontou a versão N com a receita nova do servidor".

## Por quê

- A receita guardada já existia, e nenhum controle novo de versões foi preciso.
- Conferir só quando o código é o mesmo garante que a pessoa vê o mesmo app, só montado de um jeito melhor.
- Rodar depois de cada publicação do Ziv (o processo reinicia) cobre o caso real: a receita só muda quando o Ziv muda.

## Consequências

- Uma mudança no comando ou na porta salva e ainda não publicada também muda a receita. Nesse caso, a remontagem aplica a mudança. Isso é aceitável: a pessoa já tinha salvo.
- Depois de uma publicação do Ziv que muda a receita de muitos apps, eles são remontados um por um. No servidor gratuito, cada montagem pode levar minutos.
