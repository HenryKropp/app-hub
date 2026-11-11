# MCP: o Claude entender e usar o Ziv de verdade — plano (2026-11-11)

Status: aprovado pelo Henry em conversa, a construir numa sessão nova.

## Problema
- O Claude só recebe 3 linhas sobre o Ziv (`mcpInstructions` em `backend/internal/handler/mcp_server.go`). Não sabe o que o Ziv oferece.
- O manual completo (`mcp_protocolo.md`) só chega via `formato_do_app`, na hora de **registrar** um programa pronto. Ao **escrever** um programa, o Claude não sabe nada do Ziv: fez um programinha do Henry abrindo o navegador e só mudou quando ele pediu "integrado ao Ziv".
- O protocolo copia o que o programa faz; não propõe adaptar ao jeito Ziv.
- Falta padrão na linguagem e na documentação que a pessoa vê.

## Plano
**A. Apresentação nova** (`mcpInstructions`, sempre lida): 10–15 linhas com o que o Ziv dá pronto (tela de entradas, pasta de saída, cofre, agenda, artifacts, app com tela 24h) + regra "programa para a empresa = feito para o Ziv desde o começo; antes de escrever, leia a receita".

**B. Receita "como fazer app para o Ziv"** (ferramenta nova, ex. `receita_app_ziv`): padrões certos e errados.
- Entradas pela tela do Ziv (inputs do manifesto), não `input()` no terminal.
- Resultado na pasta de saída, não caminho fixo.
- Senhas no cofre, não no código.
- Precisa de tela → app com tela publicado (`publicar_app_com_tela`), nunca `webbrowser.open`/servidor local.
- Relatório para ler → artifact. Repetição → agenda.

**C. Passo "adaptar" no registro** (`mcp_protocolo.md`): detectar os padrões errados e propor a troca, com OK da pessoa, antes de `registrar_app`.

**D. Jeito Ziv de falar**
1. Dicionário único (site e Claude): app, app com tela, artifact, execução, cofre, agenda, pasta de saída. Palavras proibidas no texto para a pessoa: manifesto, entrypoint, MCP, token, deploy. Teste Go que falha se aparecerem nas descrições/mensagens das ferramentas.
2. Respostas prontas: cada ferramenta devolve `mensagem_para_pessoa` ("Pronto: X está no Ziv. Ele pede … e entrega …. Abrir: link"); o Claude repassa.
3. Ficha padrão de todo app: Nome (o que faz, verbo + objeto), Resumo (uma frase: para quem e para quê), Como usar (até 3 passos), Pontos de atenção.
4. Guia de escolha: app / app com tela / artifact, uma linha cada.

**E. Teste de verdade**: sessão nova, sem dica, pedir o mesmo programinha. Passa se o Claude fizer do jeito Ziv sozinho e a resposta seguir o padrão.

## Ordem sugerida
A + B → D → C → E. Pouco código: mais texto, uma ferramenta nova, um teste de vocabulário. Precisa de ADR e atualizar PRD 3.11 / TASKS Fase 2b.

---

## Adendo 2026-11-11 — feedback do Claude ao publicar um app com tela pelo MCP

Uma mudança já pronta levou 6 versões (3→8) e ~20 chamadas para entrar no ar. Meta: 1 versão e ~3 passos. Estas frentes entram **antes** de A+B, porque custam tempo real hoje.

**F0 — resultado (2026-11-11, ADR 034):** a hipótese estava errada. Não houve falta de memória nem reinício do Ziv, e os ECONNREFUSED nunca chegaram ao servidor: vieram da rede do PC. **F1 feita** no mesmo dia (falta `ver_app_no_ar`).

**F0. Estabilidade (investigar primeiro).** ECONNREFUSED intermitente em ver_app, preparar_envio, enviar_chaves e no curl; conexão MCP caiu 2x. Hipótese: a VM e2-micro (1 GB) roda o Ziv e o app com tela (Chromium + Xvfb) juntos → falta de memória reinicia o Ziv. Conferir memória/OOM/reinícios na VM; limitar memória dos apps ou separar.

**F1. Publicar sem ficar no escuro.**
- `publicar_app_com_tela` devolve as últimas ~50 linhas do log quando o app não sobe (traceback). Ferramenta `ver_log_app(app_id)` para quando cai depois.
- Erro sem chutar a causa: "o processo terminou com código 1 após 2 s" ≠ "o processo está vivo, mas a porta não abriu". A dica do 0.0.0.0 só na segunda.
- Progresso: `etapa` (montando imagem, instalando, iniciando, checando porta) + tempo decorrido.
- `ver_app` com `no_ar: true/false`, última resposta e a versão que roda de fato. Não dizer "a anterior continua no ar" quando ela nunca subiu.
- Conferir o app no ar: `ver_app_no_ar(app_id, caminho)` faz um GET como a pessoa e devolve status + trecho curto. (Revisar segurança: só leitura, só apps que a pessoa vê.)

