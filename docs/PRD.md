# PRD — App Hub (Módulo 1: Gestão e orquestração de apps)

**Status:** Em construção · **Nota de estado atual:** 2026-10-26
**Fonte:** `Documentação/Especificação do produto v3.md` (visão completa, Módulos 1 e 2; v2 e v1 guardadas como histórico)

> Este PRD preserva requisitos e contexto do piloto original. O servidor atual
> está no Replit; não depende do WSL para executar. Importação de scripts e
> formulários já têm implementação, mas o fluxo completo em Windows ainda não
> foi validado. Nuvem gerenciada e aplicativo desktop com janela própria são
> evolução prevista, não entrega concluída. Consulte o [README](../README.md),
> [infraestrutura atual](INFRASTRUCTURE.md) e [fluxo de scripts](SCRIPTS.md).

---

## 1. Visão geral

O App Hub é uma plataforma onde pequenas empresas sem TI registram, organizam, rodam e mantêm os apps que criam com IA. Ele transforma "um script na pasta de alguém" em "uma ferramenta da empresa", com dono, documentação, tela de uso e histórico.

**Objetivo do Módulo 1:** qualquer pessoa da empresa encontra, entende e usa qualquer app, sem saber programar e sem depender de quem criou.

**Princípio central: o Hub é genérico.** Ele não é feito sob medida para os apps da Alis. Qualquer app, de qualquer tipo, entra pelo mesmo caminho: é registrado, ganha um **manifesto** (a "carteira de identidade" do app, um arquivo que descreve como ele roda, o que recebe e o que devolve) e, a partir dele, ganha ficha, tela de uso e histórico. Os apps da Alis são o primeiro caso real e servem para testar essa generalidade. Novos apps, criados à mão ou pela IA via MCP (V2), entram pelo mesmo manifesto.

### Piloto original — contexto histórico

- **Onde roda:** no computador do Henry (Windows), localmente, sem nuvem.
- **Apps de teste:** a pasta `C:\Users\HenryKroppAlisInvest\OneDrive - Alis\ALIS\5. Operacional\8. Programação`.
- **IA (Claude API):** fica para uma fase seguinte. O piloto começa com detecção por regras e confirmação humana, e as funções de IA entram quando a chave for configurada.

### Variedade real encontrada na pasta de teste

A pasta confirma que o Hub precisa ser genérico. Hoje ela tem:

| Padrão | Exemplos | Como o app recebe a entrada |
|---|---|---|
| Script com arquivo fixo na pasta | Rentabilidade fundos (`cnpjs.csv`), Carteira detalhada (`Input fundos.xlsx`) | Lê um arquivo com nome fixo ao lado do código |
| Script com janela de seleção | Extrair Seções do FRE, PDF to MD | Abre uma janela do Windows pedindo o arquivo ou a pasta |
| App com tela própria (web) | Sistema Alis (Streamlit), Research Platform, News Scrapper (servidor) | Abre no navegador |
| App de desktop | Análise FIAs, Transcritor de calls | Abre uma janela própria |
| Tarefa recorrente | News Scrapper (agendado no Windows) | Roda sozinho no horário |
| Executável pronto | CVM Downloader, Ferramenta CVM (`.exe`) | Duplo clique |
| Sistema complexo | Sistema-Alis (vários serviços) | Só registro e ficha |
| Documento, não app | Claude Project _ Edu, Expandir Projeção (`.docx`) | Não roda, só fica catalogado |

---

## 2. Usuários

| Perfil | Quem é | O que quer |
|---|---|---|
| **Criador** | Quem faz apps com IA (no piloto, o Henry) | Tirar o app da pasta, deixá-lo usável por todos e saber quando quebra |
| **Usuário** | Colega que não programa | Achar o app certo, entender e clicar em Rodar |
| **Sócio / Administrador** | Quem responde pela empresa | Ver o que existe, o que é usado e onde está o risco |

---

