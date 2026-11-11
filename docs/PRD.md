# PRD — App Hub (Módulo 1: Gestão e orquestração de apps)

**Status:** Em construção · **Nota de estado atual:** 2026-10-26
**Fonte:** `docs/especificacao/Especificação do produto v3.md` (visão completa, Módulos 1 e 2; v2 e v1 guardadas como histórico)

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
- **Usar · Cuidar (2026-11-09, ADR 029):** o topo traz nome, resumo e o botão de Rodar/Abrir. Embaixo, quem só usa vê a face **Usar** (o que pede, o que entrega, execuções, agendamentos). Quem cuida do app abre na face **Cuidar**: um menu com Pendências (linha de etapas registrado → aprovado → publicado → com reserva e a lista do que falta, cada item com o botão que resolve), Ficha, Servidor, Cofre, Acesso, Código e Histórico, uma seção por vez. Substitui a organização do Desenho B abaixo; as regras de edição continuam.
- **Desenho B (2026-10-05, ADR 009, organização substituída pela ADR 029):** uma página só. Primeiro a parte de uso (o que ele pede, o que ele entrega, execuções, quem cuida); no fim, o bloco recolhido "Para quem cuida do app" (pontos de atenção, outros apps, código-fonte, chaves, bibliotecas, histórico, onde está, acesso), que só aparece para quem pode ver a parte técnica. Cada parte tem seu "Editar" em texto cinza discreto; uma parte em edição por vez; se outra pessoa salvou antes, o Hub pede para recarregar. Entradas em lista mostram as opções e "pode marcar mais de uma".
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
  - **Rodar em poucos cliques (2026-10-01):** o botão Rodar fica também no cartão do catálogo. A pessoa continua na página onde estava e acompanha por uma **faixa de andamento** embaixo da tela ("Abrindo…", "aberto no seu computador", "terminou: N arquivos novos — Ver arquivos" e **Baixar** direto: um arquivo vem ele mesmo, vários vêm num .zip, erros com o próximo passo). A faixa vem do Hub: aparece igual depois de recarregar ou em outra aba, até a pessoa dispensar.
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
- **Empresa e times (combinado em 2026-09-30, para depois):** a empresa é a conta (nome, padrões, cofre, chave da IA); no piloto existe uma só (Alis). A Etapa 2 entregou áreas, líderes, apps restritos e acessos por app; ainda ficam para depois a pessoa em várias empresas, configurações próprias por time e limites. A aprovação antes de publicar já existe no Admin ("exigir aprovação") e começa desligada numa empresa nova.
- **Aprovação de app novo (Admin):** "exigir aprovação" começa **desligado** numa empresa nova; as empresas que já existiam mantêm a opção como estava. Artifacts (3.14) nunca passam por aprovação.

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

### 3.11 Claude conectado ao Ziv (MCP)
O Henry pediu isto em 2026-09-30 e redesenhou em 2026-11-04. A pessoa usa o Ziv de dentro do Claude que já usa (claude.ai, app do Claude ou Claude Code), sempre com as permissões dela e nunca mais que isso. Quem pensa é o Claude da própria pessoa, então o Ziv não precisa de chave da API do Claude.

**Os 5 usos:**
1. registrar apps prontos;
2. criar apps já pensando no Ziv;
3. publicar artifacts bonitos no Ziv;
4. agendar rodadas;
5. achar e rodar apps.

**As 4 etapas:**
- **Etapa 1 (feita em 2026-11-04, ADR 021):** conectar, achar, rodar e publicar artifacts.
  - **Tela Conectar o Claude:** fica em Configurações. Mostra o endereço do conector para o claude.ai e o app do Claude, um código pessoal para o Claude Code e a lista **Conexões ativas**, com o botão Cancelar.
  - **Conectar pelo claude.ai:** a pessoa clica em Conectar no Claude, entra no Ziv e confirma em "Permitir que o Claude use o Ziv como você?". A tela mostra para onde o acesso vai e avisa quando o endereço não é do Claude.
  - **Ferramentas:**
    - buscar, ver app, ver artifact;
    - rodar app;
    - ver execução, que espera até 30 s por vez para o Claude acompanhar até terminar;
    - minhas execuções;
    - publicar artifact, que cria um novo ou uma versão nova, com o mesmo aviso de segredos da tela;
    - guia de visual.
  - Rodar e publicar pedem "permitir?" no Claude, ficam marcados como **pelo Claude** e entram no registro de segurança.
  - O acesso acaba quando a pessoa cancela, quando é removida ou quando a chave vence.
