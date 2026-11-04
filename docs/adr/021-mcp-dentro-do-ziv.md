# 021 - MCP dentro do Ziv

**Status:** Accepted

## Context
O Henry quer usar o Ziv de dentro do Claude, tanto no claude.ai e no app do Claude quanto no Claude Code. Ele listou cinco usos:
- registrar apps prontos;
- criar apps já pensando no Ziv;
- publicar artifacts "bonitinhos";
- agendar rodadas;
- achar e rodar apps.

Em 2026-11-04 ele aprovou o desenho (`docs/superpowers/specs/2026-11-04-mcp-design.md`), que divide o trabalho em quatro etapas:
1. conectar, achar, rodar e publicar artifacts;
2. registrar apps prontos;
3. criar apps;
4. agendar.

Este ADR cobre a Etapa 1 e a forma técnica que as próximas etapas vão herdar.

## Decision
- **O MCP mora no próprio binário do Ziv**, no endereço `/mcp`; numa empresa, `/w/<empresa>/mcp`. Usa o SDK oficial de MCP para Go, em HTTP streamable sem estado.
- **Ferramentas chamam os handlers por dentro do processo.**
  - A função `callAPI` chama o roteador da API com a identidade da pessoa no contexto. Assim, as regras de acesso são as mesmas das telas, sem reimplementação.
  - `callAPI` só alcança uma lista fechada de 11 rotas, e o `{id}` só aceita dígitos. Como `callAPI` pula o middleware `Secure`, as ações que mudam algo (rodar, publicar) gravam o próprio registro em `security_audit`, como `mcp.request.*`.
- **Duas formas de entrar:**
  - **OAuth 2.1 para o claude.ai e o app do Claude:** o Claude se cadastra sozinho como cliente (DCR), PKCE S256 é obrigatório e a chave de renovação troca a cada uso. A chave de acesso vale 1 h, a de renovação 30 dias e o código de autorização 60 s, com uso único. A pessoa confirma na tela do Ziv `/conectar-claude/autorizar`, que mostra para onde o acesso vai e avisa quando o endereço não é do Claude.
  - **Código pessoal para o Claude Code:** é gerado em Configurações e mostrado uma vez só.
  - As duas formas ficam em `mcp_connections`, guardadas só como hash SHA-256 e independentes da sessão do navegador. Cancelar uma conexão ou remover a pessoa corta o acesso na hora.
- **Descoberta OAuth por empresa:** o domínio é um só, e cada empresa anuncia os próprios metadados com o sufixo de caminho (`/.well-known/oauth-protected-resource/w/<empresa>/mcp`). Uma chave de uma empresa não vale em outra, porque cada empresa tem o próprio schema.
- **Acompanhar até terminar:** o MCP não consegue avisar o Claude sozinho. Por isso `ver_execucao` espera até 30 s por uma mudança, e a descrição de `rodar_app` pede ao Claude que chame `ver_execucao` de novo e de novo até o fim.
- **Origem "pelo Claude":** `runs.origin` e `artifacts.origin` passam a valer `mcp`, e as telas mostram o selo "pelo Claude".

## Rationale
- Ficar dentro do binário dispensa instalar qualquer coisa: o mesmo endereço serve aos dois Claudes, e o Ziv publicado já tem HTTPS.
- Chamar os handlers garante que o Claude nunca faça mais que a pessoa faria pela tela.
- O OAuth é o que o claude.ai exige. O código pessoal é o caminho simples para o Claude Code e o que dá para testar no computador antes de publicar.

## Trade-offs
**Pros:**
- Nada para baixar, um serviço a menos para operar, e as regras de acesso valem num lugar só.
- Funciona em várias empresas sem domínio extra.

**Cons:**
- `callAPI` pula o `Secure`, o que exige a lista fechada de rotas e o registro de auditoria próprio. Cada etapa nova precisa liberar as rotas dela de forma deliberada.
- O cadastro aberto de clientes OAuth permite phishing de consentimento. A defesa é a tela mostrar o endereço de destino e avisar quando ele não é do Claude.
- Quando o Ziv ganhar "desativar pessoa", é preciso revogar as `mcp_connections` dela, e `mcpConnection`, a chave de renovação e o código de acesso precisam checar que a pessoa está ativa. Hoje só apagar a pessoa corta o acesso.
- O claude.ai só pode ser testado depois que o Ziv estiver publicado na internet.

## Alternatives Considered
- **Servidor MCP local (stdio) junto com o Agente:** só serviria ao Claude Code e ao app no Windows, não ao claude.ai, e exigiria atualizar o Agente a cada ferramenta nova.
- **Serviço MCP separado:** mais um serviço para publicar e manter, com regras de acesso duplicadas ou expostas por outra API.
- **Só chaves de API, sem OAuth:** o claude.ai não aceita esse caminho para conectores pessoais, e a pessoa teria que copiar segredos à mão.
