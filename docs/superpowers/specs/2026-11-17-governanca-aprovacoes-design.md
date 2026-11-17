# Governança: quem aprova, quem usa, por onde sobe — desenho

Data: 2026-11-17. Decidido com o Henry nesta conversa.

## Por quê

O Henry pôs o Projeto 2 dos links externos em espera e pediu para, antes, melhorar "acessos, segurança, dados", pensando no **fluxo dentro de uma empresa, em termos de governança**: alguém faz uma automação, e outros aprovam. Segurança é a proposta de valor do Ziv: o dono da empresa precisa poder dizer quem aprovou cada coisa que roda, quem pode usá-la e por onde ela entrou.

Hoje (ADR 009, 010, 017, 018):
- As regras de quem vê o quê são completas (cargo com 11 permissões, áreas com líderes e membros, app/artifact aberto ou restrito, acessos extras por pessoa), mas **todo app novo nasce aberto** para a empresa toda e quem decide o público é quem cuida do app.
- A aprovação é uma chave da empresa, **"Exigir aprovação"** (desligada em empresa nova; ligada na Empresa Piloto), com **uma** aprovação só, de quem administra, guardada como "aprovado sim/não" em `app_security` (sem histórico por versão). Toda versão nova precisa de nova aprovação; enquanto isso o app **não roda**.
- Artifacts não passam por aprovação.
- Código entra por três caminhos: Claude pelo MCP do Ziv, upload de pasta pelo site, e pasta registrada no PC pelo Agente.

A palavra do Henry é **flexível**: cada empresa, e cada app importante, decide o quanto aperta.

## Objetivos

1. **O líder da área sempre aprova.** Todo app e todo artifact novo passa pelo líder da área dele. Não é opcional.
2. **Ao aprovar, o líder escolhe quem usa:** só a área, a área mais outras áreas ou pessoas, ou a empresa toda. Mudar o público depois também é só do líder (ou de quem gerencia áreas), e fica registrado.
3. **TI aprova se a empresa ligar.** "TI" é uma permissão de cargo nova, "Aprovar pela TI"; a empresa dá ao cargo que quiser. Ligada, todo pedido precisa do líder **e** de alguém com essa permissão. A TI olha segurança (sites que acessa, senhas do Cofre, o que manda para fora); o líder olha se faz sentido para o negócio e quem usa.
4. **Quando reaprovar uma versão nova**, escolha da empresa (padrão da empresa):
   - **Toda versão** espera aprovação;
   - **Só quando muda o risco** (ver "Risco" abaixo): sem mudança de risco, a versão vai ao ar sozinha e os aprovadores recebem um aviso;
   - **Só a primeira vez:** versões seguintes vão ao ar com aviso.
5. **Cada app ou artifact pode ser mais rígido que o padrão** (ex.: o app de carteira de clientes fica em "toda versão" numa empresa cujo padrão é "só risco"). Quem cuida do app ou o líder pode apertar; **afrouxar abaixo do padrão da empresa, só o administrador**.
6. **Por onde o código pode subir**, escolha da empresa: todos os caminhos (como hoje), ou **só pelo Claude** (MCP do Ziv), recusando upload de pasta pelo site e pasta registrada pelo Agente. Junto, a empresa pode limitar **quem tem chave de MCP**, e cada chave fica registrada (quem, quando, de qual computador).
7. **Artifacts no mesmo fluxo:** líder aprova e escolhe quem vê; TI se ligada; mesma regra de versões. Um artifact só pode virar link externo depois de aprovado.
8. **Página "Aprovações"** com o que está esperando a pessoa, e bloco **"Esperando sua aprovação"** no Início. Cada pedido mostra o que mudou (app novo; versão nova com o que mudou de risco; público pedido), quem pediu, e os botões Aprovar / Recusar (com motivo).
9. **Registro de tudo:** cada pedido guarda quem pediu, cada etapa guarda quem aprovou ou recusou, quando e o motivo. O Admin mostra esse histórico por app/artifact e da empresa toda ("quem aprovou isso?").
10. **O criador fica sabendo:** na página do app ("Aguardando o líder de Research" / "Aguardando a TI" / "Recusado por Fulano: motivo") e pelo Claude (as ferramentas do MCP respondem com a situação do pedido).