## 3. Funcionalidades (escopo do piloto)

A numeração segue a especificação do produto.

### 3.1 Registro e publicação (1.1)
- **Registro em lote:** o criador aponta uma pasta, o Hub varre as subpastas e sugere um app por subpasta.
- **Registro individual:** o criador aponta uma pasta de um app só.
- **Diagnóstico automático:** para cada app, o Hub identifica linguagem, tipo de app, ponto de entrada (o arquivo que inicia o app), bibliotecas, arquivos de entrada e saída prováveis e chaves/senhas no código.
- **Confirmação do incerto:** o Hub mostra o diagnóstico em português e pergunta só o que ficou em dúvida ("este arquivo é uma entrada?").
- **Manifesto salvo ao lado do app:** o arquivo de configuração fica dentro da pasta do app, legível mesmo fora do Hub.
- **Detecção de segredos:** se achar senha ou chave escrita no código, avisa e oferece mover para o cofre (3.7).
- **Nada é perdido:** o estado original do app é guardado como versão 1 (3.6).
- **Aviso de duplicado:** ao registrar, avisa se já existe app parecido (por nome e descrição), no catálogo ou no mesmo lote.
- **Pastas de arquivo:** pastas com nome de material antigo (ex.: "Old", "backup") vêm marcadas para pular.
- Só publica com responsável definido (3.9).

### 3.2 Ficha do app (1.2)
- Página por app com: nome e resumo, como usar, entradas, saídas, exemplo (última execução bem-sucedida), responsável e reserva, dependências, status e uso, versão.
- **Desenho B (2026-10-05, ADR 009):** uma página só. Primeiro a parte de uso (o que ele pede, o que ele entrega, execuções, quem cuida); no fim, o bloco recolhido "Para quem cuida do app" (pontos de atenção, outros apps, código-fonte, chaves, bibliotecas, histórico, onde está, acesso), que só aparece para quem pode ver a parte técnica. Cada parte tem seu "Editar" em texto cinza discreto; uma parte em edição por vez; se outra pessoa salvou antes, o Hub pede para recarregar. Entradas em lista mostram as opções e "pode marcar mais de uma".
- Campos editáveis à mão. No piloto sem IA, o texto nasce de um modelo preenchido com o diagnóstico; com IA, é escrito em português simples.
- **Exportação:** a ficha também é salva como arquivo de texto na pasta do app.

### 3.3 Execução com interface automática (1.3), a feature central
- **Prioridade (2026-09-30):** deixar o Hub redondo com um app só (Eq. Research Files Prep, um app com janela aberto por `.exe`) antes de registrar mais apps. Para apps com janela, rodar é abrir o app no Windows, esperar ele fechar e mostrar os arquivos novos que ele gerou, para baixar pelo Hub.
- Todo app que pode rodar ganha um botão **Rodar**.
- **Scripts:** o Hub gera um formulário a partir das entradas do manifesto (upload de arquivo, data, número, texto, lista de opções). O usuário preenche e roda.
- **Apps com tela própria (web):** o Hub sobe o app e abre a tela dele.
- **Apps para vários ao mesmo tempo (em aberto, anotado em 2026-10-02):** alguns apps (ex.: Sistema Alis, Research Platform) são usados por várias pessoas juntas e precisam dos mesmos dados. Esses não podem abrir uma cópia no computador de cada um. Ideia a decidir:
  - Opção no manifesto: **pessoal** (abre no computador de quem clicou, como hoje) ou **compartilhado** (um só para todos).
  - O app compartilhado roda no servidor central (ADR 005). Todos abrem o mesmo endereço pelo navegador, e o Hub mantém o app no ar e mostra o link na ficha.
  - Um banco de dados só, ao lado do app, com a senha no cofre (3.7).
  - Aviso no diagnóstico quando um app compartilhado guarda dados em arquivo solto (planilha, CSV): duas pessoas gravando juntas podem perder dados.
  - No piloto, o servidor central é o PC do Henry: só funciona com ele ligado e na rede. De verdade, fica na nuvem (Módulo 2) ou num computador do cliente sempre ligado.
