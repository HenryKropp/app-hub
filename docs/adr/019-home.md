# 019 - Home

**Status:** Accepted

## Context
Depois do login a pessoa caía direto no catálogo de apps. Faltava um resumo do dia: o que pede atenção, o que ela mais usa e, para quem lidera, como o time está usando o Ziv. O desenho está em `docs/superpowers/specs/2026-11-03-home-e-artifacts-design.md` (seção 5).

## Decision
- **Duas tabelas novas** (migração 018, dentro do esquema de cada empresa): `favorites` (pessoa, tipo `app` ou `artifact`, item, data; chave primária nos três primeiros) e `artifact_opens` (artifact, pessoa, quando abriu). Execuções de apps já existem em `runs`, então só os artifacts precisavam de registro de uso.
- **A Home lê dados que já existem, sem feed guardado.** `GET /api/home` (Meu dia) e `GET /api/home/team` (Meu time) montam a resposta na hora a partir de `runs`, `computers`, `apps`, `artifacts`, versões, convites e pedidos de acesso. Todo item mostrado (favorito, recente, aviso, novidade do time) passa pela checagem de acesso do próprio item (`acc.For` para apps, `acc.ForArtifact` para artifacts): o que a pessoa não pode ver não aparece e o nome não vaza. Marcar com estrela algo que não se pode ver responde 404 e não grava nada.
- **Navegação:** `/` é a Home; o catálogo foi para `/apps`. "Home" só fica marcada em `/`; "Apps" fica marcada em `/apps` e `/apps/:id`.
- **Meu time:** quem tem a permissão `manage_roles` vê a empresa toda; quem lidera uma ou mais áreas vê as pessoas (membros e líderes) e os itens dessas áreas, somados; os demais recebem 404 e a tela esconde a seção. "Pendências de gestão" só aparecem para quem tem `manage_people` e sempre cobrem a empresa toda.
- **Números fixos:** execução parada esperando o computador há mais de 5 minutos, ou já marcada como interrompida (o Hub a interrompe em cerca de 2 minutos sem resposta do Agente; é o caso real, aviso `run_interrupted`); janela de avisos de 7 dias; 6 itens em "Usados recentemente" (janela de 30 dias); 5 últimas execuções; "Quem está usando" em 7 dias; "O que estão criando" em 14 dias.

### Escolhas do cofundador no plano
1. **"App com ponto de atenção"** = app que a pessoa cuida (responsável, reserva ou líder da área) com pelo menos uma execução **com erro, de qualquer pessoa, nos últimos 7 dias**. Os "Pontos de atenção" da ficha são anotações fixas; usá-los deixaria o aviso sempre cheio para todo responsável.
2. **"Apps e artifacts sem responsável ativo" ficou de fora.** Hoje o Ziv não desativa nem remove pessoas, então o responsável sempre está ativo e a lista seria sempre vazia. Volta quando existir "desativar pessoa".
3. **Abrir um artifact** é registrado quando o detalhe é carregado (`GET /api/artifacts/{id}`), no máximo uma vez a cada 30 minutos por pessoa e artifact. Salvar ou editar não conta.
4. **Convites não aceitos** = nem aceitos nem cancelados; os vencidos mostram a marca "venceu". Pedidos de acesso pendentes (`membership_requests` com status `pending`) também entram em "Pendências de gestão".
5. **O aviso de execução pode ser dispensado na própria Home**, pelo `POST /api/runs/{id}/dismiss` que já existia ("Dispensar"). Os outros avisos somem quando a causa é resolvida.
6. **Sem computador, sem aviso de computador:** quem nunca conectou um computador não recebe aviso; o "Começar" já cobre esse caso.

## Rationale
Ler do que já existe evita um segundo lugar para manter em dia e nunca fica desatualizado em relação às execuções e fichas. A checagem de acesso por item é a mesma dos catálogos, então não há regra nova para explicar. Duas tabelas pequenas bastam porque só favoritos e aberturas de artifact não existiam em lugar nenhum.

## Trade-offs
**Pros:**
- Pouco para limpar: `favorites` não tem chave estrangeira para o item. O favorito de um artifact é apagado explicitamente ao apagar o artifact (`DeleteItemFavorites`); apps não podem ser apagados hoje; e linhas órfãs ficam escondidas pelo filtro de acesso. Só `artifact_opens` e a exclusão da pessoa usam `ON DELETE CASCADE`.
- Mesmas regras de acesso em todo lugar.

**Cons:**
- A resposta é calculada a cada abertura da Home, com várias consultas. Hoje é barato; se a empresa crescer muito, pode pedir cache.
- Favorito de algo que ficou restrito some sem aviso (por desenho, para não vazar o nome).
- "Ainda não usaram" do líder conta uso em qualquer lugar: quem só usou itens fora das áreas do líder não aparece nem em "Quem está usando" nem em "Ainda não usaram" (escolha consciente).
- `artifact_opens` só cresce; não há limpeza de aberturas antigas.
- "Apps e artifacts sem responsável ativo" ainda não existe.

## Alternatives Considered
- **Tabela de "feed de atividade" gravada a cada evento:** duplicaria o que `runs`, versões e convites já dizem, exigiria preencher o passado e poderia mostrar item que a pessoa perdeu o acesso; descartado.
- **Contar aberturas de artifact pela rota do conteúdo:** o conteúdo é carregado de dentro do iframe, pela prévia, por versões antigas e por atualizações, então contaria mais de uma vez e sem saber quem abriu na página; o detalhe (`GET /api/artifacts/{id}`) é o ponto que representa "a pessoa abriu".
- **Usar os "Pontos de atenção" da ficha como avisos:** são notas fixas escritas na ficha, não um estado; todo responsável teria avisos sempre; descartado.
