# 028 - Cofre em todo app, app com tela no computador e `enviar_chaves`

**Status:** Accepted

> A tela do Cofre (item "Cofre" no menu e "Configurações" dentro de Servidor) foi substituída pela aba Configurações do ADR 032.

## Context
O Cofre do app (ADR 027) só existia para scripts. Os dois apps reais do Henry são do tipo Navegador (app com tela), então ele nunca viu o Cofre. Apps com tela compartilhados já tinham "Configurações" (`app_config`, com segredos selados), que chegam ao contêiner da versão 24h. Um app com tela ainda não abria pelo Agente: rodava só da pasta, com o `.env` da pasta.

Em 2026-11-09 o Henry decidiu:
- o Cofre aparece em todo app que roda;
- uma chave preenchida uma vez vale no computador e na versão 24h;
- o MCP preenche o máximo possível de senhas, sem que elas passem pela conversa;
- "Fazer tudo agora": o Agente também abre apps com tela no computador.

## Decision
- **Cofre = segredos de `app_config`.** `manifest.HasVault` vale para script, web, desktop, agendado e executável. O cofre lista as `keys` da ficha, os segredos de `shared.env` e as linhas seladas adicionadas pelo site. As linhas que começam com `_` (ex.: `_db_password`) são do Ziv e ficam de fora. Configurações não secretas continuam na parte Configurações. Gravar ou apagar uma chave de um app ligado no servidor responde `needs_restart`, e o site oferece reiniciar.
- **App com tela no computador (Agente 0.3.5).**
  - O pedido leva `RunPayload.Web`: `runtime`, `args` (com `{PORT}`), o arquivo de dependências, as bibliotecas extras e o código guardado no Ziv. Nunca leva as chaves, que seguem pela rota de uso único do ADR 027.
  - O servidor traduz a receita do contêiner (`recipes.LocalStart`): streamlit, uvicorn e flask viram `python -m …`, gunicorn vira waitress (roda no Windows), e Node usa `npm start`. Dockerfile não abre no computador.
  - O Agente escreve o código numa pasta própria por app. As bibliotecas Python vão para uma pasta só do Agente (`pip --target`, guardada por conjunto de dependências), e as de Node com `npm install --ignore-scripts`. Ele liga o app em `127.0.0.1` numa porta livre, com `PORT`, `BASE_URL`, `APPHUB_DATA_DIR` (a pasta escolhida) e as chaves no ambiente. Depois abre o navegador, e "Parar" desliga a árvore de processos.
  - Se o app fecha sozinho, o console fica num `ziv-app-N.txt` na pasta escolhida, nunca no Ziv.
- **`enviar_chaves` (MCP).** Dá um endereço de uso único (30 minutos, até 5 abertos por pessoa e app, tabela `mcp_key_uploads`). O Claude roda `curl` mandando o `.env` da pasta direto para `PUT /api/mcp/chaves/{token}`, sem abrir o arquivo. O Ziv confere de novo quem pode mexer no cofre e guarda selado o que for válido. Configurações não secretas vão como texto. A resposta traz só os nomes (guardadas, configurações, ignoradas), e o registro vai para `security_audit` (`app_key.imported`).

## Rationale
- Uma linha só em `app_config` evita duas fontes de verdade para a mesma senha. A versão 24h já recebia essas linhas.
- O código guardado no Ziv é a versão aprovada. Rodar da pasta da pessoa deixaria passar código não revisto.
- O `curl` direto tira os valores da conversa com o Claude, que é onde mais vazam.

## Trade-offs
**Pros:**
- O Henry preenche uma vez e o app funciona no PC e no servidor.
- O `.env` deixa de ser necessário, e o Claude preenche o cofre sem ver as senhas.

**Cons:**
- Abrir um app com tela no computador exige o Agente exatamente na 0.3.5 e Python ou Node instalados.
- A primeira abertura baixa as bibliotecas e pode demorar.
- O `curl` depende do Claude seguir a instrução de não abrir o arquivo. O texto da ferramenta diz isso explicitamente.

## Alternatives Considered
- **Cofre separado das Configurações:** a mesma senha ficaria em dois lugares e um deles ficaria desatualizado.
- **Rodar da pasta da pessoa:** a pessoa pode ter código diferente do aprovado, e isso não se encaixa no modelo de código guardado.
- **Claude lendo o `.env` e chamando uma ferramenta com os valores:** as senhas passariam pela conversa.
