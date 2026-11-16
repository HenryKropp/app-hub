# Testar e publicar sem o Henry — desenho

**Data:** 2026-11-16 · **Status:** aprovado pelo Henry em 2026-11-16 · **Origem:** TASKS "Lições do app de relatórios" (itens 1 a 4) + "Teste extenso antes de publicar"

## Por que

No app de relatórios, o Henry foi a máquina de teste de quatro versões que falharam (v11 a v14). Cada uma falhou num passo que o cofundador não tinha testado. Para testar no PC, o cofundador ainda precisou ler as senhas do Cofre de dentro do servidor. Hoje "testar" e "publicar" são a mesma coisa: uma versão nova substitui a anterior na hora, e até a rodada agendada das 9h já a usa.

O que o Henry quer: quem constrói um app (o cofundador, outro Claude ou uma pessoa usando o MCP) testa com trabalho de verdade e publica, sem o Henry no meio. O Henry só fica sabendo que saiu uma versão testada, ou só entra quando algo exige decisão dele.

## Metas

1. **Rascunho.** Uma versão nova chega ao Ziv como rascunho e não muda o que está no ar. Rodadas manuais e agendadas continuam usando a versão no ar.
2. **Um passo.** Uma ferramenta do MCP (`enviar_versao`) dispara a sequência inteira. O Ziv conduz: guarda o rascunho, testa, confere, publica, troca o app no PC e avisa.
3. **Teste de verdade, onde o app roda de verdade.** App de PC testa no PC pelo Agente. App de servidor testa no servidor, no "Testar" que já existe (deployment `kind=test`).
4. **Senhas sem ninguém ver.** O teste usa as chaves do Cofre pelo mesmo caminho das rodadas normais: o Agente busca uma vez, guarda só em memória, e os logs saem mascarados. Ninguém (cofundador, MCP, conversa) vê os valores.
5. **Evidência.** O teste devolve o log, a lista de arquivos gerados (o conteúdo pode ser baixado sob pedido) e uma foto da tela, para apps com tela.
6. **O Ziv decide se passou**, conferindo o "resultado esperado" do plano de teste. Quem envia não decide.
7. **Passou → no ar sozinho**, e o Henry recebe um aviso, por exemplo: "Relatórios versão 17 no ar, testada: 12 arquivos salvos". **Falhou →** nada muda para o usuário. A versão anterior continua e quem enviou recebe a evidência para corrigir.
8. **Troca no PC sem o Henry clicar em Parei.** É o pedido "Ziv fecha e reabre o app no PC sozinho". Detalhes na seção "Troca do app de tela no PC".

## Não metas (ficam para depois, já estão no TASKS)

- Sininho de notificações. Por enquanto o aviso usa o que existe (ver "Avisos").
- Ver fácil em qual versão o app está, e ver o que está rodando agora.
- Histórico de código com "o que mudou" e voltar para a versão anterior.
- `ver_log_app` para rodadas normais no PC. O log do **teste** já vem pelo `ver_versao`.
- Servidor mais rápido.

## Anti-metas (falha mesmo que "funcione")

- Uma versão entra no ar sem ter passado no teste, ou sem plano de teste confirmado.
- O teste faz algo irreversível que o plano não anunciou: e-mail enviado, arquivo na pasta oficial, aprovação.
- Uma senha do Cofre aparece para o cofundador, no MCP, num log ou num arquivo de evidência.
- O Henry perde o que estava digitando porque o Ziv trocou o app no meio do uso.
- O Henry volta a ser chamado para testar.

## Decisões do Henry (2026-11-16)

