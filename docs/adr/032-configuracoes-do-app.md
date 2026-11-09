# 032 - Configurações do app: senhas e ajustes num lugar só

**Status:** Accepted

> Substitui a parte de tela do Cofre do ADR 028 (item "Cofre" no menu e a seção Configurações dentro de Servidor). O armazenamento, o envio ao Agente e o `enviar_chaves` do ADR 028 continuam valendo.

## Context
Na página do app, a face Cuidar tinha dois lugares para a mesma coisa: o item **Cofre** (senhas do app, em todo app que roda) e a seção **Configurações** escondida dentro de **Servidor** (só para apps compartilhados, misturando senhas e ajustes comuns). Por baixo, os dois gravam na mesma tabela `app_config`. O Henry achou confuso: não dá para saber onde fica uma senha, nem por que a pasta de saída de um app está "no servidor".

## Decision
- **Uma aba "Configurações"** no menu de Cuidar, logo abaixo de Ficha (grupo App), para todo app que roda (script, web, desktop, agendado, executável, e qualquer app compartilhado). A palavra "Cofre" sai da página do app, das mensagens, das dicas do Claude (MCP) e da landing. O nome das ferramentas (`enviar_chaves`) e das rotas (`/keys`) fica igual, por compatibilidade.
- **Dois blocos:**
  - **Chaves e senhas**: os segredos de sempre. Nomes e estado para quem vê o app; preencher, mostrar, trocar e apagar só para o dono ou um administrador. Mostrar continua registrado no histórico de segurança (`app_key.revealed`).
  - **Outros ajustes**: valores comuns (uma pasta, um e-mail), mostrados como estão e editáveis pelo dono ou administrador. Quem só vê o app vê o nome e se está preenchido, não o valor.
- **Adicionar valor**: nome, valor e a caixa "É uma senha (fica escondida)", marcada por padrão. Marcada, grava pelo caminho das chaves; desmarcada, como ajuste comum.
- **Mesmos valores no computador e no servidor.** Os ajustes comuns passam a ir também para o Agente, junto com as chaves (`keyValues`). Valor padrão (`default`) da ficha continua só na versão do servidor. A aba explica isso numa frase e mantém o "religue o app para valer" com o botão Religar agora.
- **API:** `GET /api/apps/{id}/settings` devolve a lista única (`items` com `secret`, `can_manage`, `locked`, `missing`). Ajustes comuns gravam em `PUT/DELETE /api/apps/{id}/settings/{name}` (registro `app_setting.saved` e `app_setting.deleted`). Senhas continuam em `/api/apps/{id}/keys/...`. As rotas antigas (`/keys` e `/shared/config`) seguem funcionando. Um nome de senha nunca vira ajuste comum por essa rota.
- **Pendências:** a ficha traz `missing_settings` (os obrigatórios vazios). Se houver algum, Pendências mostra "Preencher as configurações" com o botão **Ir para Configurações**. O link que o Claude manda (`/apps/N#chaves`) abre direto nessa aba.
- **Servidor** fica com estado, publicar, cópias dos dados e parte técnica.

## Rationale
- Um lugar só para tudo que o app lê do ambiente. A pessoa não precisa saber se o app é compartilhado para achar uma senha.
- A divisão que importa para quem usa é "escondido ou não", não "computador ou servidor".
- Reaproveitar as rotas de chaves mantém a auditoria e as regras de permissão sem duplicar código de segurança.

## Trade-offs
**Pros:**
- Some a confusão Cofre x Configurações; a aba é igual em todo app que roda.
- Ajustes comuns agora valem também no computador.

**Cons:**
- Quem tinha só a permissão de restaurar (líder de área, "restaurar qualquer app") editava as Configurações do servidor; agora só o dono ou um administrador muda valores pela tela. A rota antiga `/shared/config` ainda aceita essa permissão.
- "Adicionar configuração" de Servidor também declarava o valor na ficha (com nome para as pessoas e obrigatório). Agora um valor novo é só nome e valor; o nome amigável e o "obrigatório" vêm da ficha (pelo Claude).
- Um ajuste comum que só fazia sentido no servidor também chega ao computador.

## Alternatives
- **Manter Cofre e mover Configurações para uma aba própria.** Rejeitada: continuariam dois lugares para valores do mesmo tipo.
- **Tudo como senha.** Rejeitada: uma pasta ou um e-mail escondidos atrapalham quem confere o app.
- **Juntar tudo numa rota nova, inclusive senhas.** Rejeitada por ora: as rotas de chaves já têm auditoria e são usadas pelo Agente e pelo MCP; mudar não traz ganho para quem usa.

## Adendo (2026-11-09): senha pelo nome e "Tornar senha"

Henry achou a "BTG senha" de um app aberta em Outros ajustes, porque a ficha não a marcava como senha. A correção:

- **Regra pelo nome.** Um valor cujo nome ou rótulo tem cara de senha é sempre tratado como senha, diga a ficha o que disser. Contam como cara de senha: senha, password, pwd, token, segredo, secret, api key e chave de API, de acesso ou secreta.
- **Valores que já estavam abertos.** São trancados na próxima vez que alguém abre as Configurações do app ou roda o app. Para isso o servidor precisa da chave dele.
- **Ajustes novos.** Um ajuste comum novo com nome de senha é recusado.
- **Botão "Tornar senha".** Cada ajuste comum ganhou esse botão, sem volta. O uso fica registrado no histórico de segurança (`app_setting.made_secret`).

A mesma regra de nome é a que esconde valores nas execuções (ADR 033).
