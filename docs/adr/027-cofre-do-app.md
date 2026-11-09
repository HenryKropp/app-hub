# 027 - Cofre do app: chaves de scripts guardadas no Ziv

**Status:** Accepted

## Context
Em 2026-11-09, o Henry testou a Etapa 2 do MCP (ADR 026) e tentou subir pelo Claude um script de verdade, que usa e-mail e senha. O Ziv recusou o envio três vezes porque achou senhas no código, e a empresa Teste bloqueia senhas. Até então só os apps com tela guardavam chaves, e o "Cofre de credenciais" (`docs/CREDENTIAL_VAULT.md`) nunca entrega valores a scripts. Ele também está fechado no servidor, sem `CREDENTIAL_VAULT_KEYRING`.

O Henry decidiu três coisas:
- cada app tem o seu cofre;
- só o dono do app e os administradores mexem nele;
- os dois podem ver o valor, e cada visualização fica registrada.

Desenho: `docs/superpowers/specs/2026-11-09-cofre-do-app-design.md`.

## Decision
- **A ficha do script declara as chaves e o cofre guarda os valores.**
  - A ficha ganha `keys: [{name, label, required}]`, só para scripts, com no máximo 20 chaves. Ficam de fora os nomes que o Agente ou o Python usam (`APP_HUB_`, `PYTHON`, `NODE_`, `NPM_`, `PIP_`, `PATH`).
  - Os valores ficam em `app_config` (a tabela dos apps com tela), com `is_secret`, cifrados com `HUB_SECRET_KEY` (AES-256-GCM, pacote `vault`).
- **Quem mexe:** só o dono do app (`owner_id`) ou quem tem `manage_roles` pode gravar, apagar e mostrar. Quem pode ver o app vê os nomes e se cada chave está preenchida.
- **Rotas:**
  - `GET /api/apps/{id}/keys`
  - `PUT /api/apps/{id}/keys/{name}`
  - `DELETE /api/apps/{id}/keys/{name}`
  - `POST /api/apps/{id}/keys/{name}/reveal`
  
  Gravar, apagar, mostrar e entregar ao Agente escrevem em `security_audit` (`app_key.saved`, `app_key.deleted`, `app_key.revealed`, `app_key.delivered`), sempre com a rota e nunca com o valor.
- **Como o valor chega ao script:**
  - O valor nunca entra em `agent_jobs.payload`. O pedido de execução só marca `keys: true`.
  - O Agente 0.3.4 chama `GET /api/agent/jobs/{id}/keys` uma única vez por trabalho. A rota só atende um trabalho de execução daquele computador que ainda está em andamento, e a migração 022 marca `keys_delivered_at`.
  - O Agente guarda os valores só na memória (`RunPayload.KeyValues`, `json:"-"`) e os põe no ambiente do script depois da instalação de dependências.
  - Se faltar uma chave obrigatória, a execução é recusada antes de ir ao PC (409 `keys_missing`). Se o cofre estiver fechado, a resposta é 503 `keys_locked`.
- **Pelo Claude:**
  - `registrar_app` aceita `keys`, também numa versão nova ("chaves do cofre"), e responde com as chaves que faltam e o link `/apps/N#chaves`.
  - A recusa por senhas no código aponta arquivo, linha e nome (nunca o valor) e manda mover a senha para `os.environ["NOME"]` mais `keys`.
- **Site:** a página do script ganha a parte "Cofre", com preencher, mostrar e esconder, trocar e apagar.

## Adendo (2026-11-09): chaves adicionadas pelo site
- A parte "Cofre" aparece em todo script, mesmo sem `keys` na ficha, e o dono ou um administrador usa "Adicionar chave" (nome + valor). O site transforma o que a pessoa digita no nome que o script lê ("api token" vira `API_TOKEN`).
- Uma chave adicionada pelo site não muda a ficha: ela é uma linha selada em `app_config`. Para um script, toda linha selada de `app_config` é chave do cofre (os scripts não usam `app_config` para mais nada). As chaves da ficha vêm primeiro, depois as do site, por nome.
- Valem as mesmas regras de nome (letras maiúsculas, números e `_`; nada de nomes reservados) e o mesmo limite de 20 chaves no total. "Apagar" esvazia uma chave da ficha e tira da lista uma chave do site.
- A execução entrega ao Agente todas as chaves preenchidas, da ficha ou do site. Só as da ficha marcadas como obrigatórias impedem de rodar.
- O item "Cofre (em breve)" saiu do menu, e os avisos de senha no código passam a mandar para a parte Cofre.

## Consequences
- Um script com senha sobe pelo Claude sem a senha passar pela conversa, e roda com a senha vinda do Ziv.
- A regra antiga de que nenhum valor vai ao Agente muda para os scripts: o valor viaja pela conexão do Agente, que já é autenticada e usa HTTPS, só na hora de rodar, e não fica gravado em disco.
- O script ainda pode imprimir a própria chave. Nesse caso ela aparece só no console da pasta de resultados de quem rodou.
- O cofre de credenciais antigo continua separado e sem mudança.
- Para rodar scripts, todo Agente precisa atualizar para a 0.3.4, o que acontece sozinho.

Relacionados: ADR 010 (apps compartilhados, `app_config`), ADR 026 (registrar pelo Claude).
