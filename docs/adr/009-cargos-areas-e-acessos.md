# 009 - Cargos, áreas e acessos por app

**Status:** Accepted

## Context
Até a Etapa 1 cada pessoa tinha um "role" fixo (administrador, criador ou usuário). Isso não dava conta do que o Henry pediu: um líder que cuida só dos apps da sua área, um colega que pode editar a ficha de um app e nada mais, apps que só algumas pessoas enxergam, e a parte técnica da ficha escondida de quem só usa. Desenho completo em `docs/superpowers/specs/2026-10-05-etapa-2-permissoes-e-ficha-design.md`.

## Decision
- **Cargo por pessoa**, um só, personalizável, com 9 permissões da empresa toda: `register_apps`, `edit_any_app`, `restore_any_app`, `see_tech_any_app`, `see_all_runs`, `see_all_apps`, `manage_people`, `manage_areas`, `manage_roles`. Já vêm prontos Administrador, Criador e Usuário (podem ser renomeados e mudados, não apagados).
- **Áreas** são uma lista gerenciada. Todo app pertence a uma. Cada área tem líderes e membros. O líder pode, nos apps da área, o que um administrador pode, e dá e tira acessos só neles.
- **Acessos extras** por pessoa, app a app (ou "todos os apps da área"): `edit`, `restore`, `tech`, `runs`.
- **App aberto ou restrito.** Aberto: todos veem e rodam. Restrito: vê quem tem `see_all_apps`, as áreas e pessoas escolhidas, quem cuida (responsável, reserva, líder da área), quem tem `manage_areas` e quem recebeu qualquer acesso extra nele (no app ou na área toda). Quem não pode ver recebe 404 "App não encontrado.", igual a um app que não existe, e não ganha nada nele. Todo app novo nasce aberto.
- **Permissões da empresa toda valem só no que se vê.** `edit_any_app`, `restore_any_app`, `see_tech_any_app` e `see_all_runs` não revelam um app restrito: valem só nos apps que a pessoa já enxerga, e a lista "De todos" das execuções só traz apps visíveis. Um cargo de suporte com `see_all_runs` não abre apps confidenciais sem também ter `see_all_apps`.
- **A regra mora num lugar só**, `backend/internal/access`: uma função recebe a pessoa e o app e devolve as ações permitidas. Todo handler a usa. O frontend só lê o `can` que vem pronto com cada app e nunca deduz permissão.
- **Sempre existe ao menos uma pessoa** com `manage_people` e `manage_roles`; o Hub recusa a mudança que quebraria isso.
- A ficha vira uma página só (desenho B), com a parte técnica num bloco recolhido que só aparece para quem pode vê-la, e um "Editar" por parte.

### Regras de produto decididas durante a construção
- Quem pode ver as execuções dos outros também pode baixar os arquivos dessas execuções.
- Quem pode editar um app **não** muda o responsável, a reserva nem a área dele: isso pede `can.access` (responsável, reserva, líder da área ou `manage_areas`), senão 403 "Só quem cuida do acesso do app pode trocar o responsável, a reserva ou a área." Vale para a ficha e para republicar a pasta. Motivo: quem só edita poderia se pôr como responsável e ganhar voltar versões e controle de acesso. (Substitui a regra anterior, que deixava o editor trocar os três.)
- Ninguém dá um cargo com mais permissões do que o próprio (ao mudar o cargo e ao cadastrar a pessoa), nem muda o cargo de quem tem permissões que ele não tem: 403 "Você não pode mudar o cargo de quem tem mais permissões que você." Assim um cargo "RH" só com `manage_people` não rebaixa um administrador.
- Exceção à regra do 404 nas execuções: a pessoa continua vendo e baixando as **próprias** execuções de um app que depois ficou invisível para ela (são arquivos dela). O nome do app continua no histórico dela.
- Acessos extras: salvar os de um app troca só os acessos daquele app; salvar os da área toda troca só os da área. Um não apaga o outro.
- A aba Pessoas da tela Equipe é só para quem tem `manage_people`.

## Rationale
Permissões separadas por ação (ver, editar, voltar versão, ver o técnico, ver execuções) deixam dar só o necessário. Centralizar o cálculo evita que a tela mostre um botão que o servidor recusa, ou o contrário. Responder 404 em vez de 403 não revela que um app restrito existe.

## Trade-offs
**Pros:**
- Cobre líder de área, colega com edição pontual e app restrito sem criar perfis novos a cada caso
- Uma regra testada em tabela, usada por todos os caminhos (inclusive o futuro MCP)

**Cons:**
- Mais conceitos para o administrador aprender (cargo, área, líder, acesso extra)
- Responsável e reserva não veem as execuções dos outros sem o acesso extra `runs`, o que pode surpreender

## Alternatives Considered
- Perfis fixos (admin, criador, usuário): não dava conta de líderes nem de acessos pontuais
- Permissão por app para tudo: muito trabalho manual para o administrador; o cargo cobre o que vale para a empresa toda
- Abas "Usar" e "Cuidar" na ficha: descartadas depois do protótipo; o desenho B (uma página, bloco técnico recolhido) ficou mais simples
