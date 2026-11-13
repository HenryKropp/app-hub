# TASKS — Ziv (Módulo 1, piloto local)

> Registro histórico do piloto. Os estados abaixo não substituem as tarefas
> atuais do projeto. Para o estado geral em 26/10/2026, consulte o
> [README](../README.md). Em particular, não considere o fluxo Windows completo
> validado nem a execução gerenciada na nuvem entregue.

Status: Pending / In Progress / Done / Blocked

## Fase 0 — Planejamento

| Tarefa | Status | Notas |
|---|---|---|
| Ler especificação e apps de teste | Done | Pasta `8. Programação` analisada: scripts, Streamlit, desktop, agendados, `.exe`, documentos |
| PRD do Módulo 1 | Done | `docs/PRD.md` |
| Decidir onde os apps executam (Windows vs WSL) | Done | ADR 001: Hub no WSL lendo `/mnt/c`; execução pelo Agente do Ziv no Windows (ADR 004) |

## Fase 1 — Fundação

| Tarefa | Status | Notas |
|---|---|---|
| Esqueleto Go + React + Postgres local | Done | Banco `ziv-db` (antes `app-hub-db` e `kova-db`) na porta 5433; Dockerfile pronto |
| Layout base, identidade visual, navegação | Done | Estilo novo (2026-10-02): limpo e neutro como Notion/Vercel, um violeta vivo só no Rodar e no que está aberto agora; claro por padrão, com escuro; Painel e Cofre marcados "em breve" |
| Registrar apps pelo + do catálogo | Done | 2026-10-02, pedido do Henry: saiu da barra lateral; virou o botão "+ Registrar apps" ao lado da busca (só "+" no celular), só para administrador e criador. A página tem "← Catálogo" e o Catálogo fica marcado enquanto se registra |
| Configurações | Done | 2026-10-02, pedido do Henry: item no pé da barra lateral (no celular, no menu do avatar) com Meu computador e Aparência (claro, escuro, igual ao sistema). O endereço antigo `/meu-computador` leva para lá |
| Cadastro de pessoas e login simples (protótipo) | Done | ADR 003. Primeiro acesso cria o administrador; perfis admin/criador/usuário (substituídos por cargos na Etapa 2, ADR 009) |

## Fase 2 — Manifesto e registro (1.1, 1.5)

| Tarefa | Status | Notas |
|---|---|---|
| Especificar o formato do manifesto (genérico, versionado) | Done | ADR 002, `app-hub.yaml` v1 (`internal/manifest`) |
| Diagnóstico por regras | Done | `internal/scanner`; testado nos 19 apps da pasta de teste (só leitura) |
| Detecção de segredos no código | Done | Mostra arquivo e linha, nunca o valor. "Mover para o cofre" entra na Fase 6 |
| Varredura em lote de uma pasta | Done | Um app por subpasta; ignora `.venv`, `node_modules`, pastas ocultas; sugere pular pastas "Old/arquivo" |
| Tela de revisão do diagnóstico + perguntas do incerto | Done | Tela Registrar apps |
| Salvar manifesto e ficha na pasta do app | Done | Cria só `app-hub.yaml` e `FICHA.md`; recusa sobrescrever arquivo que não é do Ziv |
| Aviso de duplicado | Done | Por palavras do nome, contra o catálogo e dentro do lote |

## Prioridade atual (2026-10-01)

Pedido do Henry: **deixar o Ziv redondo com um app só (Eq. Research Files Prep) antes de registrar mais apps.** Fase 4 para apps com janela e `.exe` feita e testada de verdade (2026-09-30). Ordem combinada depois da decisão do Ziv (ADR 005):
1. ~~Atualização automática do Agente~~ Feita e testada de verdade (2026-09-30)
2. ~~**Fase 5:** histórico de mudanças no código e na ficha, e voltar atrás~~ Feita e testada de verdade no Files Prep (2026-10-01)
3. **UX em 3 etapas** (desenho: `docs/superpowers/specs/2026-10-01-etapa-1-rodar-poucos-cliques-design.md`; protótipo: `2026-10-01-prototipo-ux.html`):
   - **Etapa 1, rodar em poucos cliques:** **feita** (código 2026-10-01; telas e teste de verdade com o Henry 2026-10-02). Ficha abre na hora, Rodar no cartão, pasta lembrada, faixa de andamento, Minhas execuções
   - ~~**Estilo visual novo do zero**~~ **feito** (2026-10-02). O Henry escolheu, depois de dois sorteios, o padrão Notion/Vercel com um violeta vivo de destaque (como o verde do Spotify). Letra Geist; barra lateral clara; Rodar como botão violeta arredondado; barrinhas violeta animadas no que está aberto agora; tipos de app em etiqueta cinza. Todas as telas trocadas; no celular, menu da conta (tema e Sair) no avatar. `DESIGN.md` gravado. De quebra: o aviso "Abrindo…" às vezes ficava até 3 s junto com o cartão da execução; corrigido. Decisão em `docs/adr/008-estilo-visual-neutro-com-violeta.md`
   - **Acabamento da Etapa 1** (avaliação Impeccable do catálogo em 2026-10-02, nota 25/40; relatório em `.impeccable/critique/`): ← próxima, já no estilo novo. O Henry escolheu corrigir tudo o que foi achado, começando por "onde ficam os arquivos":
     1. ~~Onde ficam os arquivos~~ **feito** (2026-10-02): "Salva em … · trocar" no pé do cartão (nome da pasta; caminho inteiro ao passar o mouse), "Salva em …" na faixa desde o "Abrindo…", e o caminho inteiro na ficha e na janela. Janela sem pasta padrão: na 1ª vez começa vazia com "Escolher pasta…"; depois lembra
     2. ~~Faixa~~ **feito** (2026-10-02): ordena antes de cortar em 3 (erro e interrompido > terminou > sem arquivos > aberto/abrindo), então o que fica no "+N" são os apps ainda abertos; a faixa fica sempre montada como região que o leitor de tela anuncia, e os erros são alerta; "Ver e baixar" já estava legível no escuro com o estilo novo (conferido em foto)
     3. ~~Mensagens~~ **feito** (2026-10-05): "sem arquivos novos" cinza e explicado na faixa e na página da execução; erros do navegador ("Failed to fetch") e respostas 5xx viram português claro; o erro do Windows ao abrir o app vira uma frase simples (o detalhe técnico fica no `hub.log`); "Tentar de novo" no catálogo, Minhas execuções, ficha e página da execução
     4. ~~Saídas~~ **feito** (2026-10-05), sem o Parar: Cancelar na janela de rodar; "Ver" na faixa enquanto o app está aberto (não tira da faixa); depois de "Ocultar", um lembrete "N aberto oculto · Mostrar". **Parar** ficou para depois, a pedido do Henry: precisa do Agente 0.2.4 e de teste de verdade
     5. ~~Acabamento~~ **feito** (2026-10-05): busca com nome para leitor de tela, X e "Limpar busca", Esc limpa; "Fechar" em vez de "Close"; "Em breve" só para administrador e criador; catálogo vazio com texto próprio para quem só usa; link "Pular para o conteúdo"; contador "abertos agora" no plural; no celular a faixa mostra um cartão só e conta o resto. O selo "Janela", as legendas, o vermelho do título e o contador vermelho já tinham sido resolvidos pelo estilo novo (conferido em foto). Fotos: `e2e/acabamento.mjs` (só leitura)
   - **Vários apps abertos ao mesmo tempo no mesmo computador** (pedido do Henry em 2026-10-28): **código feito** (2026-10-28, ADR 012, Agente 0.3.2). O Agente abre quantos apps a pessoa quiser, desde que em pastas separadas. Pasta igual, de dentro ou de fora de uma pasta em uso é recusada, no Ziv (na hora do Rodar, com "Trocar pasta") e no Agente: "Essa pasta já está sendo usada pelo X, que está aberto agora…". Assim nenhum arquivo vai parar no app errado. Scripts podem dividir a pasta entre si, porque cada um grava na sua Execucao-N. O Parar fecha só o app escolhido. A atualização automática espera todos os apps fecharem sem impedir que outros abram nesse meio-tempo. Testes automáticos passando. Agente do Henry já atualizado sozinho para 0.3.2. **Falta:** teste de verdade com dois apps diferentes abertos juntos; espera um segundo app registrado, que o Henry quer registrar pelo Claude (Fase 2b, MCP)
   - **Parar um app aberto pelo Ziv** (pedido do Henry em 2026-10-05): **código feito** (2026-10-28, ADR 011). Agente 0.3.1 com o pedido de parar (fecha o app e o que ele abriu); botão Parar com confirmação na faixa e na página da execução; estado novo "Parada" que guarda os arquivos criados até ali; pedido ainda não aberto é só cancelado. Testes automáticos passando. **Teste de verdade no PC do Henry (2026-10-28): passou** com a Ferramenta CVM, que abre um segundo programa: os dois fecharam, a execução ficou "Parada" e o Agente seguiu ligado. Antes do teste, o Agente do Henry foi trocado pelo 0.3.1 feito aqui, porque o 0.3.0 do Replit apontava para o endereço privado do Replit e tinha perdido a conexão. **Falta:** parar um script Python de verdade (ainda não há script registrado no Ziv do Henry)
   - Ficha mais limpa (2026-10-02, pedido do Henry): quadro de 5 caixas virou uma linha discreta abaixo do resumo; entradas dizem uma vez de onde vêm ("Você preenche tudo na janela do app"); saídas sem o padrão técnico (fica no passar o mouse); pontos de atenção mostram 3 e "Ver mais N"; histórico mostra os 5 mais novos e "Ver histórico completo", com filtro em abas pequenas e ações das versões discretas; pasta sem fonte de código
   - ~~**Etapa 2, cargos, permissões e ficha editável**~~ **feita** (2026-10-05; desenho: `docs/superpowers/specs/2026-10-05-etapa-2-permissoes-e-ficha-design.md`; ADR 009). As abas Usar/Cuidar foram descartadas; a ficha ficou no desenho B:
     1. **Done:** cargos personalizáveis (Administrador, Criador, Usuário prontos) com 9 permissões; áreas com líderes e membros; acessos extras por app ou área; app aberto ou restrito (quem não vê recebe "App não encontrado"); regra única em `internal/access`; migração 007
     2. **Done:** tela Equipe (Pessoas, Áreas, Cargos); ficha no desenho B com bloco recolhido "Para quem cuida do app" e "Editar" por parte, inclusive os campos do formato v0.2 (opções, "pode marcar mais de uma", outros apps, código-fonte); registro com os mesmos editores; fotos em `e2e/etapa2.mjs` (só leitura)
     3. **Done:** correções da revisão final (2026-10-05): ninguém cadastra com cargo maior que o seu nem muda o cargo de quem tem mais; cargos da empresa toda não revelam app restrito sem "ver todos os apps"; só quem cuida do acesso troca responsável, reserva e área; acessos de app e de área não se apagam; Equipe mais clara no celular
     4. **Pending:** teste de verdade com o Henry (criar área com líder, restringir um app e conferir com um usuário de teste que ele some do catálogo)
     5. **Pending (fora desta etapa):** ~~Parar um app aberto~~ (feito e testado de verdade em 2026-10-28); visibilidade por time como conta separada; aprovação antes de publicar; código de acesso pessoal e MCP
   - Etapa 3, catálogo e primeiro uso (favoritos, filtro por área, status e "usado há…", guia do Agente): **Pending**, depois da Etapa 2