- **Apps de desktop e executáveis:** o Hub abre o app.
- **Agente do Hub (Fase 4, 2026-09-30):** apps com janela abrem pelo Agente, um programa pequeno e invisível instalado no Windows de cada pessoa (ADR 004). A pessoa instala e conecta uma vez em **Configurações › Meu computador**. O app sempre abre no computador de quem clicou em Rodar.
  - **Rodar em poucos cliques (2026-10-01):** o botão Rodar fica também no cartão do catálogo. A pessoa continua na página onde estava e acompanha por uma **faixa de andamento** embaixo da tela ("Abrindo…", "aberto no seu computador", "terminou: N arquivos novos — Ver e baixar", erros com o próximo passo). A faixa vem do Hub: aparece igual depois de recarregar ou em outra aba, até a pessoa dispensar.
  - **Onde salvar:** na primeira vez de cada pessoa em cada app, o Hub pergunta onde salvar os resultados. Não há pasta padrão (decisão do Henry, 2026-10-02): a pessoa escolhe pela janela de pastas do Windows ou digita o caminho. Caminhos longos aparecem por inteiro. A caixinha **Lembrar esta pasta** (marcada) faz as próximas vezes abrirem direto, com um clique. A pasta aparece como "Salva em … · trocar" no cartão do catálogo e na ficha, e "Salva em …" na faixa de andamento.
  - **Parar (2026-10-28):** enquanto um app está aberto, quem o rodou vê **Parar** na faixa e na página da execução. O Hub pede confirmação ("o que não foi salvo se perde") e o Agente fecha o app junto com tudo o que ele abriu. A execução fica como **Parada**, com os arquivos que o app criou até ali. Um pedido que ainda não abriu é só cancelado. Agentes antigos não mostram o botão até se atualizarem.
  - **Vários apps ao mesmo tempo (2026-10-28):** a pessoa pode deixar vários apps abertos pelo Hub no mesmo computador, cada um na sua pasta. Uma pasta já usada por um app aberto (ou uma pasta dentro ou em volta dela) é recusada com o nome do app que está nela e a opção **Trocar pasta**, para que os arquivos novos de um app nunca apareçam na execução do outro. Scripts podem usar a mesma pasta entre si, porque cada execução ganha a sua subpasta. A atualização automática do Agente espera um momento sem nenhum app aberto e nunca impede a pessoa de abrir outro.
  - **Minhas execuções:** item da barra lateral para todos, com o número de apps abertos agora. Lista as execuções da pessoa (o administrador pode ver as de todos), com Ver e baixar e **Rodar de novo** na mesma pasta.
  - **Resultado:** ao fechar a janela, o Hub mostra todo arquivo novo ou alterado, em grupos pelas saídas da ficha, mais "Outros arquivos".
  - **Downloads:** os arquivos ficam na pasta. Ao clicar em Baixar (um arquivo ou o grupo em .zip), o Agente envia na hora. Por isso é preciso que o computador esteja ligado.
  - **Quem vê:** até a Fase 6, só quem rodou e os administradores veem e baixam uma execução.
  - **Baixar escolhendo:** a pessoa marca os arquivos que quer e baixa de uma vez (um arquivo vem direto; vários vêm num .zip).
  - **Atualiza sozinho (desde a 0.2.0):** quando sai uma versão nova, o Agente baixa, confere o arquivo e se troca quando não há app aberto. Se a versão nova não se conectar em 2 minutos, a anterior volta sozinha; depois de 3 falhas na mesma versão, o Hub para de tentar e mostra o botão para baixar. Em Meu computador aparecem "Atualizando…", "Agente atualizado sozinho em…" ou o aviso de falha.
  - **Próximo (ADR 005):** o App Hub, com ícone perto do relógio, janela própria e instalador único.