- **Etapa 2: registrar apps prontos** (construída em 2026-11-06, ADR 026). O Claude Code lê a pasta do app no PC e segue o protocolo que `formato_do_app` devolve. A pessoa confere a ficha na conversa, e o registro só acontece depois do "pode registrar".
  - O código vai num .zip por um endereço de uso único (30 minutos), sem `.env`, chaves, `.git` nem `venv`.
  - O registro passa pelo mesmo caminho do import do site, e o responsável é quem registra.
  - Vale para scripts e apps com tela, um por vez ou vários.
  - Versão nova só para apps com o código guardado no Ziv. Ela pode mudar o código, o resumo, as entradas, as saídas e (nos scripts) o arquivo de início e as dependências. Nome, área, responsável, reserva e acesso só mudam pelo site.
  - Apps com tela vão ao ar com `publicar_app_com_tela`, com a mesma checagem, os mesmos avisos e a mesma aprovação do botão Publicar. As chaves são preenchidas só no site.
  - **Publicar sem ficar no escuro** (2026-11-11, ADR 034):
    - Se o app não sobe, o Claude recebe o motivo, o fim do log (com as senhas apagadas) e se a versão anterior continua de fato no ar.
    - Quando o programa morre logo ao ligar, o Ziv avisa em segundos.
    - Enquanto publica, mostra a etapa e o tempo.
    - `ver_app` diz se o app está no ar e qual versão roda.
    - `ver_log_app` mostra o log para quem cuida do app.
  - **Chaves sem falso positivo** (2026-11-11, ADR 035):
    - O Ziv só pede a chave que o código lê de fato; uma leitura com valor padrão não é obrigatória.
    - A lista de ajustes do app com tela é refeita a cada versão, e as `keys` de uma versão nova valem.
    - `enviar_chaves` deixa de fora os nomes do .env que o app não usa.
    - `apagar_chave` apaga uma chave, só depois que a pessoa confirma.
  - **`problemas`** (2026-11-11, ADR 036): numa chamada, o Claude vê o que deu errado nos apps que a pessoa cuida (execução com erro, agenda pulada, app com tela fora do ar, publicação que falhou, ajuste faltando), com o trecho do log e o link.
  - As versões ficam marcadas "Código enviado · pelo Claude".
  - **Cofre do app** (2026-11-09, ADR 027): as senhas de um script ficam no cofre do próprio app, nunca no código nem na conversa.
    - O Claude troca a senha no código por `os.environ["NOME"]`, declara o nome em `keys` e devolve o link da parte "Cofre".
    - O dono do app ou um administrador preenche, mostra (cada visualização é registrada), troca e apaga.
    - O Agente entrega as chaves só ao script, na hora de rodar.
    - Se o Ziv recusa um envio por senha no código, diz o arquivo e a linha de cada trecho.
    - **Em todo app que roda** (2026-11-09, ADR 028): scripts, apps com tela, agendados e executáveis têm Cofre. Uma chave vale nos dois lugares: quando o app abre no computador e na versão ligada 24h no servidor. As senhas das Configurações e o Cofre são a mesma coisa.
    - O Claude preenche o cofre pelo `.env` da pasta sem ver as senhas: a ferramenta `enviar_chaves` dá um endereço de uso único e o Claude manda o arquivo direto ao Ziv com `curl`. O Ziv responde só os nomes guardados.
    - **App com tela no computador:** o botão "Abrir no computador" faz o Agente pegar o código guardado no Ziv, preparar as bibliotecas, ligar o app numa porta livre com as chaves do cofre e abrir o navegador. "Parar" desliga.
- **Etapa 3: criar apps para o Ziv.** O Claude recebe o formato do manifesto, as regras e exemplos, e entrega o app com o `ziv.yaml` pronto.
- **Etapa 4: agendar.** Primeiro o Ziv aprende a agendar; depois o Claude ganha agendar, ver agendamentos e cancelar.

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

### 3.14 Artifacts (construído em 2026-11, ADR 018)
Páginas que o Claude (ou alguém) gerou, como um gráfico ou um painel em HTML, guardadas e compartilhadas pelo Ziv. Não rodam nada: só se abrem.
- **Como entra:** colando um **link** (`https://`) ou subindo **um arquivo** `.html` (até 16 MB). Quem tem o cargo de registrar apps registra artifacts.
- **Catálogo próprio:** aba Artifacts, com cartões, **busca** por nome, etiqueta e área, filtro por área e **etiquetas** livres.
- **Mesmas regras de acesso dos apps:** aberto ou restrito, responsável e reserva, área, acessos extras. As permissões de cargo (editar qualquer ficha, voltar versões, ver todos) e os acessos dados na área toda valem também para os artifacts da área; o acesso extra num artifact só oferece editar a ficha e voltar versões. Os acessos da área toda de ver a parte técnica e ver execuções valem só para apps: não mostram um artifact restrito. Quem cuida do artifact (responsável, reserva, líder da área) muda o responsável, a reserva e a área na própria página dele, em "Sobre" → "Editar quem cuida".
- **Link mostrado dentro da página:** ao salvar um link, o Ziv confere se o site aceita aparecer dentro de outro (cabeçalhos `X-Frame-Options` e `frame-ancestors`). Se aceitar, como os links de incorporar de painéis, o link aparece na página do artifact igual a um arquivo, com "Abrir em tela cheia". Se não aceitar, fica o cartão com "Abrir" em outra aba. Links do Claude (claude.ai, claude.site) não aceitam: a tela sugere baixar o HTML no Claude e subir como arquivo.
- **Versões:** cada envio novo vira uma versão; dá para abrir uma antiga e voltar a ela.
- **Sem aprovação:** artifact novo já aparece, nunca vai para "Esperando aprovação" no Admin.
- **Segurança:** o HTML aparece numa **caixa isolada** (iframe com sandbox e cabeçalho `Content-Security-Policy: sandbox`): a página roda scripts, mas não enxerga o login nem a conta de ninguém. Segredos escritos no HTML são tratados como na importação de scripts.
- **Área com artifacts não pode ser apagada** até eles mudarem de área.
- **Depois:** publicar o artifact pelo MCP (3.11). A aba Home está em 3.15.