4. Rodar scripts: formulário automático e ambiente preparado sozinho
5. Ziv: ícone perto do relógio, janela própria e instalador único (etapas 2 e 3), antes de o primeiro colega entrar
6. Assinatura digital do Ziv (etapa 4), antes de vender
7. Depois: retomar a Fase 2b (teste com mais 4 apps e MCP)
8. **A decidir: apps para vários ao mesmo tempo** (dúvida do Henry, 2026-10-02). Ideia no PRD 3.3: opção "pessoal" ou "compartilhado" no manifesto; o compartilhado roda uma vez só no servidor central, com um banco de dados só e a senha no cofre. Decidir junto com "Rodar apps web" e antes do primeiro colega usar um app desses
9. **Ideia: compartilhar skills do Claude pelo Ziv** (Henry, 2026-10-26). Ver PRD 3.12. Antes de desenhar: pesquisar como publicar skills no claude.ai
10. **Ideia: gestão de gastos com IA** (Henry, 2026-10-26). Ver PRD 3.13. O nível 2 (chave centralizada no Ziv) é o que mais vale

**Para a próxima sessão (deixado em 2026-10-01, à noite):**
- Etapa 1: Hub da 8080 reiniciado em 2026-10-02 (migração 006 aplicada, `DEV_MODE=1`, registro em `backend/hub.log`). Telas conferidas com `e2e/etapa1.mjs` (só leitura; faixa e pasta lembrada simuladas): catálogo com Rodar, janela com "Lembrar esta pasta", faixa, Minhas execuções e "De todos", ficha com "salva em … · trocar", claro/escuro/celular. A barra lateral acompanha a rolagem. Sobras visuais: no celular, 3 cartões na faixa cobrem quase meia tela; o espaço embaixo da página (`pb-32`) é menor que a faixa com 3 cartões.
- Teste de verdade no Files Prep com o Henry (2026-10-02), conferido nos dados do Ziv: sem faixa ao abrir; 1ª vez pela janela com "Lembrar esta pasta" (pasta gravada); dispensar com ✕ não volta no F5; 2ª vez direto do cartão, sem janela; Rodar de novo por Minhas execuções; "terminou: 4 arquivos novos" → Ver e baixar abre a página e tira a faixa. Execuções 3 a 5 deram "sem arquivos novos" porque o app foi fechado antes de baixar algo novo (comportamento certo).
- Observar: na 1ª tentativa o ✕ sumiu com a faixa, mas a dispensa não chegou ao Ziv (não se repetiu). A faixa "sem arquivos novos" usa o ícone vermelho de alerta, que parece erro para um resultado normal.
- Seguir o fluxo: desenho → plano → implementação para a Etapa 2.
- O Agente no PC do Henry está na **0.2.3** e se atualiza sozinho. O código do Ziv também está na 0.2.3, e `dist/agent/AppHubAgente.exe` da pasta principal já é essa versão.
- Para publicar um Agente novo: subir `agentproto.Version`, rodar `scripts/build-agent.sh` e reiniciar o Ziv.
- Nunca ligar um Hub de teste (com outro banco) na porta 8080: o Agente real fala com ele e esquece a conexão.
- Ainda falta o Henry conferir a janela de pastas moderna (item da Fase 4).

## Nome novo: Ziv (2026-10-29, ADR 014)

Kova saiu no mesmo dia porque, dito em voz alta, soa como "cova". O símbolo também mudou: o "i" do nome com o pingo coral e três raios finos, como um farol.

| Tarefa | Status | Notas |
|---|---|---|
| Nome, ícone, nome desenhado e ícone da aba | Done | `lib/brand.ts`, `internal/brand`, `Logo` e `Wordmark` em `bits.tsx`; desenho no BRAND.md |
| Nomes técnicos com convivência de três gerações | Done | Escreve `ziv`, lê `kova` e `app-hub`; `kova-db` e a pasta `kova/shared` foram adotados na primeira subida (2026-10-29, conferido) |
| Agente com os textos novos | Pending | Só uma mensagem rara mudou; entra no próximo Agente publicado (subir `agentproto.Version`) |
| Checar a marca Ziv no INPI (classes 9 e 42) | Pending | Busca manual em busca.inpi.gov.br. Domínios `ziv.com`/`.com.br` têm dono; `zivhub.com`/`.com.br` estavam livres |
| Atualizar o design system publicado | Todo | Tokens, README, capa, logo e prévias do artifact |
| Renomear o repositório no GitHub e o módulo Go | Todo | Depende do Henry (Settings › Rename); depois trocar `go.mod` e imports |
| Migração forçada dos manifestos antigos e fim da convivência | Todo | Quando nenhum app registrado usar mais `kova.yaml` nem `app-hub.yaml` (ver ADR 013 e 014) |