- **Ambiente preparado sozinho:** na primeira vez, o Hub instala a versão certa da linguagem e das bibliotecas. Nas seguintes, reaproveita o que já instalou. O usuário não vê terminal.
- **Progresso e resultado:** mostra o andamento e, ao final, o texto impresso, as tabelas e os arquivos gerados para baixar.
- **Registro de cada execução:** quem rodou, quando, com quais entradas, duração, resultado e versão usada.
- **Repetir execução:** rodar de novo com as mesmas entradas.
- Apps que não rodam pelo Hub (documentos, planilhas com macro, sistemas complexos) aparecem só com ficha.

### 3.4 Catálogo e descoberta (1.4)
- Tela inicial com os apps em cartões: nome, resumo, responsável, status e último uso. Apps que rodam pelo Hub têm o botão **Rodar** no próprio cartão.
- Busca simples por nome, resumo e etiquetas.
- Filtro por área (ex.: Investimentos, Operações, RI) e por status.
- Favoritos por pessoa.

### 3.5 Padrões (1.5)
- Formato padrão de manifesto e de estrutura de pasta, documentado e aplicado a todos os apps.
- Na ficha, o Hub aponta o que foge do padrão (ex.: "a chave está escrita no código", "a entrada depende de uma janela de seleção").

### 3.6 Versões e histórico (1.6)
Pedido do Henry em 2026-09-30: o histórico de um app mostra três coisas, numa linha do tempo só: **mudanças no código**, **execuções** e **mudanças na ficha**.
- Toda alteração publicada gera uma versão, com quem mudou, quando e um resumo.
- Toda mudança na ficha (nome, resumo, responsável, entradas…) fica registrada com quem mudou e quando.
- O Hub detecta quando o código da pasta mudou e registra a nova versão sozinho.
- Botão para voltar a uma versão anterior. A restauração também vira uma versão.
- Histórico de execuções ligado à versão usada.
- **Construído (2026-10-01, ADR 006):** a versão nasce sozinha quando o Hub confere a pasta, sem perguntar. Descrever a mudança é opcional. Só o código-fonte tem cópia; do `.exe` fica só a impressão digital, e voltar a versão pede recompilar. A ficha só é registrada, não se restaura. Chaves no código aparecem mascaradas.