### 3.15 Home (construído em 2026-11, ADR 019)
A primeira tela depois do login (`/`): o resumo do dia da pessoa. O catálogo de apps passou para `/apps`.
- **Atenção** (some quando não há nada): minhas execuções com erro, paradas esperando o computador ou interrompidas porque o computador não respondeu ("{app} não rodou: seu computador não respondeu"; dá para "Dispensar"); meu computador desconectado ou com Agente desatualizado; para quem cuida de apps (responsável, reserva, líder da área): app compartilhado com problema, app com execução com erro na semana e versão nova esperando aprovação (só se a empresa ligou a aprovação). Cada aviso leva à página certa. Quem nunca conectou um computador não recebe aviso de computador: o "Começar" já cobre isso.
- **Favoritos:** apps e artifacts marcados com a estrela, na ordem em que foram marcados. A estrela fica nos cartões e nas páginas de app e de artifact; clicar nela não abre o cartão. Sem favoritos, uma linha curta explica a estrela.
- **Usados recentemente:** até 6 apps e artifacts que a pessoa rodou ou abriu nos últimos 30 dias, sem repetir os favoritos. Abrir um artifact conta como uso (no máximo uma vez a cada 30 minutos por pessoa e artifact).
- **Minhas últimas execuções:** as 5 mais recentes, com link para "Minhas execuções".
- **Meu time** (administrador = empresa toda; líder de área = pessoas e itens das áreas que lidera; quem não lidera nada não vê a seção): **Quem está usando** (execuções e artifacts abertos nos últimos 7 dias, por pessoa) com a lista de quem ainda não usou; **O que estão criando** (apps e artifacts novos ou com versão nova nos últimos 14 dias); **Pendências de gestão** (só para quem cadastra pessoas): convites não aceitos (com a marca "venceu"), pedidos de acesso e pessoas sem área.
- Partes vazias não aparecem; a única que sempre aparece é "Favoritos".
- Nada disso guarda um "feed" próprio: a Home lê o que já existe, e tudo o que mostra passa pela mesma checagem de acesso do item.
- **Depois:** "apps e artifacts sem responsável ativo" volta quando for possível desativar uma pessoa.

### 3.16 Agendar (entrega 1 construída em 2026-11, ADR 024)
Quem pode rodar um script pode agendá-lo para rodar sozinho no próprio computador, com o Agente ligado.
- **Quando:** a cada N minutos (mínimo 5) ou N horas, todo dia, em dias da semana ou todo mês (dia 1 a 28), sempre em horário de Brasília. A tela mostra as próximas 3 execuções antes de salvar.
- **Onde:** um computador fixo da pessoa. Desligado na hora, a execução aparece como **Perdida**.
- **Histórico:** toda execução agendada aparece no histórico com o selo "agendada". Quando não dá para rodar sem ser falha (a anterior ainda rodando, aprovação pendente), o agendamento mostra a situação.
- **Pausa automática:** depois de 5 falhas seguidas, quando a pessoa perde o acesso, quando o app é suspenso ou quando o app muda e os campos guardados não servem mais. O motivo aparece no agendamento e na Home do dono, que reativa com um clique.
- **Quem vê:** cada pessoa vê os seus agendamentos. Quem cuida do app vê todos e pode pausar ou apagar, sem ver os dados dos outros.
- **Depois:** rodar no servidor (para apps que não precisam do PC) e e-mail opcional com o resultado.

---

## 4. Fluxos principais

**Entrada:** depois do login, a pessoa cai na Home (3.15). De lá chega aos apps (`/apps`), aos artifacts e às execuções; os fluxos abaixo partem do catálogo.

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
- Encadeamento de apps e presets salvos. (O agendamento saiu daqui: ver 3.16.)
- Desenvolvimento compartilhado com resolução de conflitos.
- Trilha de auditoria exportável, resumo semanal por e-mail, tempo economizado.
- Execução de planilhas com macro e de sistemas complexos (só registro e ficha).