## Risco (para a regra "só quando muda o risco")

Proposta minha: o "risco" de uma versão é o conjunto de coisas que o Ziv já sabe ou consegue ler do código e da ficha:
- as chaves do Cofre que o app usa;
- os sites (domínios) com que o código fala, lidos do código;
- se usa navegador (receita do ADR 040);
- se é app 24h no servidor (compartilhado) ou roda no PC;
- se manda coisa para fora da empresa (ex.: e-mail, upload para site externo), quando detectável.

Muda o risco quando aparece algo **novo** nessa lista (algo que sai não conta). Para artifact, o risco muda quando ele tem link externo ativo (a versão nova vai a gente de fora).

## O que acontece enquanto uma versão espera

Proposta minha: **a versão anterior aprovada continua no ar** e rodando; a nova fica esperando (encaixa no rascunho do ADR 043, que já guarda versão sem mexer no que está no ar). Isso muda o comportamento de hoje, em que a versão nova trava o app até ser aprovada. Num link externo "sempre a mais recente", o cliente de fora continua vendo a versão aprovada anterior.

Exceção: app novo (sem versão aprovada) não roda para ninguém além de quem cria e dos aprovadores, que podem testar antes de aprovar.

## Casos de canto (propostas minhas)

- **Quem cria é o próprio líder:** a aprovação dele vale; a TI continua, se ligada.
- **Quem cria tem a permissão de TI:** a etapa de TI vale como dele; o líder continua.
- **Área sem líder:** quem administra a empresa aprova no lugar do líder.
- **Empresa de uma pessoa só:** o admin é líder e TI; aprova o que cria com um clique.
- **Dois líderes na área:** basta um.
- **Recusado:** o pedido fecha com motivo; a versão anterior segue no ar; uma versão nova abre outro pedido.
- **Versão nova chega com um pedido aberto:** o pedido antigo é substituído (como o rascunho mais novo cancela o anterior, ADR 043).
- **Apps e artifacts que já existem:** continuam no ar como estão, com o público de hoje; as regras novas valem a partir da próxima versão. O "aprovado sim/não" atual vira, na migração, um pedido aprovado registrado como "aprovado antes da governança".
- **"Exigir aprovação" de hoje** deixa de existir como chave: a aprovação do líder é sempre ligada. Na Empresa Piloto, que já exige, o efeito é parecido; em empresa nova, passa a existir aprovação do líder.
- **Suspender** um app (ADR 017) continua como está, com quem administra.
- **"Testar e publicar sem o Henry" (ADR 043/044):** a versão que passa no teste vai ao ar sozinha **só se a regra de versões deixar**; senão, passa a "testada, esperando aprovação", e o pedido mostra que o teste passou.

## Respostas à revisão do construtor (minhas decisões, podem ser corrigidas)

- **Mudança só na ficha, sem código novo.** Conta como versão nova para a regra quando mexe em item de risco (chaves do Cofre, 24h no servidor ou PC, navegador). Mudança de texto (resumo, como usar, entradas) não pede aprovação. A ficha pendente espera junto com a versão, no rascunho (ADR 043 já guarda mudanças de ficha), e a ficha aprovada continua valendo enquanto isso.
- **Mudar o app ou artifact de área** abre um pedido para o líder da área nova. Até ele aprovar, o item continua na área antiga, com o público de antes.
- **Quem vê antes da aprovação:** só quem cria, os aprovadores (líder, TI) e quem tem `see_all_apps` (auditoria). Nenhum acesso extra revela um item não aprovado.
- **Quem decide quem usa, depois da aprovação:** só o líder (ou quem tem `manage_areas`). Responsável e reserva **perdem** o poder de abrir/restringir e escolher pessoas; continuam editando e voltando versão. Dar acesso extra (editar, técnico, execuções) continua sendo do líder, como hoje (ADR 009), e passa a ficar registrado.
- **"Administrador"** = quem tem `manage_roles` (o mesmo da aba Admin). Ele aprova no lugar do líder **só quando a área não tem líder**; não passa por cima de um líder que existe. É também o único que afrouxa um app abaixo do padrão da empresa.
- **"Só pelo Claude" ligado com apps que já sobem por pasta ou site:** o app continua no ar na versão aprovada; a próxima versão por pasta ou upload é recusada com aviso "esta empresa só aceita versões pelo Claude". O teste no PC do ADR 044 continua permitido, porque é o próprio MCP que o comanda.