### 3.7 Equipe e permissões (1.7, construído na Etapa 2, ADR 009)
Em linguagem de produto, quem pode o quê:
- **Cada pessoa tem um cargo**, personalizável. Prontos: Administrador (tudo), Criador (registrar apps) e Usuário (só usar). Um cargo liga ou desliga: registrar apps, editar qualquer ficha, voltar versões de qualquer app, ver a parte técnica de qualquer app, ver as execuções de todos, ver todos os apps (inclusive restritos), cadastrar pessoas, cuidar das áreas e criar/mudar cargos.
- **Áreas** têm líderes e membros. Todo app pertence a uma área. O líder cuida dos apps da sua área: edita, volta versões, vê o técnico e as execuções de todos, e dá acessos extras só neles.
- **Acessos extras** a uma pessoa, app por app ou "todos os apps da área": editar a ficha, voltar versões, ver a parte técnica, ver execuções de todos.
- **App aberto ou restrito.** Aberto: todos veem e rodam. Restrito: só as áreas e pessoas escolhidas, quem cuida dele (responsável, reserva, líder da área), quem cuida das áreas, quem recebeu um acesso extra nele (no app ou na área toda) e o cargo "ver todos os apps". Quem não tem acesso nem enxerga o app (a página responde "App não encontrado"). Todo app novo começa aberto.
- **As permissões de cargo da empresa toda** (editar qualquer ficha, voltar versões, ver o técnico, ver as execuções de todos) valem só nos apps que a pessoa enxerga. Para chegar aos restritos, o cargo precisa também de "ver todos os apps".
- **Execuções:** cada um vê as suas. As de todos: quem tem o cargo com essa permissão, o líder da área e quem recebeu o acesso extra. Responsável e reserva não veem as dos outros sem esse acesso. Quem vê as execuções dos outros também baixa os arquivos delas. Exceção: quem deixou de ver um app (ele foi restrito depois) continua vendo e baixando as próprias execuções dele, que são arquivos seus.
- **Quem pode editar um app** muda a ficha, mas **não** o responsável, a reserva nem a área. Isso fica com quem cuida do acesso do app (responsável, reserva, líder da área ou quem cuida das áreas); quem só edita pede a eles.
- **Ninguém dá um cargo com mais permissões do que o próprio**, nem muda o cargo de quem tem mais permissões do que si (só alguém com cargo igual ou maior muda o cargo de um administrador).
- **Sempre há ao menos uma pessoa** que cuida das pessoas e dos cargos; o Hub recusa a mudança que deixaria a empresa sem ninguém assim.
- **Tela Equipe** (barra lateral, para quem cuida de pessoas, áreas ou cargos, ou lidera uma área), com três abas: Pessoas (só para quem pode cadastrar pessoas), Áreas e Cargos.
- **Cofre local:** chaves e senhas guardadas criptografadas. O app recebe a chave na hora de rodar e o usuário nunca a vê. Registro de qual app usou qual chave e quando. (Pendente, Fase 6.)
- **Empresa e times (combinado em 2026-09-30, para depois):** a empresa é a conta (nome, padrões, cofre, chave da IA); no piloto existe uma só (Alis). A Etapa 2 entregou áreas, líderes, apps restritos e acessos por app; ainda ficam para depois a pessoa em várias empresas, configurações próprias por time, limites e aprovação antes de publicar.

### 3.7b Apps compartilhados (construído em 2026-10, ADR 010)
Alguns apps precisam ficar no ar para muita gente usar ao mesmo tempo, com os mesmos dados. O Hub passa a rodá-los num servidor, em vez de abrir no computador de cada pessoa.
- **Tornar compartilhado:** na ficha, a parte "Compartilhado" mostra a checagem do que falta (com um "texto para o Claude" para consertar), deixa **Testar** numa caixa separada, com banco próprio de teste (desliga sozinha em 30 min), e **Publicar** para a equipe. **Publicar versão nova** sobe ao lado da atual e só assume se funcionar; **Voltar para a anterior** volta só a versão ou a versão com os dados.
- **Abrir:** o cartão do catálogo ganha **Abrir** e o status (no ar, dormindo, parado, com problema). O app abre em endereço próprio, sem outro login: vale o login do Hub e as mesmas regras de acesso da ficha. O app sabe quem é a pessoa (nome, e-mail e papel).
- **Descanso e cópias:** o app dorme depois de 30 min sem uso e acorda no próximo acesso. Há cópia diária do banco, cópias antes de publicar e restaurar e cópia manual, todas criptografadas; a ficha restaura.
- **Servidor e gastos:** o administrador informa quanto o servidor custa por mês e vê memória, disco e a parte de cada app no gasto, e pode parar, ligar ou mudar os limites de um app.
- **O que cada papel vê:**
  - **Quem só usa:** vê o status e **Abrir** nos apps que pode ver. Nada de testar, publicar ou servidor.
  - **Quem cuida do app** (pode restaurar): torna compartilhado, testa, publica, volta versão, muda configurações e restaura cópias.
  - **Quem vê a parte técnica:** vê mensagens do app e o uso.
  - **Quem tem "Cuidar do servidor"** (administrador, por padrão): vê a página Servidor, age sobre qualquer app compartilhado e informa o custo.
- **Limites:** exige Podman ligado no servidor; geração de PDF do Sistema Alis não funciona dentro da caixa (a checagem avisa). Subir na nuvem fica para depois.

### 3.8 Saúde (1.8)
- Toda execução com erro fica registrada com a mensagem de erro.
- Status de cada app: funcionando, com erro ou sem uso.
- **Com IA (fase seguinte):** explicação do erro em português.