| # | Decisão |
|---|---------|
| 1 | Teste passou → **publica sozinho** e avisa. |
| 2 | O teste roda em **modo teste**: o Ziv avisa o app (`ZIV_TESTE=1`) e dá uma pasta de saída separada. O app faz tudo de verdade (login, baixar, montar) e pula só o passo final irreversível. **Antes, o plano de teste é explicado e confirmado.** |
| 3 | A confirmação é **uma vez por app**. A mensagem de confirmação diz claramente: *"Os próximos testes deste app serão automáticos. Só pergunto de novo se o plano de teste mudar."* |
| 4 | App de tela no PC com versão nova: **se ninguém usou o app nos últimos 10 min, o Ziv fecha e reabre sozinho. Se está em uso, aparece um aviso na tela do app ("Versão nova pronta. Atualizar agora?")**. |
| 5 | Caminho **A**: o Ziv conduz a sequência inteira; não são ferramentas separadas que quem envia encadeia. |
| 6 | **Vale também pelo site.** App com plano de teste: depois de subir uma versão pelo site ("Enviar versão nova"), aparece o botão **"Testar e publicar"** e o Ziv testa antes de pôr no ar. App sem plano: ao subir, o site avisa **"Esta versão vai ao ar sem teste"** e ela vai ao ar como hoje, marcada "não testada". Pelo MCP o plano é sempre obrigatório. |
| 7 | **Arquivos de evidência: só o dono do app e os admins da empresa** podem ver (pelo site ou pelo MCP com a conta deles). São apagados depois de 7 dias. |

## Caminhos considerados

- **A. O Ziv conduz tudo (escolhido).** Uma ferramenta dispara um processo no servidor que passa por etapas (rascunho → teste → conferência → publicação → troca → aviso), e quem enviou acompanha por `ver_versao`.
  - A favor: o Ziv é quem decide se passou, então nenhuma versão publica sem teste, venha de qualquer Claude ou pessoa. É "publicar em um passo" de verdade.
  - Contra: mais trabalho no servidor, porque as etapas precisam sobreviver a reinícios.
- **B. Três ferramentas (enviar rascunho, testar, publicar) encadeadas por quem envia.** É mais simples, mas quem confere se passou é o próprio construtor. Um erro ou um teste pulado publica uma versão ruim, e isso enfraquece a promessa "testada antes de publicar".
- **Descartado: testar numa cópia do app (empresa Teste).** As chaves do Cofre são de cada app, então a cópia não as teria e voltaríamos a precisar ver as senhas.

## O desenho

### 1. Versão rascunho

Hoje `sendSources` (`backend/internal/handler/stored_code.go:287-298`) troca `script_sources` e a pasta do app na hora, e `launchRun` (`handler/launch.go:139-161`) usa sempre o código mais recente. A mudança:

- Uma versão (`app_versions`) ganha **situação**: `rascunho` → `testando` → `aprovada`/`reprovada` → `no_ar` → `substituida`. Só uma versão por app fica `no_ar`.
- **Rodadas normais** (manuais e agendadas) rodam o código da versão `no_ar`. **A rodada de teste** roda o código do rascunho.
- Enviar uma versão com uma rodada normal em andamento **não é mais recusado**, porque o rascunho não mexe na versão no ar. (Hoje a recusa vem de `stored_code.go:255-260`.)
- Um rascunho novo enviado enquanto outro do mesmo app ainda não foi publicado **cancela o anterior**. Ele fica `reprovada` com o motivo "substituída por versão mais nova". *(Call do cofundador.)*
- Apps já existentes: a versão atual de cada app vira `no_ar` na migração.
- **A receita pertence à versão, não ao app.** Hoje ela fica em `apps`. Ela passa a ser guardada em cada versão. Rodadas normais leem a receita da versão `no_ar`. O teste lê a do rascunho. A impressão do plano vem do rascunho. Assim o rascunho não muda entradas, chaves ou plano do app no ar. *(Call do cofundador.)*
- **Pelo site** (decisão 6), no "Enviar versão nova":
  - App cujo rascunho tem `teste`: a versão fica `rascunho` e a tela mostra **"Testar e publicar"**, que inicia a mesma sequência do `enviar_versao`.
  - App sem `teste`: a tela avisa **"Esta versão vai ao ar sem teste"**. Depois de confirmar, a versão vai direto a `no_ar`, marcada "não testada", e a página do app mostra essa marca.
  - **Voltar uma versão antiga** não testa de novo: ela já esteve no ar.
  - Versões por pasta (`origin='folder'`) seguem a mesma regra do site.