## Nova identidade: Kova (2026-10-29, ADR 013; trocada pelo Ziv no mesmo dia)

| Tarefa | Status | Notas |
|---|---|---|
| Nome num lugar só, cores, símbolo, textos "o Kova" | Done | `lib/brand.ts`, `internal/brand`; coral só como preenchimento; `scripts/contrast-check.mjs` |
| Nomes técnicos com convivência (manifesto, lixeira, etiquetas, banco, pasta de dados) | Done | Escreve o novo, lê o antigo; `app-hub-db` e a pasta de dados são adotados na subida |

## Entrada por e-mail e Começar em 6 passos (2026-11-05, ADR 023)

| Tarefa | Status | Notas |
|---|---|---|
| Onboarding: `claude_connected` e `has_artifacts` | Done | |
| `POST /api/entry/login`: e-mail e senha em todas as empresas, menos a principal | Done | |
| `SETUP_CODE` no primeiro acesso da principal | Done | |
| Tela Entrar só com e-mail e senha; cadastro leva ao Começar | Done | |
| Começar em lista com 6 passos | Done | |
| Configurações só com o status | Done | |
| Cabeçalho maior na página inicial | Done | |
| Henry testa no servidor | Done | 2026-11-05: cadastro → Começar, Entrar só com e-mail, Configurações só status |
| Entrar com Google e Microsoft | Pending | "eventualmente" |

## Servidor 24h grátis no Google Cloud (2026-11-05, ADR 022)

| Tarefa | Status | Notas |
|---|---|---|
| Conta e máquina e2-micro (`us-east1`) | Done | A Oracle recusou o cadastro (antifraude) |
| Endereço `zivapp.duckdns.org` + HTTPS (Caddy) | Done | Subdomínios dos apps com certificado sob demanda |
| Banco `ziv-db`, serviço do Ziv, cópia diária | Done | `scripts/deploy-gcp.sh` publica versões novas |
| Henry testa: cadastro, Agente novo, rodar um app | Pending | Agente baixado do próprio site novo |
| Conector do claude.ai (MCP OAuth) no endereço novo | Pending | Etapa que dependia do deploy |
| Cópia do banco fora da máquina | Pending | Hoje fica no mesmo disco |
| `ask` do TLS só para apps existentes | Pending | Hoje aceita qualquer subdomínio |
| Conferir a fatura do Google nos primeiros dias | Pending | Deve dar zero |
| Primeiro acesso da empresa principal protegido | Done | `SETUP_CODE` no `ziv.env` (ADR 023) |

## News Scrapper no servidor do Ziv, sempre ligado (← próxima sessão, decidido em 2026-10-28)

Pedido do Henry: o objetivo é as pessoas pararem de rodar apps no PC e usarem pelo Ziv. O News Scrapper (`C:\Users\HenryKroppAlisInvest\OneDrive - Alis\ALIS\5. Operacional\8. Programação\News Scrapper`) hoje é um script Python (`scraper.py`, que grava `news_data.json`, `dashboard.html` e `scraper.log` na própria pasta e lê `config.json` e `watchlist.json`) rodado a cada 5 min pela tarefa agendada "NewsScrapper", mais o `server.py`, que mostra o dashboard na porta 8080. Desenho combinado:

| Tarefa | Status | Notas |
|---|---|---|
| Modo "sempre ligado" para app compartilhado | Pending | Hoje o app dorme após 30 min sem acesso; este não pode dormir. Botão Ligar/Desligar |
| Adaptar o News Scrapper como app compartilhado | Pending | Dados em `APPHUB_DATA_DIR`; ciclo de 5 min 24/7; serve o dashboard em `PORT`; sem auto-instalação por pip nem notificações do Windows (plyer); dependências: requests, beautifulsoup4, feedparser, Pillow |
| Publicar e testar de verdade | Pending | Precisa da 8080 livre (o alis-research-hub-v2 usa a mesma porta) e do Podman respondendo (em 2026-10-28 `podman version` estava sendo morto) |
| Henry desliga a tarefa agendada do PC | Pending | Depois de o app estar no ar |

## Teste combinado do MCP (2026-11-11)
- [x] Passo 1 "problemas": achou os 3 erros em linguagem simples. Ajuste: conferir em qual PC a execução rodou antes de dizer que a pasta foi digitada errada (C:\Users\hkrop era outro PC).
- [x] Passo 2 (E): sessão nova criou "Mostra as notícias do dia de uma empresa" (app 14 da Teste) do jeito Ziv sozinho, mensagem no padrão.
- [x] Passo 3: rodou pelo site, página boa. Passo 4: Claude rodou e conferiu sozinho.
- [x] Passo 5: app com tela de notícias no ar de primeira (meta de 1 versão atingida). Antes travou no limite de 2 apps publicados por empresa; subido para 5 (prod `SHARED_APPS_PER_COMPANY=5` e padrão no código, ADR 025).
- [ ] Ajuste pequeno: em `problemas`, dizer em qual PC a execução rodou, para o Claude não chutar "pasta digitada errada".

## Página do app em abas (2026-11-11, ADR 039)
Henry achou a página do app confusa ("abrir ou no meu computador"). Revisão e 5 opções em https://claude.ai/artifact/4qtVwxi6RjcsydxxQJMz9m; escolhida a A (abas).

| Tarefa | Status | Notas |
|---|---|---|
| Trocar Usar · Cuidar por abas (Início, Execuções, Documentação, Configurações, Servidor, Histórico) | Done | Selo de estado ao lado do nome; um botão principal; "Testar no meu computador" em Mais, explicado |
| Teste do Henry em apps reais (script e app com tela no servidor) | Done | 2026-11-11 no zivapp: Início, Documentação (editou Pontos de atenção), Mais → Testar no meu computador e selo, tudo OK |
| Ilustração da landing ainda mostra "Usar · Cuidar" | Pending | Desenho de marketing em `LandingPage.tsx`; trocar quando mexer na landing |

## Rodar pelo celular ou de outro PC (decidido 2026-11-11, depois do teste do MCP)

Decisões do Henry (ainda sem documento de projeto escrito):
- Por padrão o app roda no **servidor** (do celular, sem Agente). Só vai para o PC se marcado **"precisa do PC"**: usa pasta do PC/rede, controla o programa Excel (macros, COM, Bloomberg), outro programa do Windows/sistema interno, ou login com pessoa na tela. Excel só com .xlsx (openpyxl/pandas) roda no servidor. O Claude marca pela receita do Ziv; o Henry muda na página do app.
- App "precisa do PC": roda no **PC fixo do app** (escolhido uma vez, como no agendamento, ADR 024). PC desligado → avisa e não roda (sem fila).
- App de outra pessoa: mesma regra; "precisa do PC" roda no PC fixo do dono, resultado na pasta do dono; quem clicou vê avisos e resultado no Ziv.
- Resultado no celular: certo/errado + texto do app; execução no servidor que gerou arquivo → botão baixar; no PC o arquivo fica na pasta e o Ziv mostra onde (não sobe ao servidor).
- Decisão do cofundador: limite por execução no servidor (palpite 5 min, 256 MB), uma por vez; se não der, aumentar a máquina.
- [ ] 1. Rodar no servidor pelo celular (marca "precisa do PC" + execução no servidor + resultado) — resolve "ver notícias"
- [ ] 2. PC fixo por app + Rodar à distância pelo Agente — resolve "baixar relatórios"
- [ ] 3. Baixar o arquivo gerado no servidor
- Próximo passo: escrever o design do projeto 1 em docs/superpowers/specs/, revisor, aprovação, plano.

