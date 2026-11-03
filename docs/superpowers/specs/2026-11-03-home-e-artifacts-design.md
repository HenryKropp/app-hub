# Home e Artifacts — desenho

**Data:** 2026-11-03 · **Status:** aprovado em conversa pelo Henry; aguardando revisão deste documento

Duas abas novas, independentes, construídas nesta ordem: **Artifacts** primeiro, **Home** depois (a Home mostra artifacts nos favoritos e nas novidades do time).

## 1. Por que

- **Artifacts:** times que usam muito o Claude (caso real: a empresa do irmão do Henry) acumulam dashboards e páginas interativas espalhados em links e conversas. O Ziv vira o lugar organizado deles, com as mesmas regras de dono, área e acesso que já valem para apps.
- **Home:** hoje quem entra no Ziv cai direto no catálogo. A Home mostra o "meu dia" da pessoa e, para quem lidera, como está o time.

**Sucesso:**
- Artifacts: subir um HTML do Claude, achá-lo pela busca ou por etiqueta, abri-lo funcionando (gráficos e cliques), subir uma versão nova e ver a anterior no histórico; um artifact restrito some para quem não tem acesso.
- Home: um usuário comum vê favoritos, recentes, últimas execuções e avisos só dele; um líder de área vê o uso e as novidades só da sua área; o administrador vê a empresa toda e as pendências de gestão.

## 2. Decisões do Henry (2026-11-03)

| Tema | Decisão |
|---|---|
| Que artifacts | Dashboards e páginas interativas feitos no Claude |
| Como entram | Colando um **link** ou subindo o **arquivo**. Depois, uma terceira porta pelo **MCP do Ziv** (fora deste desenho, mas o modelo de dados não pode impedir) |
| Como se acha | Cartões com nome, descrição e **etiquetas livres**, mais **busca** |
| Acesso | **Mesmas regras dos apps**: responsável, área, aberto ou restrito, acessos extras |
| Atualizar | A versão nova vira a atual; **as antigas ficam no histórico** |
| Home | "**Meu dia**" para todos; parte "**Meu time**" para quem lidera |
| Favoritos | **Os dois**: estrela fixada pela pessoa + usados recentemente |
| Avisos na Home | Minhas execuções com erro ou esperando algo de mim; meu computador; problemas no que eu cuido |
| Meu time | Quem está usando; o que estão criando; pendências de gestão |
| Aprovação | Artifacts **não passam por aprovação**: quem pode registrar sobe e aparece na hora. E para **apps também**: "exigir aprovação" passa a **começar desligado**; o administrador liga no Admin se quiser |

## 3. Escolhas do cofundador, aprovadas no resumo

- Link abre numa **aba nova**; quem abre precisa ter acesso ao link lá fora.
- Arquivo enviado é mostrado **pelo próprio Ziv, numa caixa isolada** (ver 4.4).
- Um artifact de arquivo é **um único `.html`** (o formato que o Claude exporta).
- Artifacts **não rodam no computador de ninguém**: o Agente não participa.
- A **Home vira a primeira página** (`/`). O catálogo passa a ser "Apps" na barra lateral, ao lado de "Artifacts".
- "Quem lidera": **administrador vê a empresa toda; líder de área vê só a sua área**. Quem não lidera nada não vê "Meu time".
- **Pendências de gestão** só para quem pode cadastrar pessoas.
- Partes sem conteúdo (ex.: nenhum aviso) **somem** em vez de mostrar caixa vazia.

## 4. Artifacts

### 4.1 O que é um artifact

- Nome, descrição, etiquetas (texto livre, várias), responsável, reserva opcional, área, aberto ou restrito.
- **Tipo:** `link` (endereço `https://`) ou `arquivo` (HTML guardado no Ziv).
- **Origem:** `manual` agora; `mcp` depois (mesmo campo "registrado via IA" dos apps).
- **Versões:** cada envio de arquivo ou troca de link é uma versão numerada, com quem enviou, quando e uma nota opcional ("o que mudou"). A atual é a mais nova, salvo se alguém **voltar** para uma anterior (voltar cria uma versão nova igual à escolhida, como nos apps).

### 4.2 Telas

- **Barra lateral:** "Artifacts" logo abaixo de "Apps".
- **Lista (`/artifacts`):** cartões com nome, descrição curta, etiquetas, área, tipo (ícone de link ou de arquivo) e data da última versão. Busca por nome, descrição e etiqueta; filtro clicando numa etiqueta e por área. Botão **Novo artifact** para quem pode registrar apps.
- **Novo artifact:** escolher "Colar link" ou "Subir arquivo"; nome, descrição, etiquetas (sugere as já usadas), área, aberto/restrito. O responsável é quem cria.
- **Página do artifact (`/artifacts/:id`):** para tipo arquivo, a página mostra o artifact grande na caixa isolada, com botão **Abrir em tela cheia** e **Baixar**. Para tipo link, cartão com **Abrir** (aba nova). Abaixo: descrição, etiquetas, responsável, **Enviar versão nova**, histórico de versões (abrir qualquer uma, voltar), estrela de favorito, editar e apagar.
- Mobile: lista em uma coluna; a página mostra o artifact com altura da tela.

### 4.3 Acesso