- **App novo** só aparece para os usuários depois do primeiro teste que passa. Até lá, aparece só para quem enviou e para os admins, como "em teste". *(Call do cofundador.)*
- Publicar uma versão de app de PC é só marcá-la `no_ar`. O código vai junto em cada rodada (`agentproto/proto.go:42-80`), então não há instalação. Para app de tela no servidor, publicar é o deploy normal (`/shared/publish`). O builder pode promover a imagem já construída no teste, se for simples.

### 2. Plano de teste (na receita)

A receita (`backend/internal/manifest/manifest.go`) ganha uma seção `teste`:

```yaml
teste:
  acessa: ["BTG (login com as chaves BTG_USUARIO, BTG_SENHA)", "Itaú"]  # o que o teste toca, em português, para a confirmação
  pula: "mover os relatórios para a pasta oficial do OneDrive"          # o passo irreversível que o modo teste pula; "nada" se não há
  entradas: { data: "hoje" }                                             # entradas da rodada de teste, se o app tem entradas
  espera:                                                                # o que o Ziv confere para dizer "passou"
    - arquivos: "*.pdf"
      no_minimo: 1
    - log_contem: "Concluído"
    - tela_contem: "Relatórios de hoje"   # só apps com tela
  acao: "POST /rodar"                    # só apps com tela: o que o Ziv chama para o app fazer o trabalho principal
  tempo_maximo_min: 15
```

- **App com tela precisa fazer trabalho no teste, não só abrir a página.** O teste do app com tela:
  1. abre o app;
  2. chama a `acao` (uma rota HTTP local do próprio app, a mesma que o botão principal ou a rodada agendada usa);
  3. espera ela terminar;
  4. confere `espera`;
  5. tira a foto.
  
  `acao` é obrigatória para app com tela. Sem ela, "passou" só provaria que a página abre, e esse é justamente o teste que o Henry recusou em 2026-11-13. `formato_do_app` passa a ensinar o app a expor essa rota e a responder quando terminar, de forma síncrona ou com uma rota de situação. *(Call do cofundador. Foi escolhido em vez de "Ziv clica na página" porque é mais estável, e em vez de "o app começa sozinho em modo teste" porque deixa claro o que é testado.)*
- **App que roda em dois lugares** (por exemplo, a tela no servidor e o BTG no PC pelo Agente): o teste roda as duas partes, cada uma no seu lugar, e **as duas precisam passar**. Publicar é o deploy no servidor mais a versão `no_ar` para a parte do PC, ao mesmo tempo. Se uma parte falha, nada vai ao ar. *(Call do cofundador.)*

- Sem a seção `teste`, `enviar_versao` recusa. A mensagem diz o próximo passo: "escreva a seção teste da receita; veja `receita_app_ziv`".
- `espera` precisa de pelo menos uma conferência que prove trabalho de verdade: `arquivos` ou `tela_contem`. Só `log_contem` não basta. Também passam a ser exigidos: código de saída 0 (scripts) e a página respondendo (apps com tela).
- `receita_app_ziv` e `formato_do_app` passam a ensinar a seção `teste` e o modo teste (`ZIV_TESTE`, `ZIV_PASTA_TESTE`).

### 3. Confirmação do plano (uma vez por app)