## Fase 2b — Claude conectado ao Ziv (MCP, 3.11)

Desenho aprovado pelo Henry em 2026-11-04 (`docs/superpowers/specs/2026-11-04-mcp-design.md`). A ordem mudou: primeiro conectar, achar, rodar e publicar; depois registrar, criar e agendar.

### Próximo — Claude entender e usar o Ziv ("jeito Ziv")

Plano aprovado em 2026-11-11, a construir: `docs/superpowers/specs/2026-11-11-mcp-jeito-ziv-plano.md`. A apresentação nova, a receita "como fazer app para o Ziv", o passo "adaptar" no registro, o jeito Ziv de falar (dicionário, respostas prontas, ficha padrão, guia de escolha) e o teste numa sessão nova.

| Tarefa | Status | Notas |
|---|---|---|
| F0 Estabilidade | Done | 2026-11-11 (ADR 034). O servidor não caiu: memória sobrando, Ziv ligado sem reinício, e os ECONNREFUSED nunca chegaram ao servidor (rede do PC). Achados extras: Playwright no app com tela falha sem `libatomic` (receita automática sem navegador); imagens antigas iguais (mesmo ID, várias tags) não são apagadas (`rmi` por ID recusa) |
| F1 Publicar sem ficar no escuro | Done | 2026-11-11 (ADR 034): o Ziv percebe em segundos que o programa morreu (código e tempo); falha traz o fim do log com senhas apagadas e diz se a anterior está de fato no ar; etapa e tempo enquanto publica; `ver_app` com `no_ar`/`versao_no_ar`/`ultima_publicacao`; ferramenta `ver_log_app`. **Falta:** deploy no servidor e teste do Henry |
| Limpeza de imagens com várias tags | Done | 2026-11-11: apaga primeiro pela tag da implantação e depois pelo ID (podman recusa apagar por ID uma imagem com duas tags; conferido). As imagens que já sobraram no servidor pedem limpeza cuidadosa, só das implantações aposentadas (não usar `prune -a`: apagaria a imagem guardada para voltar à versão anterior) |
| F1 resto: `ver_app_no_ar` (GET como a pessoa) | Pending | Revisar segurança: só leitura, só apps que a pessoa vê |
| F2 Chaves sem falso positivo | Done | 2026-11-11, ADR 035: leitura com padrão é opcional, constante não é chave, lista refeita a cada versão, `keys` vale para app com tela, `enviar_chaves` ignora nome que o código não lê, `apagar_chave` com confirmação. Adiado: "não é chave, ignorar" no cofre |
| F3a `problemas` | Done | 2026-11-11, ADR 036: execução com erro, agenda pausada/pulada, app com tela fora do ar, publicação que falhou, ajuste faltando; só leitura, como a pessoa |
| A+B Apresentação nova + `receita_app_ziv` | Done | 2026-11-11, ADR 037 |
| C Passo "adaptar" no protocolo | Done | 2026-11-11, ADR 037: detectar padrões errados, mudar só com OK, testar numa cópia (F4b) |
| D Jeito Ziv de falar (`mensagem_para_pessoa`, teste de palavras, ficha padrão, guia de escolha) | Done | 2026-11-11, ADR 037 |
| E Teste de verdade numa sessão nova | Pending | No teste final com o Henry |
| F4 app com tela: conferir um texto na página antes de trocar de versão | Done | 2026-11-11, ADR 038: `check_text` na ficha, `conferir_texto` no MCP; não serve para Streamlit |
| F4 resto: "como testar" de scripts (entradas de exemplo + ensaio), campo no site, vigia periódica | Pending | Ver plano, adendo 3 |

### Etapa 1 — conectar, achar, rodar e publicar artifacts (ADR 021)

| Tarefa | Status | Notas |
|---|---|---|
| Conexões, códigos pessoais e origem "pelo Claude" | Done | Migração 019; `mcp_connections` só com hash; `runs.origin` |
| Endereço `/mcp` por empresa, com chave obrigatória | Done | SDK oficial em Go; `callAPI` limitado a 11 rotas; `Cache-Control: no-store` |
| OAuth 2.1 para o claude.ai e o app do Claude | Done | DCR, PKCE S256, chave de renovação troca a cada uso; descoberta por empresa com sufixo de caminho; limite por endereço |
| Ferramentas: buscar, ver app/artifact, rodar, ver execução (espera 30 s), minhas execuções, publicar artifact, guia de visual | Done | Rodar e publicar gravam auditoria `mcp.request.*`; nada de segredos nas respostas |
| Tela Conectar o Claude e confirmação de acesso | Done | Em Configurações (`#claude`), passo opcional em Começar; aviso quando o destino não é do Claude; selo "pelo Claude" |
| Teste real com o Claude Code | Done | 2026-11-04: `claude mcp add` na empresa Teste, conectado, `buscar` devolveu os 7 apps |
| Teste com as mãos do Henry | Done | 2026-11-04 na empresa Teste: código gerado, Claude Code listou os apps, rodou o Olá Ziv (agora pergunta a pasta antes de rodar, pedido do Henry), publicou o artifact "Calculadora de Fita" pelo Claude, e cancelar cortou o acesso. O claude.ai fica para depois de publicar |
| Recusa por senhas no código diz onde | Done | 2026-11-09: a recusa (bloqueio ou confirmação) traz arquivo, linha e nome de cada trecho, nunca o valor, para o Claude corrigir. Antes vinha só a quantidade e o Claude tentava às cegas |
| Cofre do app (chaves de scripts) | Done | 2026-11-09, ADR 027. Cada script declara `keys` na ficha; dono ou administrador preenche, mostra (registrado), troca e apaga na parte "Cofre"; o Agente 0.3.4 pega as chaves uma vez por execução e entrega só ao script. Pelo Claude: `keys` no registro e link `#chaves`. Desde 2026-11-09 a parte Cofre aparece em todo script e o dono adiciona chaves pelo site ("Adicionar chave": nome + valor); o item "Cofre em breve" saiu do menu |
| Cofre também nos apps de navegador | Done | 2026-11-09 (ADR 028): Cofre em todo app que roda; mesma linha de `app_config` das Configurações (vale no PC e na versão 24h, com aviso de reiniciar); app com tela abre no computador pelo Agente 0.3.5 com as chaves; MCP `enviar_chaves` manda o `.env` direto ao Ziv sem passar pela conversa. No ar e testado pelo Henry em 2026-11-09 com o app `ziv-exemplo-cofre`: cofre pelo Claude, versão 24h, "No meu computador" (127.0.0.1, chaves e pasta) e Parar |
| Teste no claude.ai | Pending | Só depois de o Ziv estar publicado na internet |
| Pendências pequenas do MCP | Pending | Sem detecção de reuso da chave de renovação; parâmetro `resource` ignorado; `ver_app` carrega o catálogo inteiro; sem teste de concorrência do código OAuth; erro ao atualizar "último uso" dá 500; clientes OAuth cadastrados nunca são limpos; quando o Ziv ganhar "desativar pessoa", é preciso revogar as `mcp_connections` dela, e `mcpConnection`, a chave de renovação e o código de acesso precisam checar que a pessoa está ativa (hoje só apagar a pessoa corta o acesso) |

### Etapa 2 — registrar apps prontos

