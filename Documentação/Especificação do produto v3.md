# Especificação do produto — v3

06/10/2026 · @Henry · substitui a v2 de 30/09/2026 (guardada em `Especificação do produto v2.md`; a v1 de 29/09 segue em `Especificação do produto.md`)

## O que mudou em relação à v2

A v3 incorpora a primeira semana de piloto na Alis. A visão do produto não mudou. Mudaram três coisas: o desenho das permissões (cargos e áreas em vez de papéis fixos por app), o jeito de rodar (poucos cliques, sem pasta padrão) e a arquitetura (a plataforma sempre num servidor central, com o Agente crescendo até virar o App Hub no computador).

| Onde | Mudança | Por quê |
| --- | --- | --- |
| Módulo 1 | Princípios de produto escritos (poucos cliques, confiança visível, erro sempre com o próximo passo) | Guiar as decisões de tela daqui em diante |
| 1.2 | Ficha numa página só: primeiro o uso, depois um bloco recolhido "Para quem cuida do app"; "Editar" em cada parte | Quem só usa não precisa ver a parte técnica |
| 1.3 | Rodar no cartão do catálogo, faixa de andamento embaixo da tela, Minhas execuções e Rodar de novo | Usar um app em um ou dois cliques, sem sair da página |
| 1.3 | Sem pasta padrão: a pessoa escolhe onde salvar na primeira vez e a plataforma lembra | Decisão do Henry (02/10): gravar na pasta do app sujava a pasta original |
| 1.3 | Apps **pessoais** e **compartilhados** (em aberto) | Alguns apps (Streamlit com dados comuns) não podem abrir uma cópia por pessoa |
| 1.3 | O Agente vira o **App Hub**: ícone perto do relógio, janela própria e instalador único | Cara de produto sem manter dois conjuntos de telas (decisão de 30/09) |
| 1.6 | Versões nascem sozinhas quando a plataforma confere a pasta; só o código-fonte tem cópia; voltar atrás tem prévia, versão de segurança e lixeira | Pastas no OneDrive com GBs de saídas e arquivos "só na nuvem" |
| 1.7 | Papéis fixos por app viram **cargos personalizáveis**, **áreas com líderes** e **acessos extras**; app **aberto** ou **restrito** | Na prática, quem cuida dos apps de uma área é o líder dela, não um papel por app |
| 1.7 | Regras contra escalada: ninguém dá cargo maior que o seu; sempre há alguém que cuida de pessoas e cargos | Revisão de segurança da Etapa 2 |
| 2.1 | A plataforma fica **sempre num servidor central** (nossa nuvem ou um computador do cliente); o modo híbrido detalhado | Compartilhamento e continuidade exigem um lugar só |
| 2.7 | Plano Local = plataforma num computador do cliente sempre ligado | Consequência da plataforma central |
| Piloto | Situação atualizada em 06/10 e proposta de nova ordem de prioridades | Registrar o andamento real |

## Introdução

Este documento especifica um produto que transforma os apps que pequenas empresas criam com IA em ferramentas da empresa: organizadas, usáveis por qualquer pessoa e vivas mesmo quando quem criou sai. Ele se divide em dois módulos: o Módulo 1 (gestão e orquestração dos apps) e o Módulo 2 (a nuvem onde eles rodam).

### O contexto