Reusa exatamente as regras dos apps (PRD 3.7): quem vê, quem edita, quem volta versões, quem muda responsável/área/restrição, acessos extras por item e "todos da área". Quem não tem acesso recebe "Artifact não encontrado". Registrar artifact = mesma permissão de registrar app. O cargo "ver todos os apps" também vale para artifacts.

Escolhas do cofundador (depois da revisão):
- **Permissões que já existem cobrem os artifacts.** Cargo "editar qualquer ficha" e "voltar versões de qualquer app", e o acesso extra "todos os apps da área", valem também para os artifacts da área. O acesso extra num artifact só tem "editar" e "voltar versões" ("ver parte técnica" e "ver execuções" não se aplicam). As telas de acesso ganham o artifact como mais um item, sem cargo novo.
- **Sem aprovação** (decisão do Henry): nenhuma tabela de aprovação para artifacts, e eles não entram na lista "Esperando aprovação" do Admin. O administrador ainda pode **apagar** qualquer artifact.
- **Segredos seguem a chave da empresa** "bloquear segredos embutidos", como nos apps: se ligada, bloqueia; se desligada, só avisa.

### 4.4 Segurança do HTML enviado

HTML enviado por qualquer pessoa é código de terceiros. Requisitos:
- Nunca servido como página do Ziv: o conteúdo vai com cabeçalho `Content-Security-Policy: sandbox allow-scripts allow-popups allow-forms allow-downloads` (sem `allow-same-origin`), e mostrado num `iframe` com o mesmo `sandbox`. Assim ele roda numa origem opaca: não lê o cookie do Ziv, não chama a API com a sessão da pessoa e não mexe na página em volta, mesmo se aberto direto pelo endereço.
- O endereço do conteúdo confere o acesso da pessoa como qualquer outra rota.
- Scripts de CDN (o Claude usa) continuam funcionando.
- Limite de tamanho: 16 MB por arquivo (o mesmo teto dos artifacts do Claude). Só `.html`/`.htm`, conferido no Ziv, não só na janela.
- Checagem de segredos no texto (a mesma da importação de scripts), seguindo a chave da empresa (ver 4.3).
- Link: só `https://` (ou `http://localhost` em desenvolvimento).

### 4.5 Fora deste desenho
- Publicar pelo MCP.
- Artifacts com vários arquivos, dados que ficam salvos entre visitas, ou ligados a dados vivos.
- Comentários.

## 5. Home

### 5.1 Para todos — "Meu dia"

Na ordem da página:
1. **Atenção** (some se vazio): minhas execuções com erro ou paradas esperando o computador há mais de 5 minutos (nos últimos 7 dias, não dispensadas; escolha do cofundador, porque hoje não existe execução esperando uma resposta da pessoa); meu computador desconectado ou com Agente desatualizado; para quem cuida de apps (responsável, reserva, líder da área): app compartilhado com problema, app com ponto de atenção, versão nova esperando aprovação (só se a empresa ligou a aprovação). Cada aviso leva à página certa.
2. **Favoritos:** apps e artifacts com estrela, na ordem em que foram marcados. A estrela fica nos cartões e nas páginas de app e de artifact. Sem favoritos: uma linha curta explicando a estrela.
3. **Usados recentemente:** até 6 apps e artifacts que a pessoa rodou ou abriu nos últimos 30 dias, mais recentes primeiro, sem repetir os favoritos.
4. **Minhas últimas execuções:** as 5 mais recentes, com link para "Minhas execuções".

Abrir um artifact conta como "usado" (registrar a abertura); rodar um app já conta pelas execuções.

### 5.2 Para quem lidera — "Meu time"

Escopo: administrador (cargo com `manage_roles`) = empresa toda; líder de área = pessoas e itens das áreas que lidera; líder de várias áreas vê a soma.
- **Quem está usando:** por pessoa, execuções e artifacts abertos nos últimos 7 dias; lista separada de quem nunca usou (nenhuma execução nem abertura).
- **O que estão criando:** apps e artifacts criados ou com versão nova nos últimos 14 dias, por quem.
- **Pendências de gestão** (só quem pode cadastrar pessoas): convites não aceitos, pessoas sem área, apps e artifacts sem responsável ativo.

A aba Admin (ADR 017) continua sendo a visão completa da empresa; "Meu time" é o resumo do dia e vale também para líderes de área.

### 5.3 Navegação
- `/` = Home. Catálogo vai para `/apps`, com redirecionamento dos endereços antigos que apontem para o catálogo.
- Barra lateral: Home, Apps, Artifacts, Minhas execuções, e o resto como hoje.

## 6. Aprovação de apps desligada por padrão

- Empresa nova começa com "exigir aprovação" **desligado**.
- Empresas que já existem (Alis, Teste) **mantêm a chave como está** (escolha do cofundador: não mudar sozinho uma regra de segurança de quem já usa); o administrador desliga no Admin.
- A chave continua no Admin e, quando ligada, funciona como hoje.

## 7. O que fica para quem constrói

Modelo de tabelas, rotas da API, como guardar o HTML (banco ou `BLOB_STORAGE_PATH`), como reaproveitar o código de permissões dos apps, formato das etiquetas, visual dos cartões (seguindo o estilo do Ziv, ADR 008), e como contar "aberturas". Tudo por empresa (dentro do schema de cada empresa, como apps).