| Tarefa | Status | Notas |
|---|---|---|
| Protocolo de registro pelo Claude | Done | `docs/PROTOCOLO-REGISTRO.md` v0.3 aponta para o texto do MCP (`formato_do_app`, `mcp_protocolo.md`) |
| Teste local do protocolo em 4 apps da Alis | In Progress | Rodada 1 (Eq. Research Files Prep, app extra) aprovada. Faltam: Python Rentabilidade fundos, Research Platform, transcritor-calls, CVM Downloader (decidir aí a relação com o Files Prep) |
| Formato v0.2 do manifesto | Done | Origem `window`, `multiple`, `depends_on`, `build`, todos opcionais; comando `cmd/validate-manifest` para validar rascunhos |
| Tela de revisão: mostrar/editar `depends_on`, `build` e várias escolhas | Done | Etapa 2 (2026-10-05): a tela de registro usa os mesmos editores da ficha |
| Ferramentas de registrar pelo Claude | Done | 2026-11-06, no ar desde 2026-11-09 (merge 07223cc), ADR 026. O Claude Code lê a pasta no PC, a pessoa confere na conversa, e o código vai por um endereço de uso único (`preparar_envio`). `formato_do_app` traz o protocolo, `registrar_app` registra scripts e apps com tela, ou manda uma versão nova de um app com código guardado. `publicar_app_com_tela` põe o app no ar como o botão Publicar. Valores de chaves só pelo site; versões "Código enviado · pelo Claude" no histórico |
| Teste com as mãos do Henry (5 cenários do desenho, §6) | In Progress | 2026-11-09: Streamlit no link próprio e atualizar passaram. Na empresa Teste: script novo e rodar; versão 2 sem app duplicado; pasta com dois programas; Streamlit no link próprio e atualizar; pasta com `.env` |

### Etapa 3 — criar apps para o Ziv · Etapa 4 — agendar

| Tarefa | Status | Notas |
|---|---|---|
| Criar apps com `ziv.yaml` pronto | Pending | Depois da Etapa 2 |
| Agendamento no Ziv | Done | Entrega 1 em 2026-11-06 (ADR 024); ver "Agendar apps" |
| Ferramentas de agendar pelo Claude | Pending | Depois do agendamento no Ziv |

## Fase 3 — Catálogo e ficha (1.2, 1.4)

| Tarefa | Status | Notas |
|---|---|---|
| Catálogo em cartões com busca e filtro por área/status | In Progress | Cartões, busca simples e botão Rodar no cartão prontos (Etapa 1 da UX); faltam filtros por área/status (Etapa 3) |
| Favoritos por pessoa | Pending | |
| Ficha do app (visualização e edição) | Done | Etapa 2 (2026-10-05): desenho B, "Editar" por parte, grava `app-hub.yaml` e `FICHA.md` e registra no histórico |

## Fase 4 — Execução (1.3), feature central

| Tarefa | Status | Notas |
|---|---|---|
| Agente do Ziv no Windows (instalar, conectar, ligar com o Windows) | Done | ADR 004. `scripts/build-agent.sh`; Meu computador e Conectar. Testado de verdade em 2026-09-30 (A-HKROPP01) |
| Rodar app com janela/`.exe` pelo Agente, com pasta escolhida | Done | Padrão: pasta do app; outra pasta: cópia temporária do `.exe`. Eq. Research Files Prep rodado de verdade em 2026-09-30 na pasta do app. Janela de pastas moderna (Agente 0.1.1): falta o Henry conferir |
| Resultados agrupados pelo manifesto | Done | Arquivos novos ou alterados; grupos pelas `outputs` + "Outros arquivos". Padrões aceitam `**`; grupos calculados pela ficha atual. Ficha do Files Prep corrigida após o 1º teste (`*/*/**/*.pdf`, `*_secoes.pdf`) |
| Baixar resultados pelo Agente (arquivo e .zip) | Done | Sob demanda; só arquivos da execução. Um arquivo, grupo inteiro, ou arquivos marcados ("Baixar selecionados", pedido do Henry). Erros viram página legível |
| Atualização automática do Agente | Done | 2026-09-30, testada no PC do Henry (0.2.0 → 0.2.1 sozinho; 0.2.2 com defeito voltou 3 vezes e o Ziv parou; 0.2.3 entrou). Para publicar: subir `agentproto.Version`, rodar `scripts/build-agent.sh` e reiniciar o Ziv. Sem número novo, nada atualiza. A 0.1.x precisa de uma última instalação manual |
| Ziv: ícone, janela própria e instalador único (B) | Pending | Decidido na ADR 005 (2026-09-30): o Agente vira o Ziv, mostrando as mesmas telas do site numa janela, com o Ziv sempre num servidor central. Etapas 2 e 3, antes do primeiro colega |
| Assinatura digital do Ziv | Pending | Etapa 4, antes de vender (certificado pago) |
| Pendências menores da atualização automática | Pending | "Atualizando…" aparece enquanto o Agente espera um app fechar (pode durar horas); espera pelo app não é cancelada se o Agente fechar; uma execução nova pode passar na frente da atualização; `install()` chama `/quit` sem limite de tempo; poucos testes para falha ao abrir a versão nova e para download 404/401; uma atualização bem-sucedida antiga pode virar "falha" se outra entrar em menos de 10 min (só histórico) |
| Pendências menores da revisão da Fase 4 | Pending | Cópia do `.exe` que sobra bloqueia a próxima execução; `entrypoint` com subpasta; pasta sem permissão interrompe a leitura; nomes com `..`; trava rara no download; trava de instância única por porta TCP; erro de uma consulta fica na tela; espera de 5 min na janela de pastas se o Agente caiu; apps em pasta do WSL; limpeza de pedidos de conexão vencidos |
| Registro das execuções | Done | Quem, quando, duração, pasta, computador, versão do app (data e tamanho do `.exe`); Interrompida quando o Agente reinicia ou o computador não responde |
| Executor genérico isolado da pasta original | Pending | Para scripts: roda numa cópia de trabalho; coleta arquivos novos como saídas |
| Ambiente reproduzível por app | Pending | Versão da linguagem e bibliotecas fixadas, preparado na 1ª vez |
| Formulário automático a partir do manifesto | Pending | Arquivo, data, número, texto, lista |
| Progresso ao vivo e resultado (texto, tabela, downloads) | Pending | |
| Apps web: subir e abrir; desktop/executável: abrir | Pending | |
| Repetir execução | In Progress | Apps com janela: **Rodar de novo** na mesma pasta, em Minhas execuções e na página da execução (Etapa 1 da UX). Scripts: falta repetir com as mesmas entradas |
| Rodar em poucos cliques (Etapa 1 da UX) | Done | Feito: Rodar no cartão, pasta lembrada por pessoa e app ("Lembrar esta pasta"), faixa de andamento embaixo (sobrevive a recarregar), Minhas execuções (50 por vez, "De todos" para admin), ficha abre sem esperar a pasta (ADR 007). Telas e teste de verdade com o Henry feitos em 2026-10-02 |

## Fase 5 — Versões (1.6)

| Tarefa | Status | Notas |
|---|---|---|
| Versão 1 no registro + detecção de mudança na pasta | Done | Versão 1 na primeira conferência; conferência ao abrir a ficha e o histórico, antes de rodar e ao publicar. ADR 006 |
| Histórico, comparação e voltar atrás | Done | Página da versão (O que mudou / Como estava); voltar com prévia, versão de segurança e lixeira. Testado de verdade no Files Prep (2026-10-01) |
| Histórico de mudanças na ficha | Done | Campo por campo, só registrado (não se restaura) |
| Linha do tempo única: código + ficha + execuções | Done | Seção Histórico na ficha, com filtro e "Carregar mais"; execução mostra a versão usada |
| Sobras da Fase 5 | Pending | Manifesto do Files Prep não lista os `.zip` que o app baixa (entram no histórico como binários); limpar versões antigas; página da versão: respostas fora de ordem ao trocar de versão, erro esconde a página inteira, sem teste de quem vê Descrever/Voltar; leitura travada do OneDrive dentro da trava do app pode segurar a ficha; erro ao gravar a versão depois de voltar devolve um erro genérico; a máscara de chaves esconde um pouco a mais (ex.: `token = settings.TOKEN`); um arquivo que some no meio da conferência faz a conferência falhar; duas restaurações no mesmo segundo usam a mesma lixeira; a primeira conferência lê todos os arquivos, inclusive os que estão só na nuvem |