Com IAs como Claude, ChatGPT e Cursor, qualquer funcionário consegue criar um app em minutos: um extrator de dados de PDF, um resumo de notícias, um gerador de relatório. Isso já acontece em escala no Brasil: 21,6% dos executivos brasileiros usam plataformas de vibe coding, contra 15,6% nos EUA ([Jitterbit, via Times Brasil/CNBC](https://timesbrasil.com.br/colunas/ai-451/brasil-lidera-automacao-ia-vibe-coding-tropecos-jitterbit-2026/)).

Criar ficou fácil. O que continua difícil é tudo o que vem depois. O app nasce numa pasta, no computador de quem criou, e depende de Python instalado, das bibliotecas certas, de um comando no terminal e de chaves de API guardadas em algum lugar. Na prática, só quem criou consegue usar.

### O caso que originou o produto

Na Alis Investimentos, uma gestora de 4 pessoas sem TI, a equipe já tem vários apps criados com Claude, para extrair dados de documentos, acompanhar notícias e outras tarefas. Cada app fica numa pasta separada. Ninguém sabe rodar os apps dos outros, e se alguém sair da empresa, os apps dessa pessoa morrem junto. Um deles roda só localmente e não pode ser usado de casa.

### As dores

1. **Continuidade:** o app depende de quem criou.
2. **Compartilhamento:** colegas não conseguem usar os apps uns dos outros.
3. **Acesso:** o app só funciona naquele computador.
4. **Manutenção:** apps feitos com IA quebram, e só quem criou conserta.
5. **Segurança:** senhas e chaves espalhadas em pastas e máquinas.
6. **Retrabalho:** duas pessoas criam a mesma solução sem saber.
7. **Falta de padrão:** cada app é feito de um jeito, em uma linguagem.

### A solução em uma frase

Uma plataforma onde PMEs sem TI registram, organizam, rodam e mantêm os apps que criam com IA, com a IA fazendo o trabalho técnico por trás: ler o código, documentar, gerar a tela, corrigir e, no Módulo 2, hospedar.

### Para quem

PMEs brasileiras de até 50 pessoas, sem TI próprio, que já criam apps com IA. O primeiro cliente é a própria Alis, num piloto do Módulo 1 rodando localmente, sem nuvem.

## Módulo 1 — Gestão e orquestração de apps

O Módulo 1 é o produto de verdade: a camada que o cliente vê e onde está o diferencial. Ele transforma "um script na pasta do Henry" em "uma ferramenta da empresa", com dono, documentação, tela de uso e histórico. Funciona sozinho, rodando os apps no computador de quem usa; o Módulo 2 depois leva essa execução pra nuvem.

**Objetivo:** qualquer pessoa da empresa encontra, entende e usa qualquer app, sem saber programar e sem depender de quem criou.

**Princípio central: a plataforma é genérica.** Ela não é feita sob medida para os apps de um cliente. Qualquer app, de qualquer tipo, entra pelo mesmo caminho: ganha um **manifesto** (a "carteira de identidade" do app, que diz como ele roda, o que recebe e o que devolve) e, a partir dele, ficha, tela de uso e histórico. Apps registrados à mão ou criados pela IA via MCP (1.11) usam o mesmo manifesto.

**Princípios de produto**

1. **Rodar em poucos cliques;** detalhes só para quem quiser aprofundar.
2. **Quem só usa nunca precisa entender** código, pastas técnicas ou o manifesto.
3. **Confiança visível:** sempre deixar claro qual versão rodou, onde os arquivos foram salvos e o que mudou.
4. **Genérico antes de sob medida:** nenhuma tela feita para um app específico.
5. **Erro sempre vem com o próximo passo,** em português claro.

O sucesso tem duas metades, com o mesmo peso: a pessoa **resolve rápido e sozinha** (achou, rodou, pegou os arquivos sem pedir ajuda) e **confia no resultado** (sabe que rodou a versão certa e que os arquivos são os certos).

| Feature | Resolve | Pra quem |
| --- | --- | --- |
| 1.1 Registro e publicação | Tirar o app da pasta e colocar na plataforma | Criador |
| 1.2 Ficha do app | Saber o que o app faz e como usar | Todos |
| 1.3 Execução com interface automática | Usar sem terminal nem Python | Usuário |
| 1.4 Catálogo e descoberta | Achar o que já existe e evitar retrabalho | Todos |
| 1.5 Padrões e modelos | Apps feitos do mesmo jeito | Criador |
| 1.6 Versões e histórico | Saber o que mudou e voltar atrás | Criador, sócio |
| 1.7 Acesso, permissões e cofre | Quem usa o quê; chaves protegidas | Sócio |
| 1.8 Saúde e manutenção com IA | App quebrado detectado e corrigido | Criador |
| 1.9 Continuidade e responsáveis | Nenhum app morre quando alguém sai | Sócio |
| 1.10 Painel de governança | Visão de tudo o que existe e do risco | Sócio |
| 1.11 Servidor MCP | Criar e manter de dentro da IA que o criador já usa | Criador |

### 1.1 Registro e publicação de apps

O criador entrega uma pasta, e a plataforma devolve um app pronto pra ser usado pela empresa. É a porta de entrada de tudo: sem registro, o app continua invisível.

**Como funciona**

1. O criador escolhe a origem: arrastar a pasta, apontar uma pasta compartilhada, conectar um repositório (GitHub) ou pedir à IA pelo MCP (1.11).
2. A IA lê o código e monta um diagnóstico: linguagem, tipo de app, ponto de entrada, bibliotecas, entradas esperadas, saídas geradas e chaves usadas. Sem IA configurada, um diagnóstico por regras faz o mesmo papel, com mais perguntas.
3. A plataforma mostra o diagnóstico em português e pede confirmação só do que ficou incerto ("este campo é uma data ou um texto?").
4. Gera o manifesto (`app-hub.yaml`), a ficha (1.2, `FICHA.md`) e a tela de uso (1.3). Os dois arquivos ficam dentro da pasta do app; a plataforma nunca altera o código.
5. O criador define responsável, reserva e quem pode usar (1.7, 1.9) e publica.

**Tipos de app suportados**

| Tipo | Exemplo | Suporte |
| --- | --- | --- |
| Script com entrada e saída | Extrai tabelas de um PDF e gera Excel | Completo, com tela automática |
| App com tela própria (web) | App Streamlit ou página web | Completo, abre a tela do app |
| App de desktop | Programa com janela própria (tkinter, PyQt) | Completo, abre a janela e recolhe os arquivos gerados |
| Executável pronto | Um `.exe` gerado a partir de um script | Completo, abre e recolhe os arquivos gerados |
| Tarefa recorrente | Resumo de notícias todo dia às 7h | Completo, com agendamento |
| Planilha com macro | Excel com VBA | Registro e ficha; execução no Excel |
| Sistema complexo | Vários serviços e banco de dados | Registro e ficha; execução fora do escopo inicial |
| Documento, não app | Instruções, prompts, modelos em `.docx` | Só catálogo e ficha |

**Como o app recebe a entrada** também varia e entra no manifesto: arquivo com nome fixo ao lado do código, janela de seleção do Windows, a própria janela do app, pergunta na tela ou parâmetro no comando. Um app pode depender de outro (`depends_on`) e pode ter um passo de preparo (`build`), como gerar o `.exe` a partir do script.

**Detalhes**

- **Detecção de segredos:** se a IA acha senha ou chave escrita no código, avisa, move pro cofre (1.7) e troca a linha por uma referência.
- **Ambiente reproduzível:** a plataforma fixa versão da linguagem e das bibliotecas, para o app rodar igual em qualquer máquina.
- **Detecção de duplicados:** antes de publicar, compara com o catálogo (1.4) e com os outros apps do mesmo lote, e avisa se já existe algo parecido.
- **Registro em lote:** varre uma pasta com vários apps e registra todos de uma vez, cada um com seu diagnóstico. Pastas com cara de material antigo ("Old", "backup") vêm marcadas para pular.
- **Registro pela IA:** o criador pode pedir ao Claude que registre os apps de uma pasta (1.11).
- **Não destrutivo:** a plataforma só adiciona o manifesto e a ficha à pasta, e se recusa a sobrescrever um arquivo com o mesmo nome que ela não criou.
- **Nada é perdido:** o código original fica guardado como versão 1 (1.6).

**No piloto da Alis:** registro em lote da pasta onde ficam os apps, rodando localmente, com os manifestos salvos junto de cada app. Diagnóstico por regras, testado nos 19 apps da pasta de programação; registro pelo Claude com o protocolo em teste, pausado.

### 1.2 Ficha do app (documentação automática)

Cada app ganha uma página escrita pela IA, em português simples, que responde "o que é, pra que serve e como uso". Sozinha, ela resolve boa parte da continuidade: quem chega depois entende o app sem perguntar a ninguém.

**O que a ficha mostra**

| Campo | Exemplo | Origem |
| --- | --- | --- |
| Nome e resumo | "Extrator de ITR: puxa DRE e balanço do PDF da CVM" | IA, editável |
| Como usar | Passo a passo em 3 a 5 linhas | IA, editável |
| Entradas | Um PDF; um ticker (texto) | Leitura do código |
| Saídas | Uma planilha Excel com duas abas | Leitura do código |
| Exemplo | Uma execução real, com entrada e resultado | Última execução bem-sucedida |
| Responsável e reserva | Henry; reserva: outro colega | Criador (1.9) |
| Dependências | Python 3.12, pdfplumber, chave da API X | Leitura do código |
| Status e uso | Funcionando; rodou 14 vezes no mês | Execuções (1.3, 1.8) |
| Versão | v3, alterada há 5 dias por Henry | Histórico (1.6) |

**Como a página se organiza**

- **Uma página só, primeiro o uso:** o que o app pede, o que ele entrega, as execuções e quem cuida dele.
- **No fim, um bloco recolhido "Para quem cuida do app":** pontos de atenção, outros apps de que depende, código-fonte, chaves, bibliotecas, histórico (1.6), onde o app está e quem tem acesso (1.7). Só aparece para quem pode ver a parte técnica.
- **"Editar" em cada parte,** discreto. Uma parte em edição por vez; se outra pessoa salvou antes, a plataforma pede para recarregar em vez de sobrescrever.
- **Mostra pouco e oferece mais:** pontos de atenção mostram 3 e "Ver mais"; o histórico mostra os mais recentes e "Ver histórico completo".

**Detalhes**

- **Atualiza sozinha:** quando o código muda, a IA revisa a ficha e destaca o que mudou.
- **Texto humano tem prioridade:** se alguém editou um campo à mão, a IA sugere mudanças mas não sobrescreve.
- **Pergunte ao app:** um campo de perguntas ("funciona com ITR de banco?") respondido pela IA a partir do código e da ficha.
- **Exportação:** a ficha vira um arquivo de texto guardado junto do app, que continua legível mesmo fora da plataforma.

**No piloto da Alis:** ficha gerada para cada app registrado, salva como arquivo ao lado do código e editável na própria página, parte por parte (pronta em 05/10). A tela de registro usa os mesmos editores. Sem IA, o texto nasce de um modelo preenchido pelo diagnóstico por regras, ou é escrito pelo Claude via MCP (1.11).

### 1.3 Execução com interface automática

Esta é a feature central do produto: qualquer app registrado ganha um botão "Rodar" e uma tela de uso, mesmo que tenha sido escrito como um script de terminal. É o que permite a um colega que não programa usar o app de outro.

**Como funciona**

1. A partir das entradas detectadas em 1.1, a plataforma gera um formulário: campo de arquivo, data, número, texto, lista de opções.
2. O usuário preenche e clica em Rodar.
3. A plataforma prepara o ambiente do app (versão da linguagem, bibliotecas, chaves do cofre), sem o usuário ver nada disso.
4. Mostra o progresso e, no fim, o resultado: tabela na tela, texto, gráfico ou arquivo pra baixar.
5. Registra a execução: quem rodou, quando, com quais entradas, quanto tempo levou e se deu certo.

**Tipos de entrada e saída**

| Entrada detectada | Vira na tela | Saída detectada | Aparece como |
| --- | --- | --- | --- |
| Caminho de arquivo | Campo de upload | Arquivo gerado (Excel, CSV, PDF) | Botão de download |
| Data | Seletor de data | Tabela | Tabela navegável |
| Número | Campo numérico | Texto impresso | Caixa de texto |
| Lista fixa de valores | Menu de opções | Gráfico | Imagem |
| Texto livre | Campo de texto | Erro | Mensagem explicada pela IA (1.8) |

**Detalhes**

- **Apps com tela própria** (Streamlit, página web): a plataforma sobe o app e abre a tela dele, sem gerar formulário.
- **Apps de desktop e executáveis:** a plataforma abre a janela do app, espera a pessoa usar e fechar, e mostra os arquivos novos ou alterados, agrupados pelas saídas da ficha, mais "Outros arquivos".
- **Onde salvar:** não há pasta padrão. Na primeira vez de cada pessoa em cada app, a plataforma pergunta onde gravar os resultados (janela de pastas do Windows ou caminho digitado). A caixinha **Lembrar esta pasta** faz as próximas vezes abrirem direto, com um clique. A pasta aparece como "Salva em … · trocar" no cartão e na ficha. O app original nunca é alterado: se a pasta escolhida não é a do app, a plataforma abre uma cópia temporária do executável a partir dela.
- **Scripts rodam numa cópia de trabalho** (a fazer), para não sujar a pasta original.
- **Baixar os resultados:** um arquivo, um grupo inteiro (.zip) ou só os arquivos que a pessoa marcou.

**Rodar em poucos cliques**

- **Rodar no cartão do catálogo:** a pessoa não precisa abrir a ficha. Com a pasta lembrada, é um clique.
- **Faixa de andamento** embaixo da tela: "Abrindo…", "aberto no seu computador", "terminou: N arquivos novos · Ver e baixar", "sem arquivos novos" (normal, em cinza) ou o erro com o próximo passo. A faixa vem da plataforma: aparece igual depois de recarregar ou em outra aba, até a pessoa dispensar. Mostra no máximo 3 cartões (1 no celular), com prioridade para erros e resultados.
- **Minhas execuções:** a lista das execuções da pessoa, com o número de apps abertos agora, "Ver e baixar" e **Rodar de novo** na mesma pasta. Quem tem permissão vê também as execuções de todos.
- **Cancelar e Parar:** dá para cancelar antes de o app abrir. **Parar** um app já aberto pela plataforma está a fazer.

**Apps pessoais e compartilhados (em aberto)**

Alguns apps são usados por várias pessoas ao mesmo tempo e precisam dos mesmos dados (um sistema em Streamlit, uma plataforma de research). Esses não podem abrir uma cópia no computador de cada um. A ideia, a decidir antes de o primeiro colega usar um app assim:

- Opção no manifesto: **pessoal** (abre no computador de quem clicou) ou **compartilhado** (um só para todos).
- O compartilhado roda uma vez só no servidor central (2.1). Todos abrem o mesmo endereço, e a plataforma mantém o app no ar e mostra o link na ficha.
- Um banco de dados só, ao lado do app, com a senha no cofre (1.7).
- Aviso no diagnóstico quando um app compartilhado guarda dados em arquivo solto (planilha, CSV): duas pessoas gravando juntas podem perder dados.
- **Execuções salvas:** dá pra repetir uma execução anterior com as mesmas entradas, ou salvar um preset ("ITR trimestral").
- **Encadeamento:** a saída de um app pode ser a entrada de outro (extrair PDF → atualizar modelo), montando um fluxo.
- **Agendamento local:** rodar num horário enquanto o computador estiver ligado; na nuvem, isso vai pro Módulo 2 (2.3).
- **Onde roda:** no Módulo 1, no computador de quem clica. O ambiente é preparado automaticamente na primeira vez e reaproveitado nas seguintes.

**O Agente do Hub**

Quem abre o app no computador da pessoa é o **Agente do Hub**, um programa pequeno e invisível instalado uma vez em cada computador.

- **Instala e conecta uma vez:** a pessoa baixa o Agente na tela "Meu computador", ele abre o navegador e ela clica em Conectar, sem digitar código. Depois disso, ele liga sozinho com o Windows.
- **Quem começa a conversa é o Agente:** ele pergunta à plataforma se há algo para fazer. Por isso atravessa firewall e funciona igual com a plataforma local ou na nuvem (Módulo 2).
- **Cada computador tem sua própria chave**, e desconectar um computador corta o acesso na hora.
- **Os resultados ficam no computador:** a plataforma guarda só a lista. Ao clicar em Baixar, o Agente envia o arquivo na hora, e só arquivos daquela execução. Para baixar, o computador precisa estar ligado.
- **Atualiza sozinho:** quando sai uma versão nova, o Agente baixa e se troca sem a pessoa fazer nada, nunca no meio de uma execução. Se a versão nova não funcionar, a anterior volta sozinha; depois de 3 falhas, a plataforma pede para baixar de novo.
**O App Hub (o Agente com rosto)**

O Agente cresce e vira o **App Hub**, o programa da plataforma no Windows:

- Mostra a plataforma numa **janela própria**, com as mesmas telas do site (sem telas separadas para manter).
- **Ícone perto do relógio** com o estado da conexão e o que está aberto.
- **Instalador único** e, antes de vender, **assinatura digital** (sem ela, o Windows mostra "Windows protegeu seu computador").
- O navegador continua funcionando, para o celular e para quem não instalou.
- Só Windows por enquanto; Mac se algum cliente pedir.

**No piloto da Alis:** botão Rodar para apps com janela, pelo Agente. Primeiro caso completo: o Eq. Research Files Prep (`.exe`), com resultados agrupados e baixados pela plataforma, testado de verdade em 30/09 e 02/10. Rodar em poucos cliques pronto em 02/10, com acabamento em 05/10. A fazer: Parar, scripts com formulário e ambiente preparado sozinho, apps web e o App Hub.

### 1.4 Catálogo e descoberta

Todos os apps da empresa num só lugar, encontráveis por quem precisa. Resolve o retrabalho: antes de criar, a pessoa vê que o colega já fez.

**O que tem**

- **Tela inicial** com os apps em cartões: nome, resumo, responsável, status e último uso.
- **Busca em linguagem natural:** "tem algo que extrai dados de PDF?" encontra o app mesmo que o nome não tenha essas palavras.
- **Organização:** áreas (Investimentos, Operações, RI) e etiquetas livres; favoritos por pessoa.
- **Filtros:** por área, responsável, status (funcionando, com erro, sem dono) e frequência de uso.
- **Mais usados e recentes**, pra quem chega novo saber por onde começar.

**Contra o retrabalho**

- **Aviso de parecido:** ao registrar (1.1) ou criar pelo MCP (1.11), a plataforma compara com o catálogo e sugere "já existe o Extrator de ITR, quer usar ou adaptar?".
- **Componentes reaproveitáveis:** partes de apps (ler PDF da CVM, buscar cotação) ficam disponíveis pra outros apps, via padrões (1.5).
- **Pedidos:** quem não sabe criar pode registrar "preciso de um app que...", e quem cria vê a lista.

**No piloto da Alis:** tela inicial com todos os apps registrados em cartões, Rodar no cartão e busca simples (prontos). A próxima etapa (Etapa 3) traz filtro por área, favoritos, status, "usado há…" e um guia para instalar o Agente no primeiro uso.

### 1.5 Padrões e modelos da empresa

Regras e modelos que fazem todo app da empresa nascer do mesmo jeito. A padronização não é vendida como dor, mas acontece por baixo e deixa tudo mais fácil de manter.

**O que a empresa define**

- **Estrutura padrão de pasta e manifesto:** onde ficam código, entradas, saídas e configuração.
- **Linguagem e bibliotecas preferidas:** por exemplo, Python 3.12, pandas pra dados, pdfplumber pra PDF.
- **Modelos prontos:** "script de entrada e saída", "tarefa agendada", "app com tela", já com login, cofre e registro de execução.
- **Componentes da empresa:** funções reaproveitáveis (ler PDF da CVM, puxar cotações, enviar e-mail).
- **Regras de segurança:** nada de chave no código; dados de cliente só em apps com acesso restrito.
- **Visual:** cores e logo nas telas geradas (1.3).

**Como os padrões são aplicados**

1. **Ao criar pelo MCP (1.11):** a IA recebe os padrões como instrução e já escreve o app no formato da empresa.
2. **Ao registrar um app de fora (1.1):** a plataforma aponta o que foge do padrão e oferece adaptar com um clique, sem mudar o que o app faz.
3. **Ao alterar (1.6):** a mudança é checada contra as regras antes de publicar.

**Detalhes**

- A plataforma vem com um conjunto de padrões inicial; a empresa ajusta.
- Padrão é recomendação, não bloqueio, exceto nas regras de segurança.

**No piloto da Alis:** estrutura padrão de pasta e manifesto, aplicada aos apps existentes.

### 1.6 Versões e histórico

Cada mudança num app vira uma versão, com quem mudou, quando e por quê, explicada em português. Qualquer pessoa pode ver a evolução e voltar pra uma versão anterior.

**Como funciona**

- **Versão automática:** toda alteração (por MCP, pela web ou mexendo direto na pasta) gera uma nova versão, sem perguntar. A plataforma confere a pasta ao abrir a ficha, antes de rodar e ao publicar. Descrever a mudança é opcional.
- **O histórico fica na plataforma, não na pasta do app:** nada é criado dentro da pasta da pessoa. Só o código-fonte tem cópia; de binários (como o `.exe`) fica a impressão digital, e voltar uma versão pede recompilar (a plataforma avisa). A conferência só lê os arquivos que mudaram, para não baixar o que está "só na nuvem" no OneDrive.
- **Resumo legível:** a IA descreve a mudança: "Henry mudou o filtro de datas pra aceitar trimestres (12/09)".
- **Comparar versões:** o que mudou no comportamento, não só no código.
- **Voltar atrás:** um botão restaura qualquer versão anterior, com prévia do que vai mudar. Antes, o estado atual é guardado como versão de segurança, e arquivos a mais vão para uma lixeira, nunca são apagados. A restauração também vira versão.
- **Rascunho e publicado:** o criador testa uma versão nova antes de ela valer pra todo mundo.

**Desenvolvimento compartilhado**

- Duas pessoas podem trabalhar no mesmo app; a plataforma avisa se as mudanças conflitam e a IA propõe como juntar.
- Um colega continua de onde o outro parou, com o contexto das versões anteriores.

**Histórico de execuções**

Além das versões do código, a plataforma guarda cada execução (1.3): quem rodou, em qual computador, versão usada, entradas, onde salvou, resultado e tempo. Isso permite responder "quem gerou esse relatório e com qual versão?".

**Histórico da ficha**

Toda mudança na ficha (nome, resumo, responsável, entradas, saídas) também fica registrada: quem mudou, o quê e quando.

**Linha do tempo do app**

Na página de cada app, as três histórias aparecem juntas, em ordem: mudanças no código, mudanças na ficha e execuções.

**No piloto da Alis:** versão automática a cada alteração, com resumo opcional e botão de voltar; log de execuções e histórico da ficha, numa linha do tempo só (prontos e testados de verdade em 01/10). A ficha só é registrada, não se restaura. Rascunho, resumo escrito pela IA e desenvolvimento compartilhado ficam para depois.

### 1.7 Acesso, permissões e cofre de chaves

Define quem pode ver, usar e alterar cada app, e guarda as chaves e senhas que os apps usam sem que ninguém precise vê-las ou copiá-las.

**Acesso**

- **Login com a conta da empresa** (Google ou Microsoft, ou link por e-mail); sem senha nova pra lembrar.
- **Empresa:** é a conta. Guarda nome, áreas, padrões (1.5), cofre, chave da IA e pastas de apps. Cada pessoa pertence a uma empresa só.

**Cargos (o que cada pessoa pode na empresa toda)**

Cada pessoa tem um cargo. A plataforma vem com três prontos, e a empresa pode criar outros ou mudar estes:

| Cargo pronto | Pode |
| --- | --- |
| Administrador | Tudo |
| Criador | Registrar apps |
| Usuário | Ver e rodar os apps que enxerga |

Um cargo é uma combinação de 9 permissões: registrar apps, editar qualquer ficha, voltar versões de qualquer app, ver a parte técnica de qualquer app, ver as execuções de todos, ver todos os apps (inclusive restritos), cadastrar pessoas, cuidar das áreas e criar ou mudar cargos.

**Áreas (quem cuida de quais apps)**

- Área e time são a mesma coisa (Investimentos, Research, Operações). Cada área tem **líderes** e **membros**, e todo app pertence a uma área.
- **O líder cuida dos apps da sua área:** edita a ficha, volta versões, vê a parte técnica e as execuções de todos, e dá acessos extras, só nesses apps.

**Acessos extras (exceções por pessoa)**

Uma pessoa pode receber, app por app ou para todos os apps de uma área: editar a ficha, voltar versões, ver a parte técnica e ver as execuções de todos.

**App aberto ou restrito**

- **Aberto:** todos da empresa veem e rodam. Todo app novo começa aberto.
- **Restrito** (apps com dados de carteira ou de clientes): só enxergam as áreas e pessoas escolhidas, quem cuida do app (responsável, reserva, líder da área), quem cuida das áreas, quem recebeu um acesso extra nele e o cargo "ver todos os apps". Para os outros, o app simplesmente não existe ("App não encontrado").
- As permissões de cargo da empresa toda valem só nos apps que a pessoa enxerga. Para chegar aos restritos, o cargo precisa também de "ver todos os apps".

**Regras que protegem a empresa**

- **Ninguém dá um cargo com mais permissões do que o próprio,** nem muda o cargo de quem tem mais.
- **Sempre há ao menos uma pessoa** que cuida das pessoas e dos cargos; a plataforma recusa a mudança que deixaria a empresa sem ninguém assim.
- **Quem só edita um app não muda o responsável, a reserva nem a área.** Isso fica com quem cuida do acesso do app.
- **Execuções:** cada um vê as suas. As de todos, só quem tem essa permissão (no cargo, como líder da área ou por acesso extra). Quem deixou de ver um app continua vendo as próprias execuções dele, porque os arquivos são seus.

**Tela Equipe:** Pessoas, Áreas e Cargos, para quem cuida de pessoas, áreas ou cargos, ou lidera uma área.

**Cofre de chaves**

- Chaves de API, senhas de sistemas e tokens ficam guardados criptografados na plataforma.
- O app recebe a chave na hora de rodar; o usuário nunca a vê.
- **Chaves da empresa vs pessoais:** uma chave da empresa é compartilhada por todos que usam o app; uma chave pessoal (ex.: login de um sistema) é pedida de cada usuário.
- **Troca centralizada:** mudou a senha de um sistema, troca num lugar e todos os apps passam a usar a nova.
- **Registro de uso:** qual app usou qual chave, quando.

**No piloto da Alis:** cargos, áreas com líderes, acessos extras, apps restritos e a tela Equipe estão prontos (05/10), falta o teste de verdade. O login ainda é com e-mail e senha, e num modo de desenvolvimento que precisa ser desligado antes de abrir a plataforma para a equipe; antes da nuvem, troca por Google, Microsoft ou link por e-mail. Uma empresa só (Alis). O cofre, a empresa com várias áreas e configurações próprias, e a aprovação antes de publicar ficam para depois.

### 1.8 Saúde e manutenção com IA

A plataforma percebe quando um app quebra, explica o motivo em português e propõe a correção. É o maior diferencial de longo prazo: hoje, só quem criou consegue consertar.

**Por que apps feitos com IA quebram**

- Um site ou API muda o formato (o portal de notícias mudou o layout).
- Uma biblioteca atualiza e muda o comportamento.
- Uma chave expira ou uma senha muda.
- A entrada vem diferente do esperado (um PDF com outra estrutura).

**Como funciona**

1. **Detecta:** toda execução é acompanhada; erro, tempo fora do normal ou resultado vazio viram alerta.
2. **Explica:** a IA lê o erro e o código e escreve pro usuário: "o site de notícias mudou o formato das manchetes; o app não consegue mais ler".
3. **Avisa:** o responsável recebe o alerta (e-mail ou mensagem), com o diagnóstico.
4. **Propõe:** a IA escreve a correção como uma versão de rascunho (1.6) e testa com a última entrada que funcionou.
5. **Aprova:** o responsável vê o antes e depois e aprova com um clique. Nada muda sem aprovação humana.

**Checagens preventivas**

- **Teste periódico:** roda o app com um exemplo conhecido de tempos em tempos, pra achar o problema antes do usuário.
- **Dependências:** avisa sobre bibliotecas desatualizadas ou com falha de segurança conhecida.
- **Chaves:** avisa antes de uma chave expirar.

**Status de cada app:** funcionando, com alerta, quebrado ou sem uso, visível na ficha (1.2), no catálogo (1.4) e no painel (1.10).

**No piloto da Alis:** registro de erros, com explicação da IA quando a chave de IA estiver configurada; correção proposta fica para uma versão seguinte do produto.

### 1.9 Continuidade e responsáveis

Garante que nenhum app morra quando alguém sai, tira férias ou muda de função. É a dor que o sócio sente mais forte e, por isso, um dos principais argumentos de venda.

**Como funciona**

- **Responsável e reserva:** todo app tem as duas pessoas definidas; a plataforma não deixa publicar sem responsável.
- **Saída de pessoa:** quando alguém é removido da empresa na plataforma, ela lista os apps daquela pessoa e pede a transferência antes de concluir.
- **Ausência temporária:** em férias, os alertas (1.8) vão pra reserva automaticamente.
- **Apps em risco:** destaque pra apps sem reserva, com responsável inativo, usados por muita gente mas mantidos por uma pessoa só, ou sem uso há muito tempo.
- **Passagem de bastão:** ao transferir um app, a IA gera um resumo pro novo responsável: o que o app faz, decisões importantes do histórico (1.6) e problemas recentes (1.8).

**Teste de continuidade**

Uma verificação simula "e se esta pessoa saísse hoje?" e mostra quais apps ficariam sem dono e quais processos da empresa dependem deles.

**No piloto da Alis:** responsável e reserva em todos os apps, com a lista de apps em risco.

### 1.10 Painel de governança

Uma tela pro sócio ou administrador ver, em um minuto, o que existe, o que é usado e onde está o risco. É o que faz quem paga enxergar o valor do produto.

**O que mostra**

| Bloco | Conteúdo |
| --- | --- |
| Inventário | Quantos apps existem, por área e por tipo |
| Uso | Apps mais usados, quem usa, execuções por semana |
| Saúde | Apps com erro ou alerta agora e há quanto tempo |
| Risco | Apps sem reserva, com dono único ou responsável inativo (1.9) |
| Segurança | Apps com acesso a dados sensíveis, chaves perto de expirar, desvios dos padrões (1.5) |
| Atividade | Versões publicadas, novos apps e transferências no período |
| Custo | Gasto com nuvem e IA por app (quando o Módulo 2 estiver ativo) |

**Detalhes**

- **Resumo semanal por e-mail:** a IA escreve um parágrafo com o que mudou e o que merece atenção.
- **Trilha de auditoria:** quem rodou, alterou ou deu acesso a quê, quando, exportável. Útil pra empresas reguladas, como gestoras.
- **Tempo economizado:** estimativa por app, a partir do número de execuções e do tempo que a tarefa levava à mão (informado pelo responsável).

**No piloto da Alis:** inventário, uso e apps em risco numa tela simples.

### 1.11 Servidor MCP

O servidor MCP liga a plataforma à IA que o criador já usa. Em vez de construir um editor próprio, a plataforma vira o ambiente onde o Claude, o Cursor ou o ChatGPT criam e mantêm os apps da empresa. MCP (Model Context Protocol) é um padrão aberto que essas ferramentas já suportam.

**Exemplo**

O criador pede ao Claude: "cria um app que resume as notícias das empresas da carteira todo dia às 7h". Pelo MCP, o Claude:

1. busca no catálogo se já existe algo parecido (1.4);
2. puxa os padrões e componentes da empresa (1.5);
3. cria o app dentro da plataforma, com versão desde a primeira linha (1.6);
4. usa a chave do cofre sem vê-la (1.7);
5. testa, publica e agenda (1.3), pedindo confirmação antes de liberar pra empresa.

**Ferramentas expostas pelo MCP**

| Ferramenta | O que a IA consegue fazer |
| --- | --- |
| Catálogo | Buscar apps e componentes, ler fichas |
| Padrões | Ler as regras e modelos da empresa |
| Projeto | Criar app, ler e alterar arquivos, salvar versão |
| Execução | Rodar o app com entradas de teste e ler o resultado |
| Saúde | Ler erros e logs, propor correção |
| Publicação | Publicar, definir acesso e agendar |
| Cofre | Referenciar chaves pelo nome, nunca lê-las |

**Regras de segurança**

- A IA age com as permissões de quem está usando (1.7), nunca mais que isso.
- Publicar pra empresa, mudar acesso e mexer em apps restritos exigem confirmação humana.
- Toda ação feita pela IA fica na trilha de auditoria (1.10), marcada como "via IA".

**Por que isso importa:** a plataforma fica neutra entre IAs. Se surgir uma IA melhor, os apps e o histórico continuam no mesmo lugar. A importação manual (1.1) continua existindo pra quem cria fora.

**Registro pelo MCP (antecipado para o piloto)**

A primeira parte do MCP a existir é o registro, porque dispensa a chave de IA da plataforma: quem lê o código e escreve a ficha é o Claude que o criador já usa.

1. O criador gera um **código de acesso pessoal** na plataforma e conecta o Claude, uma vez só.
2. Pede, por exemplo: "registra todos os apps da pasta de programação".
3. O Claude lista as pastas e o catálogo, lê o código com **chaves e senhas mascaradas** pela plataforma (ele nunca vê os valores), parte do diagnóstico por regras e completa com o que entendeu: resumo em português, entradas, saídas, arquivo de início, área (futuro time) e pontos de atenção.
4. Registra cada app com responsável e reserva, pedindo confirmação a cada um. O app fica marcado como **registrado via IA**.

O passo a passo que o Claude segue é um protocolo escrito (`docs/PROTOCOLO-REGISTRO.md`), testado primeiro à mão e depois transformado nas instruções do MCP.

**No piloto da Alis:** o registro pelo MCP entra no piloto, antes do resto do MCP. Está pausado desde 30/09/2026, até a plataforma ficar redonda com um app só. Criar app do zero, rodar, ler saúde e usar o cofre pelo MCP ficam para depois.

## O piloto da Alis

O piloto roda no computador do Henry (Windows), sem nuvem, usando como teste a pasta de programação da Alis (`5. Operacional\8. Programação`).

**O que a pasta real mostrou**

A variedade confirmou que a plataforma precisa ser genérica: scripts que leem um arquivo com nome fixo, scripts que abrem uma janela de seleção, apps web (Streamlit), apps de desktop, uma tarefa agendada no Windows, executáveis prontos, um sistema com vários serviços e documentos que não são apps.

**Onde está hoje (06/10/2026)**

| Parte | Situação |
| --- | --- |
| Fundação: login, pessoas, visual | Pronta. Visual novo em 02/10: limpo e neutro, com um violeta vivo de destaque, claro e escuro |
| Registro (1.1): diagnóstico por regras, lote, segredos, duplicados, manifesto e ficha na pasta | Pronto |
| Registro pelo Claude (1.11) | Pausado. Protocolo testado com 1 app; faltam 4 apps e o MCP em si |
| Ficha (1.2): página única, bloco técnico recolhido, "Editar" por parte | Pronta (05/10) |
| Catálogo (1.4): cartões, busca, Rodar no cartão | Pronto. Faltam filtros, favoritos e status (Etapa 3) |
| Execução (1.3) de apps com janela (`.exe`) pelo Agente, em poucos cliques | Pronta e testada de verdade (Eq. Research Files Prep). Falta o Parar |
| Atualização automática do Agente | Pronta e testada de verdade (30/09); Agente na versão 0.2.3 |
| Versões e linha do tempo (1.6) | Prontas e testadas de verdade (01/10) |
| Cargos, áreas, acessos e apps restritos (1.7) | Prontos (05/10). Falta o teste de verdade |
| Execução de scripts com formulário, ambiente preparado sozinho, apps web | A fazer |
| App Hub: ícone, janela própria, instalador único, assinatura | A fazer |
| Cofre (1.7) | A fazer |
| Saúde, continuidade e painel (1.8 a 1.10) | A fazer |
| Funções de IA dentro da plataforma | Aguardando decisão sobre a chave de IA |

**O que a primeira semana ensinou**

- **O Agente foi a decisão certa:** o mesmo desenho funciona hoje, com a plataforma no computador do Henry, e amanhã, com ela na nuvem.
- **Pastas reais são grandes e vivem no OneDrive:** o histórico não pode copiar tudo nem baixar arquivos "só na nuvem".
- **Permissões por papel em cada app não refletem a empresa:** quem cuida dos apps de uma área é o líder dela. Daí cargos, áreas e acessos extras.
- **A plataforma ficou redonda num caso só:** o tipo de app mais comum na pasta (scripts) ainda não roda, e só o Henry usa. Os próximos passos precisam testar a generalidade e trazer usuários de verdade.

**Prioridade**

Ordem combinada:
1. Teste de verdade das permissões com o Henry e o botão **Parar** (Agente 0.2.4).
2. Etapa 3: catálogo e primeiro uso (favoritos, filtro por área, status, guia do Agente).
3. Rodar scripts: formulário automático e ambiente preparado sozinho.
4. App Hub (ícone, janela própria e instalador único), antes do primeiro colega entrar.
5. Assinatura digital, antes de vender.
6. Retomar o registro pelo Claude, com mais 4 apps e o MCP.
7. Decidir apps pessoais e compartilhados, junto com "rodar apps web".

**Proposta em discussão (06/10):** trazer **rodar scripts** para logo depois do item 1, com o *Rentabilidade fundos* como app piloto, porque é a feature central e a maior parte da pasta é script. Em seguida, **saúde, apps em risco e painel** (1.8 a 1.10), que usam dados já existentes e são o argumento de venda para o sócio. Depois, **plataforma num servidor central com login de verdade** e um ou dois colegas usando. A Etapa 3 vem quando houver muitos apps no catálogo.

## Módulo 2 — Nuvem gerenciada

O Módulo 2 tira a execução dos apps do computador de quem usa e leva pra nuvem. Com ele, os apps funcionam de qualquer lugar, rodam sozinhos no horário e não dependem de nenhuma máquina ligada. Para o cliente, a nuvem é invisível: ele só vê que o botão "Rodar" agora funciona também do celular.

**Objetivo:** todo app registrado no Módulo 1 pode rodar na nuvem com um clique, com segurança, custo previsível e cobrança em reais.

**Premissa:** não temos data center. Alugamos infraestrutura (no início Fly.io, Railway ou Render; depois AWS ou similar) e vendemos a camada simples por cima, como Heroku e Vercel fazem sobre a AWS.

| Feature | Resolve |
| --- | --- |
| 2.0 A plataforma sempre num lugar central | Um catálogo só, com apps rodando na nuvem ou no computador |
| 2.1 Hospedagem e execução gerenciada | App rodando sem depender de um computador |
| 2.2 Isolamento e segurança | App de um cliente nunca acessa dados de outro |
| 2.3 Agendamento e tarefas em segundo plano | Tarefas que rodam sozinhas no horário |
| 2.4 Acesso remoto e links | Usar de casa, do celular, em viagem |
| 2.5 Dados, arquivos e região | Onde ficam entradas, saídas e dados dos apps |
| 2.6 Otimização de custo | Rodar muitos apps pequenos gastando pouco |
| 2.7 Planos e cobrança | Preço fixo, previsível, em reais |

O Módulo 2 fica fora do piloto da Alis, que roda só localmente.

### 2.0 A plataforma sempre num lugar central

O "cérebro" da plataforma (catálogo, fichas, histórico, permissões, cofre, MCP) fica **sempre num servidor central**: na nossa nuvem ou, no plano Local, num computador do cliente sempre ligado. O código é o mesmo nos dois casos. Instalar a plataforma inteira em cada computador foi descartado: cada um teria o próprio catálogo, o que quebra o compartilhamento e a continuidade.

**O que muda para um app com janela quando a plataforma vai para a nuvem** (exemplo: o Eq. Research Files Prep)

1. A pessoa clica em Rodar, pelo navegador, pelo App Hub ou pelo celular.
2. A plataforma na nuvem anota o pedido: "abrir este app no computador desta pessoa".
3. O Agente, que fica perguntando "tem pedido para mim?", pega o pedido e abre o app na tela da pessoa. Funciona atrás do firewall da empresa, porque quem começa a conversa é sempre o Agente.
4. A pessoa usa o app, que grava os resultados na pasta dela, como sempre.
5. Ao fechar, o Agente avisa quais arquivos foram gerados.
6. Ao clicar em Baixar, o arquivo sai do computador, passa pela plataforma e chega ao navegador, sem ficar guardado na nuvem.

Consequências: baixar fica um pouco mais lento; o computador precisa estar ligado para baixar; os arquivos passam pela nuvem a caminho do navegador (ponto para o compliance de clientes regulados); cada Agente precisa ser conectado uma vez ao endereço novo. Antes de ir para a internet, a plataforma precisa de login de verdade e de endereço seguro (https).

**O que roda onde**

| Tipo de app | Na nuvem | No computador, pelo Agente |
| --- | --- | --- |
| Script com entrada e saída | ✅ Ideal | ✅ Também |
| App com tela própria (web) | ✅ Ideal, sobretudo se for compartilhado | ✅ Se for pessoal |
| App de desktop e executável | ❌ O servidor não tem tela nem Windows | ✅ Sempre |
| App que depende do escritório (OneDrive local, Excel instalado, rede interna, sistemas de terceiros) | ❌ | ✅ Sempre |

### 2.1 Hospedagem e execução gerenciada

Com um clique, o app registrado passa a rodar na nuvem, e o botão "Rodar" (1.3) passa a executá-lo lá, não na máquina de quem clica.

**Como funciona**

1. O responsável ativa "rodar na nuvem" na ficha do app.
2. A plataforma empacota o app com o ambiente fixado em 1.1 (linguagem, bibliotecas, configuração).
3. Faz um teste com o exemplo conhecido; se passar, a execução na nuvem vira o padrão.
4. Cada nova versão publicada (1.6) é empacotada e testada do mesmo jeito, com volta automática pra anterior se o teste falhar.

**Formas de execução**

| Tipo de app | Como roda na nuvem |
| --- | --- |
| Script com entrada e saída | Sobe sob demanda, roda e desliga |
| App com tela própria | Fica disponível por link; dorme quando ninguém usa e acorda no primeiro acesso |
| Tarefa recorrente | Roda no horário agendado (2.3) |

**Detalhes**

- **Limites por execução:** tempo máximo, memória e processamento, ajustáveis por plano (2.7).
- **Logs e saída:** tudo o que o app imprime e gera volta pra plataforma e entra no histórico (1.6) e na saúde (1.8).
- **Modo híbrido:** apps que precisam acessar algo só disponível no escritório (um arquivo na rede local, um sistema interno, um programa instalado) ou que têm janela própria continuam rodando localmente pelo Agente do Hub (1.3), e a plataforma deixa claro onde cada um roda. O Agente foi desenhado para isso: com a plataforma na nuvem, só muda o endereço com que ele conversa.
- **Infra trocada por baixo:** a camada de execução é abstraída, para migrar de Fly.io/Railway pra AWS sem o cliente perceber.

### 2.2 Isolamento e segurança

A plataforma roda código gerado por IA, de vários clientes, nos mesmos servidores. O isolamento é inegociável: o app de um cliente nunca pode enxergar dados, arquivos ou chaves de outro.

**Camadas de proteção**

- **Execução isolada:** cada execução roda num ambiente fechado (container ou micro máquina virtual), descartado ao final.
- **Separação por empresa:** dados, arquivos, cofre e rede de cada cliente ficam separados.
- **Rede controlada:** por padrão, o app só acessa os endereços de que precisa (declarados no manifesto); o resto é bloqueado.
- **Chaves só na hora:** o cofre (1.7) entrega a chave apenas durante a execução, e ela nunca fica gravada no ambiente.
- **Criptografia:** dados guardados e em trânsito sempre criptografados.

**Controles pro cliente**

- **Revisão antes da nuvem:** a IA analisa o app antes de subir e aponta riscos (envia dados pra fora, grava onde não devia).
- **Registro completo:** o que cada execução acessou fica na trilha de auditoria (1.10).
- **Conformidade:** atendimento à LGPD e documentação de segurança pronta pra clientes regulados (gestoras, escritórios).

Esta é a parte mais técnica do produto e a principal responsabilidade do sócio técnico.

### 2.3 Agendamento e tarefas em segundo plano

Apps que rodam sozinhos, sem ninguém clicar e sem computador ligado. É onde o Módulo 2 mais agrega sobre o Módulo 1, porque o agendamento local só funciona com a máquina ligada.

**Gatilhos**

| Gatilho | Exemplo |
| --- | --- |
| Horário | Resumo de notícias todo dia útil às 7h |
| Calendário | Relatório pro cotista no primeiro dia útil do mês |
| Chegada de arquivo | Novo PDF numa pasta ou e-mail dispara a extração |
| Fim de outro app | Extração termina e o modelo é atualizado (encadeamento, 1.3) |
| Chamada externa | Outro sistema aciona o app por um link seguro |

**Entrega do resultado**

- Por e-mail, em mensagem (Teams, Slack, WhatsApp) ou numa pasta compartilhada.
- O resultado também fica na plataforma, no histórico de execuções (1.6).

**Detalhes**

- **Calendário brasileiro:** "dia útil" considera feriados nacionais.
- **Nova tentativa:** se falhar, tenta de novo algumas vezes antes de alertar o responsável (1.8).
- **Tarefas longas:** execuções de minutos ou horas continuam mesmo que o usuário feche a tela, e avisam quando terminam.

### 2.4 Acesso remoto e links

Qualquer app na nuvem pode ser aberto de qualquer lugar pelo navegador, inclusive no celular, sempre com login. Resolve diretamente o caso da Alis: usar de casa o app que hoje só roda no escritório.

**Como funciona**

- **Endereço da empresa:** todos os apps ficam num endereço próprio (ex.: apps.nomedaempresa.com.br ou um subdomínio da plataforma).
- **Link por app:** cada app tem um link fixo, que pode ser favoritado ou colado numa mensagem; quem abre precisa estar logado e ter acesso (1.7).
- **Celular:** as telas geradas (1.3) se adaptam a telas pequenas.
- **Compartilhamento externo controlado:** o responsável pode gerar um link temporário pra alguém de fora (um cliente, um parceiro), com validade e registro de uso.

**Detalhes**

- Sessões expiram após inatividade.
- Acesso de dispositivo novo pode exigir confirmação em dois passos, a critério da empresa.

### 2.5 Dados, arquivos e região

Define onde ficam os arquivos que os apps recebem e geram, e os dados que precisam guardar entre uma execução e outra.

**Tipos de dado**

| Tipo | Exemplo | Onde fica |
| --- | --- | --- |
| Entradas enviadas | O PDF que o usuário subiu | Armazenamento da empresa, com prazo de retenção |
| Saídas geradas | O Excel resultado | Armazenamento da empresa, ligado à execução |
| Dados persistentes do app | Histórico de notícias já enviadas | Banco simples por app |
| Arquivos da empresa | Pasta no OneDrive, SharePoint ou Google Drive | Na origem, acessados por conector |

**Detalhes**

- **Conectores de pasta:** o app lê e grava direto nas pastas que a empresa já usa, sem copiar tudo pra plataforma.
- **Retenção:** a empresa define por quanto tempo entradas e saídas ficam guardadas.
- **Região:** opção de manter os dados no Brasil (região São Paulo), importante para LGPD e clientes regulados.
- **Exportação:** a empresa pode baixar todos os seus apps, versões e dados a qualquer momento. Não há aprisionamento.

### 2.6 Otimização de custo

A plataforma roda muitos apps pequenos gastando o mínimo de infraestrutura. O cliente não gerencia nada disso: a economia vira preço baixo e previsível pra ele e margem pra nós.

**Como economizamos**

- **Liga só quando precisa:** scripts sobem na execução e desligam ao terminar; apps com tela dormem sem uso.
- **Densidade:** vários apps pequenos, de vários clientes, dividem os mesmos servidores, sempre isolados (2.2).
- **Tamanho certo:** a plataforma mede quanto cada app usa de memória e processamento e ajusta o tamanho do ambiente.
- **Reaproveitamento:** ambientes com as mesmas bibliotecas são preparados uma vez e reutilizados.
- **Tarefas só no horário:** apps agendados não ficam ligados esperando.

**O que o cliente vê**

- Um comparativo simples: "rodar estes 8 apps por conta própria custaria X; aqui está incluído no plano".
- Consumo de IA por app (quando o app usa chaves de IA do cofre), pra saber quais apps gastam mais tokens.

Gerenciar a conta de nuvem que o cliente já tem (FinOps) fica fora do escopo: nosso público hoje quase não gasta com nuvem.

### 2.7 Planos e cobrança

Mensalidade fixa por empresa, em reais, com nota fiscal. Para PME sem TI, previsibilidade pesa mais que ser o mais barato: a conta variável em dólar das plataformas atuais é um dos motivos pra não usá-las.

**Estrutura proposta (valores a definir nas entrevistas)**

| Plano | Pra quem | Inclui |
| --- | --- | --- |
| Local | Quem só quer o Módulo 1 | Gestão completa; a plataforma fica num computador da empresa sempre ligado, e os apps rodam nos computadores de quem usa |
| Equipe | Até cerca de 10 pessoas | Módulo 1 + nuvem para uma faixa de apps e execuções |
| Empresa | Até cerca de 50 pessoas | Faixas maiores, auditoria completa, dados no Brasil, suporte prioritário |

**Regras**

- **Preço por faixa**, não por usuário nem por app: cobrar por usuário desestimula o uso, e cobrar por app desestimula criar.
- **Sem surpresa:** passar da faixa gera aviso e sugestão de upgrade, nunca cobrança automática inesperada.
- **Pagamento brasileiro:** boleto, Pix ou cartão nacional; nota fiscal emitida automaticamente.
- **Teste grátis** com os apps reais da empresa.