### 3.9 Continuidade e responsáveis (1.9)
- Todo app tem responsável e reserva.
- Lista de **apps em risco**: sem reserva, com dono único, sem uso há muito tempo ou com erro recente.

### 3.10 Painel de governança (1.10)
- Uma tela com inventário (apps por área e tipo), uso (mais usados, execuções por semana) e apps em risco.

### 3.11 Registro pelo Claude via MCP (1.11, antecipado)
Pedido do Henry em 2026-09-30: registrar apps de dentro do Claude, sem preencher nada à mão.
- O criador conecta o Claude que já usa ao Hub, com um **código de acesso pessoal** gerado na tela Pessoas.
- O criador pede, por exemplo, "registra todos os apps da pasta de programação". O Claude:
  1. lista as pastas e o que já está no catálogo;
  2. lê o código de cada app, com **chaves e senhas mascaradas** pelo Hub (o Claude nunca vê os valores);
  3. parte do diagnóstico por regras e completa com o que entendeu do código: resumo em português, entradas, saídas, arquivo de início, área e pontos de atenção;
  4. registra o app com responsável e reserva, usando o formato padrão do manifesto.
- **Confirmação humana:** cada registro passa pela confirmação do próprio Claude ("permitir registrar?"), e o app fica marcado no Hub como **registrado via IA**.
- A IA age com as permissões de quem gerou o código de acesso, nunca mais que isso.
- **Não precisa de chave da Claude API:** quem lê e escreve é o Claude do criador.

### 3.12 Skills do Claude no catálogo (ideia, não decidida)
Ideia do Henry em 2026-10-26: compartilhar skills do Claude pelo Hub, como se compartilham apps.
- **O que é uma skill:** uma pasta com um `SKILL.md` (nome e descrição no topo, depois as instruções), às vezes com scripts, modelos ou textos de referência. Não há registro central: o Claude usa as skills que estão no lugar onde ele procura.
- **No Hub:** mais um tipo no catálogo, ao lado de script e janela, com responsável, ficha, versões, área e permissões. Mais simples que um app: não roda, não prepara ambiente, não gera resultados. Checagem de segredos no texto, como na importação de scripts.
- **Entrega (o ponto difícil):** pôr a pasta onde o Claude de quem tem acesso procura, e tirar de quem perdeu o acesso. Caminhos possíveis:
  1. o Agente copia a pasta para `.claude\skills` no Windows (Claude Code), com atualização automática;
  2. o Hub serve uma loja de plugins do Claude Code, filtrada pelas permissões de cada pessoa;
  3. claude.ai e Claude Desktop: falta pesquisar se dá para publicar skills da empresa por um sistema de fora ou só por um administrador.
- **Junto com o MCP (3.11):** a skill ensina o Claude a fazer a tarefa e o Hub roda o app de dentro da conversa.
- **Por que interessa:** empresas vão acumular dezenas de skills sem dono nem versão, o mesmo problema que o Hub resolve para apps.

### 3.13 Gestão de gastos com IA (ideia, não decidida)
Ideia do Henry em 2026-10-26: gerir tokens e créditos de IA pelo Hub. Três níveis, do mais simples ao mais completo:
1. **Painel de gastos:** o Hub lê o consumo da conta da empresa nos provedores de IA (a Anthropic tem uma API de uso e custo para administradores; falta confirmar o detalhe disponível, sobretudo para os planos do claude.ai) e mostra o gasto por mês, chave e modelo.
2. **Chave de IA centralizada (o nível que mais vale):** os apps não guardam chave. Pedem a IA através do Hub, que usa a chave da empresa guardada no cofre. O Hub mede o gasto por app, pessoa e área, e acaba com chave escondida no código (risco já visto na importação de scripts).
3. **Créditos e limites:** orçamento mensal por área ou pessoa, aviso em 80%, bloqueio ou aprovação ao estourar, relatório de consumo. Para vender o Hub, também serve de base para cobrança.
- **Ligações:** cofre (1.7), página Servidor e gastos e porteiro dos apps compartilhados (3.7b), skills do Claude (3.12).

