# 023 - Entrada por e-mail, Começar em 6 passos e Configurações só com status

**Status:** Accepted

## Context
No primeiro uso do Ziv no servidor (ADR 022), em 2026-11-05, o Henry se confundiu em três pontos.
- **Duas entradas.** Criou a empresa "Teste" e depois teve que entrar de novo na empresa *principal*, que é outra empresa.
- **Cabeçalho.** O logo e o cabeçalho da página inicial estavam pequenos.
- **Depois de entrar.** Não ficava claro o que fazer. As Configurações misturavam o status das conexões com o passo a passo para conectar.

Desenho completo: `docs/superpowers/specs/2026-11-05-entrada-e-comecar-design.md`.

## Decision
- **Entrar só com e-mail e senha.** `/acesso` deixa de pedir o endereço da empresa.
  - A principal expõe `POST /api/entry/login`, que procura o e-mail em todas as empresas de clientes, menos na principal.
  - A rota devolve só as empresas em que a senha bateu e não abre sessão. Com uma empresa, o site entra direto nela. Com várias, pergunta "Em qual empresa?" e entra pela rota de login da empresa escolhida.
  - Sem acerto, devolve 401 "E-mail ou senha não conferem.", a mesma mensagem de sempre.
  - Quando o e-mail não existe em lugar nenhum, roda um bcrypt de mentira com o mesmo custo (12), para o tempo de resposta ficar parecido.
  - A rota fica sob `/api/entry/` e herda o limite de tentativas do cadastro.
  - Entrar com Google e Microsoft fica para depois; a rota não fecha essa porta.
- **A principal fica escondida.** Nenhuma página pública aponta para ela; ela vira o painel do dono do Ziv.
  - As rotas da raiz continuam acessíveis para quem digitar o endereço.
  - O primeiro acesso dela num servidor novo passa a pedir o `SETUP_CODE` do servidor, comparado em tempo constante. Sem `SETUP_CODE` definido, o primeiro acesso é recusado, a não ser com `DEV_MODE`.
  - A principal não mostra o Começar.
- **O cadastro leva ao Começar.** Quem cria a empresa já sai logado em `/w/<empresa>/comecar`. Ao entrar, quem ainda tem passo do Começar por fazer cai nele; quem terminou cai na Home.
- **Começar em lista**, com o primeiro passo não feito aberto. São 6 passos: Computador, Claude, Subir app, Rodar, Artifact e Convidar.
  - O progresso vem do `/api/onboarding`, que ganhou `claude_connected` e `has_artifacts`.
  - `claude_connected` só conta conexão ativa que o Claude já usou (`last_used_at`); gerar um código sem usar não conta.
  - "Subir app" e "Convidar" só aparecem para quem tem permissão.
  - `/instalar-agente` passa a levar a `/comecar#computador`.
- **Configurações só com status.** Cada computador e cada conexão do Claude aparecem com o status e um Desconectar. O que falta conectar aparece como "Conectar no Começar".
- **Cabeçalho da página inicial maior:** logo de 48px, barra de 84px e menu de 16px.

## Consequences
- **Tempo de resposta.** O tempo cresce com o número de empresas em que o e-mail existe. Isso revela quantas são, não quais.
- **Limite de tentativas.** O limite do `/api/entry/` (20 por endereço por janela) vale também para o login. Num escritório atrás de um único IP, isso pode apertar; dá para subir o limite depois.
- **Desempenho.** Cada login percorre todas as empresas prontas. Isso está bem para poucas empresas, mas precisa de outro índice quando passarem de dezenas.
- **Compartilhados.** As empresas de clientes continuam sem apps compartilhados (ADR 015). O Henry, que usa uma empresa normal, fica sem eles até o projeto próprio.