## Fase 6 — Acesso e cofre (1.7)

| Tarefa | Status | Notas |
|---|---|---|
| Empresa e times (time = área; pessoa em uma empresa só) | In Progress | Áreas com líderes e membros feitas na Etapa 2 (ADR 009). Falta a empresa como conta, vários times com configurações próprias, limites e aprovação antes de publicar |
| Permissões por app (usar / editar) | Done | Etapa 2 (2026-10-05, ADR 009): cargos, líderes de área, acessos extras e app restrito. Parar um app aberto: feito e testado de verdade em 2026-10-28 (Agente 0.3.1) |
| Arquivos de senhas (`.env`) fora do histórico de versões | Done | 2026-10-26 (ADR 006). Migração 010 limpou as versões já guardadas |
| Cofre local criptografado + injeção na execução | Pending | Hoje as senhas dos apps que rodam no computador ficam no `.env` da pasta. Os scripts importados pelo navegador guardam seus arquivos no Ziv (`script_sources`) |
| Registro de uso de chaves | Pending | |

## Fase 7 — Saúde, continuidade e painel (1.8, 1.9, 1.10)

| Tarefa | Status | Notas |
|---|---|---|
| Status por app a partir das execuções | Pending | |
| Responsável/reserva obrigatórios + lista de apps em risco | Pending | |
| Painel: inventário, uso, risco | Pending | |

## Fase 8 — IA

| Tarefa | Status | Notas |
|---|---|---|
| Configurar chave da Claude API | Blocked | Decisão adiada pelo Henry |
| Diagnóstico e ficha escritos por IA | Pending | Resolvido pelo MCP (Fase 2b) para o registro; dentro do Ziv ainda depende da chave |
| Explicação de erros em português | Blocked | Depende da chave |
| Busca em linguagem natural, "pergunte ao app" | Blocked | Depende da chave |

## Fase 9 — MCP completo

| Tarefa | Status | Notas |
|---|---|---|
| MCP: criar app do zero, rodar, ler saúde, usar cofre | Pending | Depois das Fases 4–7; registro já coberto na Fase 2b |

## Apps compartilhados (2026-10-06, ADR 010)


| Tarefa | Status | Notas |
|---|---|---|
| Tasks 1 a 18 (manifesto, cofre, motor Podman, bancos por app, receitas, checagem, implantação, porteiro, vigia, API da ficha, uso e custo, catálogo, parte "Compartilhado" da ficha, página Servidor e gastos) | Done | Juntado à versão principal (`main`) em 2026-10-26, com todos os testes passando |
| Task 19: ADR 010, PRD, TASKS, INFRASTRUCTURE e script de fotos (`e2e/compartilhados.mjs`) | Done | As fotos só rodam depois que o Henry reiniciar o Ziv com o código novo |
| Teste de ponta a ponta com o Podman de verdade (2026-10-07, num Hub de teste separado) | Done | 12 de 12 passos passaram. Os 5 defeitos achados foram corrigidos: parada do app levava 10 s (o app não recebia o sinal de parar), o vigia religava depois de uma só checagem de saúde que falhava (agora pede 2 seguidas e confere de novo antes de religar), disco "usado" contava os blocos reservados, `PUT /api/server/cost` com corpo errado gravava zero, e a resposta de `POST /api/apps` vinha sem permissões nem versão |
| Cópias dos bancos criptografadas (2026-10-26) | Done | Com a chave do cofre, em pedaços autenticados; as cópias antigas são criptografadas quando o Ziv liga. Restaurar lê as duas. "Baixar" entrega a cópia já aberta (`pg_restore` lê); cópia danificada dá aviso em vez de arquivo quebrado |
| Task 19: teste de verdade com o Henry (Streamlit de exemplo, testar, publicar, descansar, voltar versão, custo, checagem do Sistema Alis V2) | Pending | Um passo por vez, conferindo pelos dados do Ziv |

### Depois
- Subir na nuvem (por último).
- Revisão de 2026-10-26, itens pequenos: o resumo das versões antigas ainda pode citar nomes como `.env.local` (o conteúdo já saiu); amarrar cada cópia ao seu app (identificador no cabeçalho); criptografar também arquivos `.dump` soltos na pasta sem registro no banco.
- A montagem da caixa ainda copia o `.env` da pasta para dentro da imagem. Tirar, e a checagem pedir que as senhas passem pelas configurações do app (cofre).
- Sistema Alis V2 para a equipe (antes, só a checagem).
- Linha "Versão nova pronta para publicar" na ficha (spec 4.3).
- Nome da versão no ar montado a partir da pasta publicada (hoje vem do número da implantação).
- Cofre: amarrar cada segredo ao seu app (dados associados por app) e permitir trocar a chave (`HUB_SECRET_KEY`).
- Uso do computador (`hostinfo`) fora do Linux: hoje a página Servidor e gastos mostra zero em outros sistemas.
- Fotos do porteiro (páginas "acordando", "entre de novo", "teste encerrado") no script `e2e/compartilhados.mjs`.
- Rodar os testes com `-race` num computador com `gcc`.
- Pequenas sobras guardadas na revisão (baixa prioridade):
  - Motor: `Fake` com `sync.Mutex` sobrando; `Inspect`/`Logs`/`Exec` aceitam caixa desconhecida; "no such image" vira "não encontrado"; motor fora do ar soma até 10 s por comando.
  - Bancos: testes do `appdb` mais firmes (erros ignorados, sem teste de senha errada); `Fake` aceitar só nomes válidos; bancos de teste também trancados; `LockDown` tira `CONNECT` de `PUBLIC` no banco do Ziv a cada início (avisar o Henry).
  - Receitas e checagem: subpasta no módulo (`src/app.py`), imports separados por vírgula, `requirements.txt` com comentários, `process.env["X"]`, arquivos acima de 1 MB ignorados sem aviso, porta 0 sem `EXPOSE`.
  - Montagem: `.dockerignore` simples (sem `**`); `Wait` pode passar do tempo em até 5 s.
  - Implantação: "Voltar para a anterior" não marca acesso (pode descansar logo); `Unshare` ignora erros ao remover caixa e imagem (registrar); `deploy.go` grande (separar); imagens de montagens que falharam ficam para trás.
  - Porteiro: `stripCookie` também tirar `hub_session`; vigia de WebSocket com tempo-limite; "fail closed" contar do primeiro erro; testes de não acordar em pedido cancelado e da página de teste encerrado.
  - Vigia: app que caiu depois de "Ligar" manual não é tentado de novo.
  - Telas: "Abrir" aparece também para app parado ou que caiu; ficha guarda informação velha sem avisar quando um GET falha depois; quem só vê o app recebe erro em vez da linha com "Abrir" se a primeira leitura falha; `hostinfo` com erro dá custo zero sem aviso (falta `host_ok`); testes de permissão e de "Ligar" na página Servidor e gastos.

## Portal multiempresa — correções da revisão (2026-11-03, ADR 015)