**F2. Chaves sem falso positivo.**
- `os.environ.get("X", padrão)` com padrão não é obrigatória. Nome de constante (VAR_DEMO) não é chave.
- A lista de chaves é refeita a cada versão, não acumula.
- `keys` da ficha numa versão nova é a fonte da verdade (hoje é ignorada, `mudou: false`).
- No cofre, mostrar as chaves que o Ziv acha obrigatórias com "não é chave, ignorar".
- Apagar chave pelo Claude, com confirmação (hoje só preenche e deixou lixo no cofre).

**Entra nas frentes que já existiam:**
- B (receita): documentar o ambiente do servidor: Python 3.12, código em /app, `python -m <pacote>`, porta em PORT, gunicorn disponível, sem OneDrive. Dica: testar copiando para uma pasta rasa antes de publicar.
- C (adaptar): avisar sobre `Path(__file__).parents[N]` com N grande e caminhos `C:\` (o aviso windows_paths não pegou esse caso). Ler config de demonstração por atributo do pacote, não variável de ambiente.
- C (protocolo): na primeira falha "não respondeu", pedir/ler o log em vez de tentar de novo; não mandar versões por chute.
- D (falar): explicar "rodar" num app com tela — `pode_rodar: false` + execuções no PC confundiram. App com tela = fica no ar; rodar = só script ou app no PC.

## Ordem nova
F0 → F1 → F2 → A+B → D → C → E.

## Adendo 2 — o Claude ver tudo e reagir (pedido do Henry, 2026-11-11)

O Henry quer que o Claude veja direto no Ziv todos os logs e erros e reaja sozinho.

**F3a. Ver tudo (ferramentas de leitura).**
- `ver_log_app(app_id, linhas)`: log do app com tela no servidor e das execuções de script.
- `problemas(app_id?)`: lista única do que deu errado (execução com erro, app com tela fora do ar, publicação que falhou, agenda que não rodou, chave faltando), com quando, versão, trecho do log e link.
- Só para quem cuida do app ou é admin. Senhas do cofre apagadas do log antes de sair (o Ziv conhece os valores e troca por ••••).

**F3b. Reagir.** O MCP só responde quando o Claude pergunta, então "proativo" precisa de um gatilho:
1. Na conversa: a apresentação manda o Claude, ao mexer num app, chamar `problemas` antes e avisar o que achar. Depois de publicar ou rodar, conferir o resultado sozinho.
2. Fora da conversa: o Ziv avisa quem cuida do app quando algo quebra (aviso no site/e-mail) com um botão "pedir ao Claude para corrigir".
3. Vigia automática (depois): uma rotina agendada do Claude (Claude Code/cloud) passa por `problemas` de tempos em tempos, diagnostica pelo log, prepara a correção e pede OK antes de mandar uma versão nova. Nunca publica sem a pessoa aprovar.

Ordem: F3a junto com F1 (é a mesma coisa: entregar o log); F3b.1 junto com A; F3b.2 e F3b.3 depois de E.

## Adendo 3 — testar tudo antes, do jeito de cada app (pedido do Henry, 2026-11-11)

Cada app é diferente, então o teste não é um só. Ideia: **o próprio app diz como se testa**, e o Ziv roda esse teste igual para todos.

**F4a. Ficha ganha "como testar"** (campo novo no manifesto, o Claude preenche ao registrar, a pessoa confere):
- Script: entradas de exemplo + o que deve sair (ex.: "um .xlsx na pasta de saída", "termina sem erro em até 2 min").
- App com tela: caminho de saúde (ex. `/saude` ou `/`) + um texto que deve aparecer na página.
- App com janela / executável: abre e continua vivo por N segundos.
- Agenda: o mesmo teste do script, rodado uma vez antes de ligar.

**F4b. Antes de mandar (no PC, pelo Claude).** A receita/protocolo manda o Claude testar numa cópia limpa da pasta, como o servidor vê (pasta rasa, sem OneDrive, sem C:\, sem .env da pessoa), com as entradas de exemplo. Só registra se passar. Pegaria o `parents[4]` do feedback.

**F4c. Ensaio no servidor antes de trocar.** Ao publicar um app com tela, o Ziv sobe a versão nova ao lado da que está no ar, roda o teste da ficha e **só troca se passar**. Se falhar, a antiga segue no ar (de verdade) e o Claude recebe o motivo + log (F1). Para script: botão/ferramenta "ensaiar" que roda com as entradas de exemplo sem contar como execução real.

**F4d. Teste também depois.** A vigia (F3b.3) pode rodar o teste da ficha de tempos em tempos e virar um "problema" se falhar.

Ordem: F4b entra com B (receita); F4a+F4c junto com F1 (publicação); F4d junto com a vigia.