- O Ziv guarda uma "impressão" (hash) do plano: `acessa`, `pula`, as chaves usadas e o lugar onde roda (PC/servidor).
- Plano sem confirmação, ou impressão diferente da confirmada → a versão fica em **"esperando confirmação do plano de teste"**. O teste não roda.
- **A confirmação é feita por uma pessoa no site do Ziv, nunca pelo MCP.** Assim um Claude não confirma o próprio plano. O `enviar_versao` devolve o link da tela de confirmação, e o cofundador passa o link ao Henry. *(Call do cofundador, por segurança.)*
- A tela de confirmação mostra o plano em português, quem confirma (admin da empresa ou dono do app) e, em destaque: *"Os próximos testes deste app serão automáticos. Só pergunto de novo se o plano de teste mudar."*
- A confirmação é registrada: quem, quando e a impressão confirmada.
- Mudar `espera`, `entradas` ou `tempo_maximo_min` **não** pede confirmação nova, porque não muda o que o teste toca.

### 4. Rodada de teste

- **Onde:**
  - App de PC (script/exe/tela no PC) → num PC pelo Agente. **O PC de teste é escolhido na tela de confirmação do plano**, que já sugere o PC das rodadas agendadas ou o PC de quem confirma. Ele fica guardado com a confirmação e pode ser trocado na página do app. Trocar o PC não pede nova confirmação. *(Call do cofundador.)*
  - App de tela no servidor → o "Testar" do servidor (`deploy.Service.Test`, `kind=test`, banco de teste, pasta de dados de teste).
- **Modo teste:** a rodada recebe `ZIV_TESTE=1` e `ZIV_PASTA_TESTE=<pasta>`. No PC, a pasta fica dentro da pasta de estado do Agente (`...\testes\app-N\versao-M`), nunca na pasta de saída oficial. O app grava ali.
- **Chaves:** o mesmo caminho das rodadas normais (`Keys: true`, busca única, só em memória). O log do teste passa pela mesma máscara de segredos dos logs do servidor (`deploy/progress.go:131-154`), e o Agente também mascara antes de enviar.
- **Evidência que o Agente devolve ao servidor:**
  - o log completo (`.apphub-console.txt`), mascarado;
  - a lista de arquivos da pasta de teste (nome, tamanho);
  - os arquivos, enviados só até um limite (por exemplo 20 MB no total), que ficam 7 dias no servidor. **Só o dono do app e os admins da empresa** podem vê-los (decisão 7), pelo site ou pelo MCP. O conteúdo dos arquivos não é mascarado, porque são dados do trabalho. A máscara de segredos vale para o log;
  - a foto da tela.
- **Foto da tela:**
  - App de tela no PC: o Agente abre a página do app num navegador sem janela. O Edge, que todo Windows tem, faz `--headless --screenshot`.
  - App de tela no servidor: o servidor fotografa a URL de teste com um navegador sem janela num contêiner auxiliar.
  - Scripts: não há tela. A evidência são os arquivos. Se o script usa navegador, a receita recomenda que ele salve as próprias fotos na pasta de teste.
- **PC desligado ou Agente fora do ar:** a versão fica "esperando o PC". Isso aparece no `ver_versao` e na página do app. Depois de 24 h sem PC, a versão é reprovada por tempo.

### 5. Conferência e publicação

- O Ziv confere cada item de `espera` e o código de saída e grava o resultado item por item. Por exemplo: "arquivos *.pdf: 12 (mínimo 1) ✓".
- **Tudo ✓ → publica:** marca `no_ar`, ou faz o deploy para um app de servidor. Depois troca o app no PC (seção 6) e avisa (seção 7).
- **Algo ✗ → reprova:** a versão no ar continua igual. A evidência fica no `ver_versao`. O Henry não recebe alerta de teste reprovado. *(Call do cofundador: falha de teste é assunto de quem construiu.)*

### 6. Troca do app de tela no PC

Vale para app de tela rodando aberto no PC no momento em que a versão nova vai ao ar:

