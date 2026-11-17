# 046 - Governança: quem aprova, quem usa, por onde sobe

**Status:** Accepted

## Context
O Henry pôs o Projeto 2 dos links externos em espera (ADR 045) e pediu,
antes, para melhorar acessos, segurança e dados, pensando no fluxo dentro de
uma empresa: alguém faz uma automação e outros aprovam. Antes, a aprovação
era uma chave da empresa ("Exigir aprovação"), com um "aprovado sim/não" por
app, sem histórico por versão, sem artifacts, e quem cuidava do app decidia
quem usava. Desenho aprovado em 2026-11-17
(`docs/superpowers/specs/2026-11-17-governanca-aprovacoes-design.md`).

## Decision
**Decisões do Henry:**
- O **líder da área sempre aprova** app e artifact novos e, ao aprovar,
  escolhe quem usa: só a área, a área e mais áreas ou pessoas, ou a empresa
  toda. Depois disso, mudar o público é só do líder (ou de quem gerencia
  áreas).
- A **empresa escolhe se a TI também aprova** (permissão de cargo nova
  "Aprovar pela TI").
- **Regra de versões flexível:** toda versão / só quando muda o risco / só a
  primeira. Cada app ou artifact pode ser **mais rígido**; afrouxar abaixo do
  padrão da empresa, só o administrador.
- Opção **"Só pelo Claude"**: o site e a pasta do PC passam a recusar código.
  A empresa também pode limitar quem tem chave do MCP (permissão "Conectar o
  Claude"), e o Admin lista as chaves e cancela.
- **Artifacts no mesmo fluxo.**
- Empresa nova começa leve (só líder, "só quando muda o risco", qualquer
  caminho). Empresa que já exigia aprovação (a Empresa Piloto) migra para líder + TI +
  toda versão, e o cargo Administrador ganha "Aprovar pela TI".
- Construído tudo de uma vez, antes de ir ao ar.

**Como foi feito (minhas decisões):**
- **Pedidos com etapas** (`approval_requests` + `approval_steps`, migrações
  030 e 031). Motivos: novo, versão, ficha, área. Situações: aberto,
  aprovado, recusado, automático (foi ao ar com aviso), substituído. Cada
  etapa guarda quem decidiu, quando, o motivo e se foi no lugar de alguém
  (admin numa área sem líder; o próprio criador líder ou TI).
- **O que está no ar** fica em `app_governance.approved_version`. A versão
  aprovada continua rodando enquanto a nova espera; a execução usa sempre a
  aprovada e fecha se aparecer uma versão sem aprovação.
- **Risco** (`internal/risk`): chaves do Cofre, domínios lidos do código,
  navegador (playwright/selenium/puppeteer), 24h no servidor ou PC. Muda o
  risco quando aparece algo novo; o pedido mostra o que entrou.
- **Regra de acesso num lugar só:** `access.Can` ganhou `Audience` (líder ou
  `manage_areas`); `Access` continua sendo "cuida do app". Item pendente só
  aparece para quem criou, quem aprova e quem audita.
- **Rascunhos (ADR 043/044)**: a versão testada que a regra manda aprovar
  fica "aguardando"; o pedido diz que o teste passou.
- **Ficha que ganha risco** (chave do Cofre, 24h no servidor), por qualquer
  caminho que salva a ficha: o app **pausa** até o líder decidir. Isso se
  afasta do desenho (que mantinha a ficha aprovada valendo num rascunho):
  a ficha é o arquivo na pasta e o app roda o que está lá, então mantê-la
  seria um rascunho só de ficha. Quem muda a ficha escolhe a hora; o pedido
  diz o risco que entrou.
- **Revisão final (2026-11-17):** a checagem de ficha e de troca de área
  vale em todo caminho que salva a ficha; troca de área de artifact também
  espera o líder novo; app sem registro de governança fica retido; pedido
  substituído ou que perdeu o rascunho não aprova a versão mais nova; versão
  achada na pasta não é aprovada em nome do dono; "só pelo Claude" recusa
  voltar para uma versão da pasta nunca aprovada; a empresa apertar a regra
  alcança itens com regra mais frouxa.
- **MCP:** as ferramentas respondem com a situação do pedido ("Aguardando o
  líder de Research.").
- **Tela:** página **Aprovações** com contador no menu, bloco "Esperando sua
  aprovação" no Início, situação e histórico na página do app e do artifact,
  "Aprovação de versões" em Configurações, e seção **Governança** no Admin
  (TI, regra, caminho, chaves do MCP, histórico da empresa).

## Rationale
O pedido com etapas responde "quem aprovou esta versão, e quando?", deixa
cada empresa apertar o quanto quiser e não trava quem já usa o app.

## Trade-offs
**Pros:**
- Histórico completo por item e da empresa.
- Empresa pequena não trava: quem é líder e TI aprova o próprio pedido sozinho.
- App aprovado nunca para por causa de uma versão nova.

**Cons:**
- Mais tabelas e mais um passo para publicar.
- O Ziv garante o caminho (só MCP) e quem tem chave, mas não prova qual conta
  de Claude está do outro lado.
- Aviso aos aprovadores só dentro do Ziv; e-mail depois do domínio.

## Alternatives Considered
- **Remendar o que existia** (marca "TI aprovou" ao lado de "aprovado"):
  rápido, mas sem histórico por versão; cada regra nova seria outro remendo.
- **Motor de regras genérico** (cadeias montadas pela empresa): flexível
  demais para o pedido, difícil de usar e de construir.
