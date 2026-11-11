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