| Tarefa | Status | Notas |
|---|---|---|
| Apps compartilhados de volta na empresa principal | Done | Desligados nas outras empresas até ficarem por empresa |
| Limite de login por conta (não por rede) | Done | 10 senhas erradas por e-mail; 300 tentativas por endereço |
| Cadastro: endereços reservados, teto e cota diária | Done | `RESERVED_COMPANY_SLUGS`, `COMPANY_LIMIT`, `COMPANY_SIGNUPS_PER_DAY` |
| Apps em uso aprovados uma vez (migração 015) | Done | Conferido no banco local: o app da Alis ficou aprovado |
| Deploy do Replit e empresas criadas em outra cópia do servidor | Done | Confirmar na primeira publicação |
| Teste de credencial um por vez; auditoria sem anônimos | Done | |
| Testes das telas estáveis (304/304) | Done | |
| Cadastro sem campo de endereço | Done | O servidor gera o endereço do nome da empresa; se já existe ou é reservado, usa o próximo número (`horizonte-2`) |
| Confirmação de e-mail no cadastro | Pending | Precisa de SMTP |
| Limite de login compartilhado entre cópias do servidor | Pending | Hoje é por cópia |
| Apps compartilhados por empresa | In Progress | Código feito em 2026-11-06 na branch `etapa-2-registrar`: nomes de containers, redes, bancos e pastas por empresa; endereço `app.empresa.dominio`; chave liga/desliga por empresa no Admin da principal. Falta: Caddy com os endereços por empresa, ADR 025, deploy e conferir o isolamento na VM |

## Agente sem atrito (2026-11-03, ADR 016)

| Tarefa | Status | Notas |
|---|---|---|
| Agente das empresas em um EXE só, com o endereço dentro | Done | Sem ZIP; `ZivAgente-<empresa>.exe` |
| Tela "Conecte seu computador" logo após o cadastro | Done | `/instalar-agente`; acompanha a conexão ao vivo; dá para pular |
| Aviso de computador desconectado (faixa + diálogo no Rodar) | Done | "Sem o Agente, não será possível rodar os apps do catálogo." |
| Começar sem a etapa do nome da empresa | Done | 4 etapas |
| Teste no Windows do EXE com endereço dentro | Pending | Precisa recompilar o Agente |

## Aba Admin no lugar de Segurança (2026-11-03, ADR 017)

| Tarefa | Status | Notas |
|---|---|---|
| Letras do app um pouco menores | Done | Base de 15px |
| Painel Admin: apps por área com uso e situação, computadores, quem mais usa, atividade | Done | `/admin`, `GET /api/admin/overview` |
| Aprovar / suspender / reativar na página do app | Done | `PUT /api/apps/{id}/control`; quem não administra vê só o motivo |
| Controles da empresa: pausar todos, exigir aprovação, apagar histórico | Done | No fim do Admin |
| Aba Segurança removida (`/seguranca` leva ao Admin) | Done | Classificação, domínios e exportação ficam só no servidor |
| Cofre de credenciais escondido | Pending | Volta quando a chave `CREDENTIAL_VAULT_KEYRING` for configurada e um app precisar |
| "Bloquear segredos embutidos" não tem efeito | Pending | A opção é salva mas nada a usa; decidir se implementa ou remove |

## Teste com os apps de teste e tipos de arquivo aceitos (2026-11-03)

| Tarefa | Status | Notas |
|---|---|---|
| Teste manual dos 7 apps de teste na empresa Teste | Done | Conexão, rodar, suspensão, erro, Parar, número, data e arquivo ok no Windows |
| Tipos de arquivo aceitos por campo de arquivo (`accept`) | Done | A janela de escolha filtra; o Ziv recusa outros tipos no navegador e no servidor; aparece na ficha |
| Contador de palavras recusa PDF, Word e Excel | Done | `exemplos/apps-de-teste`; o importador já envia `accept` |
| Python no Windows quebrava ao escrever ✓, emoji ou letras fora do cp1252 | Done | Agente 0.3.3 usa `PYTHONIOENCODING=utf-8`; o registro `.apphub-console.txt` sai em UTF-8 |
| Atualização automática do Agente de uma empresa (`/w/<empresa>`) falhava e voltava atrás | Done | O Ziv mandava o EXE sem o endereço da empresa; agora a atualização leva o mesmo final de arquivo do download. Testado no Windows: 0.3.2 → 0.3.3 em 5 s |
| Ao reabrir um app importado depois de o PC reiniciar, duas leituras ao mesmo tempo recriam a pasta e uma dá erro | Pending | Visto no log (`restore imported working copy`); some na segunda leitura; corrigir com trava |
| Nome da subpasta de resultados ("Execucao-N") | Pending | Henry: depende de cada app; nada decidido |
| Botão Baixar na faixa de andamento | Done | Quando o app termina com arquivos, a faixa mostra "Baixar" (1 arquivo, vem ele mesmo) ou "Baixar tudo" (vários, num .zip), além de "Ver arquivos". Rota `GET /api/runs/{id}/download`. Traz só o resultado: o registro do console (`.apphub-console.txt`) fica de fora, a não ser que seja o único arquivo |

## Artifacts (2026-11-03, ADR 018)

| Tarefa | Status | Notas |
|---|---|---|
| Artifacts: link ou arquivo `.html`, etiquetas, busca, versões, acesso como nos apps, sem aprovação | Done | `/artifacts`; HTML numa caixa isolada; conferência visual em `e2e/artifacts.mjs` |
| "Exigir aprovação" começa desligado em empresa nova | Done | Empresas que já existiam mantêm a opção |
| Aba Home | Done | Ver a seção "Home" abaixo (ADR 019) |
| Correções da revisão final: prévia recarrega na versão nova, acessos de área técnicos/execuções não alcançam artifacts, "Editar quem cuida" (responsável, reserva, área), busca no celular, trava na versão nova | Done | 2026-11-03 |
| Link que aceita frame aparece dentro da página; link do Claude sugere subir o HTML | Done | 2026-11-03, pedido do Henry após o teste; `artifact_versions.embeddable` (migração 017) |
| Publicar artifact pelo MCP | Done | 2026-11-04, Etapa 1 do MCP (ADR 021) |
| Pendências pequenas dos Artifacts | Pending | Sem teste de que empresa antiga mantém a aprovação; grant de restaurar e `see_tech_any_app` sem teste; lista de artifacts duplica `artifactRef`; PUT troca resumo e etiquetas (o front manda a ficha toda); URL do link sem limite de tamanho; permissão duplicada dá 500; blobs órfãos em `code_blobs` nunca limpos; sem teste de `setArtifactGrants`; dicas e ARIA das abas da tela nova; página não reinicia ao trocar de `:id`; sem teste de editar Sobre, rolar para `#acesso` e apagar |

## Home (2026-11-04, ADR 019)

| Tarefa | Status | Notas |
|---|---|---|
| Favoritos e registro de aberturas de artifacts | Done | Migração 018 (`favorites`, `artifact_opens`); `PUT/DELETE /api/favorites/{kind}/{id}`; abertura registrada no máximo a cada 30 min |
| `GET /api/home` (Meu dia): atenção, favoritos, recentes, últimas execuções | Done | Tudo passa pela checagem de acesso; favorito que virou restrito ou foi apagado some sem erro |
| Home esconde execuções de apps que a pessoa não vê | Done | Correção da revisão da tarefa 2 |
| `GET /api/home/team` (Meu time) | Done | Administrador = empresa toda; líder = soma das áreas; os demais recebem 404 |
| Estrela de favorito e catálogo em `/apps` | Done | `/` passa a ser a Home; "Apps" fica marcado em `/apps` e `/apps/:id` |
| Página Home: Meu dia e Meu time | Done | Partes vazias somem, menos "Favoritos" |
| Conferência visual e documentação | Done | `e2e/home.mjs` (claro, escuro, celular 390 px, pessoa sem nada); PRD 3.15; ADR 019 |
| "Apps e artifacts sem responsável ativo" em Pendências de gestão | Pending | Volta quando existir "desativar pessoa" |

## Visual preto e branco (2026-11-04, ADR 020)