## Restrições

- A regra de acesso continua num lugar só (`backend/internal/access`); a tela só lê o `can` pronto.
- Responder 404 para quem não pode ver continua valendo para apps restritos.
- O Ziv **não consegue provar qual conta de Claude** está do outro lado do MCP; ele garante o caminho (só MCP) e quem tem chave. Dito ao Henry nesta conversa.
- Sem e-mail por enquanto (sem domínio, ADR 045): o aviso aos aprovadores é dentro do Ziv. E-mail entra quando houver domínio.
- Isolamento por empresa como no resto do Ziv.

## Não objetivos

- Motor de regras genérico (cadeias de aprovação montadas pela empresa).
- Aprovação por execução (cada vez que alguém roda).
- Provar a conta de Claude usada.
- Aviso por e-mail ou celular (fica para depois do domínio).
- Projeto 2 dos links externos (em espera).

## Anti-objetivos (falharia mesmo funcionando)

- Um app ou artifact rodando ou visível para alguém sem a aprovação que a regra pede.
- Não dar para responder "quem aprovou esta versão, e quando?".
- Um app aprovado parar de funcionar porque saiu uma versão nova que ainda espera aprovação.
- Uma empresa pequena (sem TI, uma ou duas pessoas) travada pela governança.
- Alguém afrouxar sozinho a regra de um app abaixo do padrão da empresa.

## Abordagens consideradas

1. **Pedidos de aprovação com etapas** — escolhida pelo Henry. Cada app/artifact novo ou versão nova (que a regra mande aprovar) gera um pedido com as etapas exigidas (líder; TI se ligada); cada etapa guarda quem, quando e motivo. Substitui o "aprovado sim/não" e dá o histórico.
2. **Remendar o que existe** (marca "TI aprovou" ao lado do "aprovado"): mais rápido, sem histórico por versão; cada regra nova vira outro remendo. Descartada.
3. **Motor de regras genérico:** flexível ao extremo, complexo de usar e de construir para o que foi pedido. Descartada.

**Construção:** tudo de uma vez, antes de ir ao ar (escolha do Henry), não em etapas.

## Decisões do Henry

- Projeto 2 dos links externos em espera; antes, governança.
- A empresa escolhe quem aprova além do líder (TI ou não).
- O líder da área sempre aprova e, ao aprovar, escolhe quem usa.
- Regra de versões flexível: toda versão / só quando muda o risco / só a primeira; um app importante pode exigir tudo.
- Opção "só subir pelo Claude", contra código de fora e contra ferramenta fora da empresa; flexível.
- Artifacts no mesmo fluxo.
- Caminho 1 (pedidos com etapas), construído inteiro de uma vez.
- Configurações iniciais: **empresa nova** = só líder, TI desligada, "só quando muda o risco", todos os caminhos. **Empresa Piloto** no dia que for ao ar = líder + TI ligada (o cargo Administrador ganha "Aprovar pela TI") e "toda versão", para não afrouxar o que ela já tem; todos os caminhos.

## Fica para quem constrói

- Formato das tabelas, rotas e telas, seguindo o padrão do Ziv; texto exato em português, no tom do Ziv.
- Como ler os domínios do código (scanner) e montar a "impressão" de risco de cada versão.
- Como o pedido convive com `app_drafts` (ADR 043) e com versões que chegam direto (upload, pasta do PC).
- Migração do `app_security` atual para pedidos.