---

## 4. Fluxos principais

**A. Registrar os apps existentes (criador)**
1. Abre o Hub e, no catálogo, clica em "+ Registrar apps".
2. Escolhe a pasta de programação.
3. O Hub mostra a lista de apps encontrados, cada um com o diagnóstico.
4. Para cada app, confirma o que ficou incerto, define área, responsável e reserva.
5. Publica. Os apps aparecem no catálogo.

**B. Usar o app de um colega (usuário)**
1. Abre o Hub, busca "rentabilidade" e encontra o app.
2. Lê a ficha e clica em Rodar.
3. Sobe o arquivo de CNPJs no formulário e clica em Rodar.
4. Acompanha o progresso e baixa a planilha gerada.

**C. Atualizar um app (criador)**
1. Altera o código na pasta, com a IA que já usa.
2. O Hub percebe a mudança e pede para registrar a nova versão, com um resumo.
3. Se algo der errado, volta para a versão anterior com um clique.

**E. Registrar pelo Claude (criador)**
1. Na tela Pessoas, gera o código de acesso e conecta o Claude (uma vez só).
2. No Claude, pede: "registra os apps da pasta de programação".
3. O Claude lê cada app, mostra o que entendeu e registra, pedindo confirmação a cada um.
4. Os apps aparecem no catálogo com a marca "via IA", e o criador pode revisar no Hub.

**D. Ver a saúde da empresa (sócio)**
1. Abre o painel.
2. Vê quantos apps existem, quais são usados, quais estão com erro e quais estão em risco.

---

## 5. Requisitos não funcionais

- **Local:** tudo roda no computador do Henry, sem depender de internet (exceto quando o próprio app precisa).
- **Não destrutivo:** o Hub nunca apaga nem altera o código de um app sem confirmação. Ele só adiciona o manifesto e a ficha à pasta.
- **Execução isolada da pasta original:** rodar um app não pode sujar nem sobrescrever a pasta original. Arquivos gerados ficam guardados no histórico da execução.
- **Português simples** em toda a interface, sem termos técnicos sem explicação.
- **Manifesto aberto e legível:** o app continua compreensível fora do Hub.
- **Segredos nunca visíveis** na interface, nos registros ou no manifesto. Arquivos de senhas (`.env`) não entram no histórico de versões, e as cópias dos bancos ficam criptografadas.
- **Preparado para o Módulo 2:** a forma de rodar os apps fica separada do resto, para depois levar a execução para a nuvem sem refazer o Hub.
- **Operável pela IA (MCP):** tudo o que a interface faz (registrar, ler ficha, rodar, versionar) fica disponível como operação que a IA também pode usar (3.11).

---

## 6. Fora do escopo do piloto

- **Módulo 2 inteiro** (nuvem, acesso remoto, cobrança).
- **MCP além do registro:** criar apps do zero, rodar, ler saúde e usar o cofre pelo MCP ficam para depois das Fases 4–7. O registro pelo MCP (3.11) entra já.
- **Funções de IA com chave própria do Hub:** diagnóstico por IA dentro do Hub, ficha escrita por IA dentro do Hub, explicação de erros, "pergunte ao app", busca em linguagem natural e resumo das mudanças. Ficam para quando a chave da API estiver configurada.
- **Correção automática de apps (1.8):** V2.
- Login com Google ou Microsoft, grupos de acesso, chaves pessoais por usuário.
- Encadeamento de apps, presets salvos e agendamento pelo Hub.
- Desenvolvimento compartilhado com resolução de conflitos.
- Trilha de auditoria exportável, resumo semanal por e-mail, tempo economizado.
- Execução de planilhas com macro e de sistemas complexos (só registro e ficha).