| Tarefa | Status | Notas |
|---|---|---|
| Tokens neutros do `DESIGN.md` de volta; coral só em Rodar, "+ Registrar apps", marca da navegação ativa, seleção e indicadores ao vivo | Done | Revertida a deriva creme/petróleo de 2026-10-30 (commit 942c2d9) |
| Páginas públicas no visual neutro com `AuthLayout` | Done | `/boas-vindas`, `/entrar`, `/cadastre-se`, `/acesso`, `/convite`; catálogo de exemplo honesto, sem produto falso |
| Palavras simples ("Hub" vira "Ziv", "Escolher arquivos", "Endereço da empresa no Ziv") | Done | |
| Erros com próximo passo (Conectar e mensagem geral) | Done | "Algo deu errado. Tente de novo; se continuar, avise quem administra o Ziv." |
| Conferência visual | Done | `e2e/visual-neutro.mjs` (somente leitura; 1440 e 390 px; claro e escuro em Home, Apps e Entrar); sem defeito novo |
| Banners demais e faixa de execução velha | Pending | Crítica de 2026-11-04 (22/40): empilham no topo e no rodapé |
| Fonte base 14px e Home mais limpa | Done | Linhas compactas em vez de cartões; Home em duas colunas (favoritos e recentes; últimas execuções); avisos do mesmo computador aparecem uma vez e o de computador desconectado some quando a faixa do topo já avisa; "O que estão criando" e pendências mostram 5 |
| Home sem botão Rodar | Pending | Favoritos e recentes só abrem a ficha |
| Nomes duplicados no catálogo | Pending | |
| Tela Admin muito longa | Pending | |
| Botão de voltar versão | Pending | Hoje a volta de versão está escondida |

## Texto da landing page (2026-11-06)

| Tarefa | Status | Notas |
|---|---|---|
| Abertura falando do risco: "As ferramentas que sua equipe cria com IA viram ferramentas da empresa." | Done | Público principal: dono/sócio |
| Parte de baixo em 4 passos com telas de exemplo (ficha, Rodar, versões, documentos) | Done | Substitui os 5 blocos de texto; exemplos marcados como ilustrativos |
| Botões "Criar o espaço da empresa" e aviso para convidados usarem Entrar | Done | Evita que um convidado crie uma segunda empresa |
| Fontes menores, avisos em 12 px, margem de 16 px no celular | Done | Crítica impeccable + ux-copy: 23/32 |
| Passo próprio para o sócio ("Você sabe o que existe e quem cuida") | Pending | Sugerido na crítica; Henry não escolheu nesta rodada |

## Agendar apps (2026-11-06, ADR 024)

| Tarefa | Status | Notas |
|---|---|---|
| Tabela de agendamentos, status Perdida e origem "agendada" | Done | Migração 020 |
| Cálculo de horários em Brasília (minutos, horas, dia, dias da semana, mês) | Done | `internal/schedule` e `lib/schedules.ts` |
| Rodar e agendamento pelo mesmo caminho de checagens | Done | `launchRun` |
| API de agendamentos e quem vê o quê | Done | Dados de outra pessoa ficam escondidos |
| Relógio que dispara, Perdida, Pulada, espera e pausas automáticas | Done | 5 falhas seguidas pausam; atraso de até 1 h |
| Telas: Agendar, seção Agendamentos, selo "agendada", aviso na Home | Done | `e2e/agendar.mjs` (1366 e 390 px) |
| Teste do Henry com o Agente ligado | Pending | Agendar a cada 5 min e ver as execuções; desligar o PC e ver Perdida |
| Peça 2: rodar no servidor | Pending | Para apps que não precisam do PC (ex.: News Scrapper) |
| Peça 3: e-mail opcional por agendamento | Pending | Precisa escolher um serviço de envio |
| Faixa de execuções abertas sem as agendadas que já terminaram | Done | Revisão final; ficam no histórico |

## Ficha em duas faces: Usar e Cuidar (2026-11-09, ADR 029)

| Tarefa | Status | Notas |
|---|---|---|
| 5 opções de organização da página do app | Done | Henry escolheu misturar "Usar · Cuidar" com "Menu de seções" (opção 6) |
| Faces Usar e Cuidar, menu de seções, Pendências com etapas | Done | `CareMenu.tsx`; quem cuida abre em Cuidar |
| Um coral só na página (Publicar e Abrir do servidor neutros) | Done | |
| Testes da página atualizados e novos (faces, pendências, aprovação) | Done | |
| Publicado em zivapp.duckdns.org | Done | 2026-11-09 |
| Teste do Henry na página de um app real | Pending | Script e app de navegador |

## Landing nova no estilo da Pluggy (2026-11-09, ADR 030)

| Tarefa | Status | Notas |
|---|---|---|
| Prévias A, B, C e variações até a B5 | Done | Henry escolheu B5: Poppins + destaque em Instrument Serif, escala de tamanhos da Pluggy |
| Topo: título curto e desenho animado (apps espalhados → Ziv → áreas) | Done | Versão em pé no celular; animação desliga com "reduzir movimento" |
| Faixa de ferramentas rodando numa linha (Claude, ChatGPT, Gemini, Python…) | Done | Logos do simple-icons guardados no app |
| Blocos Catalogado e Documentado com telas de exemplo | Done | Marcadas como exemplo ilustrativo |
| Como funciona em 4 passos verticais, com uma imagem que acompanha o passo | Done | Avança sozinho a cada 3,5 s; clicar escolhe o passo |
| Fontes servidas pelo próprio app (sem Google Fonts) | Done | `@fontsource/poppins`, `@fontsource/instrument-serif` |
| Publicado em zivapp.duckdns.org | Done | 2026-11-09 |
| Logos de Excel e Windows na faixa | Pending | Não existem no simple-icons; precisariam de outra fonte |

## Arquivar, Configurações e Execuções (2026-11-09, ADRs 031, 032, 033)

| Tarefa | Status | Notas |
|---|---|---|
| Arquivar app: botão no pé da página do app, para quem controla o acesso (responsável, reserva, líder da área) | Done | Some do catálogo, da Home, da busca e do Claude; não roda; agendamentos pausados; versão no servidor parada |
| Apps › Arquivados com "Trazer de volta"; apagado de vez depois de 30 dias | Done | Limpeza de hora em hora; a pasta no computador nunca é mexida |
| Aba Configurações (junta o Cofre e as configurações do servidor) | Done | Blocos "Chaves e senhas" e "Outros ajustes"; os mesmos valores valem no computador e no servidor |
| Pendência "Preencher as configurações" quando falta valor obrigatório | Done | |
| Aba Execuções na face Cuidar, com "Rodando agora" e relógio | Done | |
| "Rodando agora" em Minhas execuções e na Home | Done | Só as execuções da própria pessoa |
| Página da execução: o que foi preenchido, tempo correndo e registro do app | Done | Valores com cara de senha não são guardados |
| Publicado em zivapp.duckdns.org | Done | 2026-11-09 |
| Teste do Henry no Ziv publicado | Pending | |

## Navegador na receita automática (2026-11-12, ADR 040)

| Tarefa | Status | Notas |
|---|---|---|
| Receita Python com Playwright traz Chromium, bibliotecas, Xvfb e libatomic1 | Done | Base bookworm; passo do navegador compartilhado entre apps; montado de verdade e o Chromium abriu com janela em 512 MB |
| App com navegador recebe pelo menos 512 MB, mesmo com teto da empresa menor | Done | |
| Checagem antes de publicar: "Navegador ok" no lugar do aviso | Done | Selenium, Puppeteer e Playwright em Node ainda avisam |
| Guia do MCP: Playwright em Python vem pronto | Done | |
| Memória somada no servidor: 512 → 768 MB (`SHARED_APPS_MEMORY_BUDGET_MB`) | Done | Um app com navegador + um app comum |
| Correções da revisão de código (9 pontos) | Done | 2026-11-13: memória ao republicar, aviso de memória, piso de 512 visível, pasta ilegível, Playwright travado em 1.64.1 (ou o do app), `--with-deps` no 2º passo, `playwright-stealth` não conta, Selenium junto com Playwright ainda avisa. Imagem montada e Chromium abriu |
| Publicar no zivapp e republicar o app de relatórios (BTG/BBA) | Pending | O app deve parar de instalar o navegador sozinho |
| Playwright em Node e Selenium | Pending | Depois |