- **Ziv saber se o app está em uso:** o Agente passa a abrir o app atrás de um pequeno repassador local. O navegador fala com o Agente, que repassa ao app. Assim o Agente sabe quando foi o último pedido feito pelo navegador e consegue mostrar o aviso na tela. *(Call do cofundador. O builder pode escolher outro jeito de saber o "último uso", desde que ele reflita uso real da tela.)*
- **"Parado" quer dizer sem uso da tela há 10 min e sem trabalho em andamento.** O app pode estar trabalhando sem ninguém olhar, como a rodada das 9h que baixa o BTG. Por isso o Agente só troca se:
  - não há rodada ou trabalho do Ziv ativo naquele app;
  - o app, se expõe a rota opcional `GET /_ziv/ocupado`, não responde "ocupado". `formato_do_app` ensina essa rota.
  
  Os pedidos automáticos que a página faz para acompanhar o andamento não contam como uso. Conta só o que vem de ação da pessoa (navegar, enviar formulário). O builder define o critério exato. *(Call do cofundador.)*
- **Sem uso há 10 min:** o Agente para o app antigo, abre o novo na mesma porta e recarrega a aba aberta, se possível.
- **Em uso:** o Agente mostra no topo da página do app uma faixa: "Versão nova pronta. [Atualizar agora]". Clicar troca na hora. Se a pessoa ignorar, a troca acontece sozinha quando o app ficar 10 min sem uso.
- Com isso, `registrar_app` e `enviar_versao` deixam de recusar por app aberto.

### 7. Avisos

Ainda não existe sininho. Cada publicação grava um **aviso** (tabela própria: empresa, app, versão, texto, data, lido) e aparece onde os avisos aparecem hoje. Texto, por exemplo: "Relatórios versão 17 no ar — testada: 12 arquivos salvos". O sininho futuro lê a mesma tabela. *(Call do cofundador.)*

### 8. MCP

| Ferramenta | O que faz |
|---|---|
| `enviar_versao` (nova) | Junta o que hoje são preparar_envio + upload + registrar_app + publicar_app_com_tela. Primeira chamada: devolve o comando de envio (como `preparar_envio`). Quando o upload chega, o Ziv inicia a sequência sozinho. Retorna o id da versão. Para um app novo, cria o app. |
| `ver_versao` (nova) | Situação e etapa atual ("testando no PC — 3 min"). No fim: resultado de cada conferência, o log mascarado, a lista de arquivos, o link da foto e o link de confirmação quando há. Espera até 30 s por mudança, como `ver_execucao`. |
| `baixar_arquivo_teste` (nova) | Devolve o conteúdo de um arquivo de evidência (texto) ou um link (binário/imagem). Só para o dono do app ou admin. |
| `registrar_app`, `publicar_app_com_tela` | Continuam funcionando, por compatibilidade, e passam a cair no mesmo fluxo de rascunho e teste. A descrição delas aponta para `enviar_versao`. |

Toda recusa ou espera diz o próximo passo. Exemplos: "falta a seção teste", "esperando confirmação: mande este link ao responsável", "esperando o PC ligar".

## Restrições

- Segurança é o principal valor do Ziv. As chaves nunca saem do caminho Cofre → app. A evidência passa pela máscara.
- O servidor é um e2-micro. Montar um app de tela leva cerca de 28 min, e o teste no servidor vai herdar essa lentidão. O `ver_versao` precisa deixar claro que está andando.
- O Agente é Go no Windows. Uma versão nova do Agente precisa da troca automática que já existe. O fluxo novo exige uma versão mínima do Agente e recusa com mensagem clara abaixo dela.
- Testes de handler em Go não podem rodar em paralelo.
- A sequência de etapas precisa sobreviver a reinício do servidor: o estado fica no banco, não em memória.

## Fica a cargo de quem constrói

- Formato exato das tabelas, das migrações e da máquina de estados.
- Como o processo no servidor anda pelas etapas (fila no Postgres, laço de verificação etc.).
- Limites exatos: tamanho da evidência, 24 h esperando PC, 7 dias de guarda.
- Desenho visual da tela de confirmação e da faixa "Versão nova pronta", seguindo DESIGN.md (preto e branco).
- Se o deploy de app de servidor promove a imagem do teste ou monta de novo.
