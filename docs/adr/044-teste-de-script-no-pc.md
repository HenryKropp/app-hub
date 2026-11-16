# 044 - Teste de script no PC antes de pôr no ar

**Status:** Accepted

## Context
A Etapa 2 do projeto "Testar e publicar sem o Henry" (desenho de 2026-11-16)
pede que uma versão nova de um script só vá ao ar depois de rodar de verdade,
em modo teste, no PC de alguém da empresa. Quem manda é o Claude (pelo MCP),
ninguém está olhando, e o Henry não quer mais ser o testador. O rascunho de
versão da ADR 043 já guarda a versão sem mudar o app no ar; faltava rodar,
conferir e publicar sozinho.

## Decision
- **O teste é um trabalho `test` do Agente, separado de `runs`.** A tabela
  `agent_jobs` ganha `draft_id`, e o resultado vai para o próprio rascunho
  (`test_log`, `test_checks`, `test_files`, `test_exit_code`) e para
  `draft_evidence`. Nada do teste aparece em Minhas execuções, nas estatísticas
  do app nem na pasta de saída oficial. O Agente 0.4.0 roda o script numa
  pasta só do teste (`testes/app-N/rascunho-M`), com `ZIV_TESTE=1` e
  `ZIV_PASTA_TESTE`, e mascara os valores do Cofre no log antes de mandar.
  O servidor mascara de novo.
- **O estado fica no banco e é movido por um relógio.** O rascunho tem uma
  etapa (`esperando_confirmacao`, `esperando_pc`, `rodando`, `publicando`).
  `testPipelineTick` (a cada 10 s) e o resultado do Agente andam um passo de
  cada vez, sempre a partir da etapa lida (`SetDraftStage` com a etapa de
  origem). Assim, os dois nunca dão o mesmo passo. Um trabalho que o PC não
  pegou volta para `esperando_pc`. Um PC que não aparece em 24 h, ou um teste
  que passa do tempo, reprova o rascunho com o motivo.
- **Publicar sozinho usa o mesmo `publishDraft` do site**, chamado por
  `publishDraftBG` com um `httptest.ResponseRecorder`. Não foi preciso
  reescrever `applySources` nem duplicar a trava, a recusa de rascunho
  fechado e a marca "testada". É um padrão estranho, mas o caminho de
  publicação continua único.
- **O plano de teste é confirmado só pelo site,** uma vez por app, pelo dono
  ou por um administrador. A rota não entra em `mcpRoutes` e responde 403
  quando chamada pelo Claude. O Ziv guarda uma impressão do plano (o que
  acessa, o que pula, as chaves e o lugar). Se ela muda, pede a confirmação
  de novo.
- **O PC de teste é sempre um computador de quem confirma.** O Ziv não sugere
  o PC do agendamento, que pode ser de outra pessoa: rodar código ainda não
  publicado no PC de alguém sem ele saber pesa mais.
- **`enviar_versao` funciona em duas chamadas.** A primeira devolve o comando
  de envio. A segunda, com `envio`, cria o rascunho e começa o teste. As
  recusas de senha e de ficha voltam a quem enviou do mesmo jeito de hoje.
  Pelo MCP, script só vai por `enviar_versao`: `registrar_app` recusa
  script. Se falta uma chave, a versão fica guardada, e depois
  `enviar_versao` com `app_id` e `versao` começa o teste sem novo envio.
- **App novo nasce restrito e "em teste".** Só o dono e os admins o veem, e
  ele não roda até o primeiro teste passar.
- **O teste respeita a aprovação de segurança do app no ar** (`executionBlock`):
  numa empresa que exige aprovação, o teste só roda se a versão no ar está
  aprovada. O código novo do rascunho **não** passa por aprovação antes do
  teste: ele roda no PC de quem confirmou o plano, com as chaves que o plano
  declara. A versão que vai ao ar depois precisa de aprovação, como hoje, e o
  aviso diz isso. (Pendente de decisão do Henry: exigir aprovação do código
  do rascunho antes do teste nas empresas que pedem aprovação.)
- **O teste recebe só as chaves que o plano declara** (as que aparecem em
  "Chaves usadas" na confirmação), não o Cofre inteiro. Valores do Cofre são
  mascarados no log, nos nomes de arquivos, nas conferências e no conteúdo
  que `baixar_arquivo_teste` traz para a conversa.

## Rationale
O Agente já roda scripts no PC com o Cofre. Reaproveitar o trabalho do Agente
com um tipo novo mantém o isolamento (pasta própria, máscara, sem execução na
lista) sem um segundo mecanismo. O relógio no banco sobrevive a reinício do
Hub e a PC desligado, que é o caso normal. A confirmação no site garante que
uma pessoa, e não o Claude, aceita o que o teste vai tocar.

## Trade-offs
**Pros:**
- "Testada" passa a significar: rodou de verdade, com as chaves, e as
  conferências passaram.
- Uma versão quebrada nunca chega ao ar. A anterior continua igual.
- O dono recebe um aviso na Home quando a versão vai ao ar.

**Cons:**
- O teste depende de um PC ligado com o Agente 0.4.0 de quem confirmou.
- O padrão do recorder HTTP exige cuidado ao mudar `publishDraft`.
- Um app novo cujo teste nunca passa fica parado e escondido até ser
  arquivado.

## Alternatives Considered
- **Teste como uma execução comum (`runs`):** misturaria testes com o uso real
  e mandaria arquivos de teste para a pasta de saída oficial.
- **Três ferramentas encadeadas pelo Claude (enviar, testar, publicar):**
  quem confere se passou seria o próprio construtor. Um teste pulado
  publicaria uma versão ruim.
- **Confirmar o plano pela conversa:** o Claude poderia confirmar sozinho.
  Isso tira a pessoa do único ponto em que ela decide o que o teste toca.
- **Usar o PC do agendamento:** pode ser de outra pessoa (decisão 2 acima).
