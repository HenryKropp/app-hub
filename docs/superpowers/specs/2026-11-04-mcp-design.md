# MCP do Ziv: desenho

**Data:** 2026-11-04 · **Status:** aprovado pelo Henry em 2026-11-04 (documento e escolhas do cofundador)

O MCP é a porta que deixa o Claude de cada pessoa usar o Ziv por conversa. Este documento detalha a **Etapa 1** e resume as seguintes. Ele substitui a ordem antiga da Fase 2b em `docs/TASKS.md`, em que o registro de apps vinha primeiro.

## 1. Por que

Hoje, para usar o Ziv, a pessoa precisa entrar nele, achar o app, preencher a janela Rodar e subir artifacts pela tela. Quem já trabalha o dia todo no Claude quer pedir isso ali mesmo: "tem app que baixa os documentos da CVM?", "roda o Files Prep para PETR4", "publica esse dashboard no Ziv".

**Sucesso da Etapa 1:**
- O Henry conecta o Claude Code ao Ziv local com um código pessoal e:
  - pede um app por assunto e recebe o certo, com a explicação da ficha;
  - roda um app de teste e o Claude acompanha de 30 em 30 segundos até terminar, então diz o resultado e onde estão os arquivos; a execução aparece em Minhas execuções;
  - publica um HTML feito pelo Claude e o vê em Artifacts, funcionando; pedir de novo para o mesmo artifact cria a versão 2.
- Uma pessoa sem acesso a um app restrito não o acha nem o roda pelo Claude.
- Depois de o Ziv ser publicado: o mesmo funciona no claude.ai e no app do Claude, entrando com a conta do Ziv.

## 2. Decisões do Henry (2026-11-04)

| Tema | Decisão |
|---|---|
| Para que serve | Registrar apps prontos; criar apps já pensando no Ziv; publicar artifacts "bonitinhos"; agendar apps; achar e rodar apps |
| Ordem | Etapa 1: conectar, achar, rodar e publicar artifacts. Etapa 2: registrar apps prontos. Etapa 3: criar apps para o Ziv. Etapa 4: agendar |
| Qual Claude | **Os dois**: claude.ai/app do Claude (Windows/Mac) e Claude Code |
| Instalação | Uma tela no Ziv para a pessoa conectar o Claude. Não há nada para baixar e nada vai junto com o Agente, porque o MCP mora dentro do Ziv (proposta do cofundador, aceita pelo Henry) |
| Etapa 1 | Aprovada como resumida: conectar, agir como a pessoa, achar, rodar, publicar artifact com guia de visual, limite do claude.ai só com o Ziv na internet |

## 3. Etapa 1: o que a pessoa vive

### 3.1 Conectar

- **claude.ai e app do Claude:**
  - Quem administra o Claude da Alis adiciona o Ziv uma vez como conector personalizado, usando o endereço da empresa no Ziv.
  - Depois, cada pessoa clica em **Conectar** no Claude. Abre a tela de entrada do próprio Ziv, e a pessoa entra como sempre (e-mail e senha ou Google).
  - Vê uma confirmação: "Permitir que o Claude use o Ziv como você?" e clica em **Permitir**.
- **Claude Code:**
  - Na tela **Conectar o Claude**, a pessoa gera um **código pessoal** com um nome (ex.: "notebook").
  - O Ziv mostra uma vez o comando pronto para colar, já com o código, e um botão Copiar.
  - A lista mostra os códigos com nome, quando foram criados e o último uso. Cada um tem **Cancelar**.
- **Endereço:** cada empresa tem o seu, no mesmo padrão das telas: `…/w/<empresa>/mcp`; a empresa principal usa `…/mcp`. O Claude conectado a uma empresa só enxerga aquela empresa.

### 3.2 O Claude age como a pessoa

- Tudo passa pelas mesmas regras das telas: o Claude só acha, abre, roda e publica o que a pessoa poderia fazer no Ziv.
- A conexão do Claude é independente da sessão do navegador: sair do Ziv no navegador não desconecta o Claude (decisão do cofundador).
- O acesso do Claude acaba na hora em três casos:
  - a pessoa é desativada (todas as conexões dela caem);
  - a pessoa cancela a conexão ou o código na tela Conectar o Claude;
  - a chave temporária vence sem ser renovada.
- Rodar e publicar são ações que mudam algo. O Ziv as marca assim, para o Claude pedir "permitir?" antes, como ele já faz com ações desse tipo.

### 3.3 O que o Claude consegue fazer (ferramentas)

| Ferramenta | O que faz |
|---|---|
| Buscar | Procura em apps e artifacts por palavra, área ou etiqueta; devolve nome, resumo, tipo, área e link |
| Ver app | A ficha do app: o que pede (entradas), o que entrega, quem cuida, versão, se roda no computador ou no servidor e se a pessoa pode rodar |
| Ver artifact | Nome, resumo, etiquetas, versões e link |
| Rodar app | Recebe o app e as respostas do que ele pede (mesmos campos da janela Rodar). Roda no computador da pessoa pelo Agente, ou no servidor quando o app é compartilhado. Devolve o número da execução e o link |
| Ver execução | Situação (esperando, rodando, concluída, erro…), mensagem de erro em português, e a lista de arquivos gerados com a pasta onde ficaram. Pode **esperar até 30 segundos** por uma mudança antes de responder |
| Minhas execuções | As últimas execuções da pessoa |
| Publicar artifact | Recebe o HTML (feito pelo Claude), nome, resumo, etiquetas e área. Cria o artifact ou, se receber o artifact existente, sobe uma versão nova. Devolve o link |
| Guia de visual | Um texto curto com o visual do Ziv (preto e branco, fonte Geist, coral só em ação) e as regras do Ziv para HTML (um arquivo só; bibliotecas de CDN permitidas; nada de pedir senha) para o artifact ficar bonito e funcionar dentro do Ziv |

**Acompanhar até o fim (Henry, 2026-11-04: "loops de 30 em 30 segundos").** O Claude só fala quando a pessoa escreve; não consegue mandar mensagem sozinho depois. Por isso:
- depois de Rodar, ele consulta "Ver execução" de 30 em 30 segundos, na mesma resposta, até o app terminar, e então diz o resultado e onde ficaram os arquivos; as descrições das ferramentas pedem isso a ele;
- se a pessoa interromper, ou a conversa parar antes, o Claude confere de novo quando a pessoa perguntar ("terminou?");
- a faixa de execuções do Ziv continua avisando no navegador, como hoje.

Os mesmos limites das telas continuam valendo:
- **Rodar:** sem computador conectado, o Claude recebe a mesma explicação que a pessoa veria ("seu computador não está conectado; abra o Agente"). Entradas inválidas voltam com a mensagem de erro do Ziv.
- **Publicar:** os limites de tamanho e o aviso de segredos no HTML são os mesmos do upload pela tela. Quando o Ziv encontra algo que parece senha, o Claude recebe o aviso e precisa confirmar explicitamente para publicar mesmo assim.
- O que entrou pelo Claude fica marcado: o artifact mostra a origem "pelo Claude" (o artifact já tem o campo de origem) e a execução também.

### 3.4 A tela "Conectar o Claude"

- Fica em **Configurações**, abaixo de "Meu computador". Também aparece um link para ela em **Começar**.
- A tela tem dois blocos, um para cada Claude:
  - **Site ou app do Claude:** o endereço do conector com Copiar, mais 3 passos em palavras simples. Inclui a nota: "Se a sua empresa usa o Claude em plano de equipe, quem administra o Claude adiciona o Ziv uma vez; depois cada um só clica em Conectar".
  - **Claude Code:** gerar código e ver a lista de códigos, como descrito em 3.1.
- Abaixo dos dois blocos fica **Conexões ativas**: a lista de tudo que está conectado em nome da pessoa (claude.ai, app do Claude e cada código do Claude Code), com nome, último uso e **Cancelar**. O administrador não cancela conexões de outras pessoas nesta etapa: desativar a pessoa já corta tudo (decisão do cofundador).
- Enquanto o Ziv roda só no computador (endereço `localhost`), o bloco do site/app avisa que essa forma só funciona depois de o Ziv ser publicado na internet.

## 4. Fora da Etapa 1

- Registrar apps (Etapa 2), criar apps (Etapa 3) e agendar (Etapa 4).
- Mandar arquivos do chat como entrada de um app. Na Etapa 1, só valem entradas que a janela Rodar aceita como texto, número, escolha, data ou caminho no computador da pessoa.
- Baixar o conteúdo dos arquivos gerados para dentro do Claude. O Claude informa onde eles estão e dá o link.
- Parar uma execução, aprovar apps, gerir pessoas ou mexer no Admin pelo Claude.
- Chave da API do Claude no Ziv: quem pensa é o Claude da própria pessoa.
- Instalar algo no computador, ou empacotar o MCP com o Agente.

## 5. Restrições

- **Onde roda:** o MCP fica dentro do próprio servidor do Ziv (o mesmo programa em Go), no transporte HTTP atual do MCP, com o SDK oficial de MCP para Go. Ele chama as mesmas regras de acesso dos handlers, sem reimplementá-las.
- **claude.ai e o app do Claude** exigem um endereço público em HTTPS e entrada pelo padrão OAuth do MCP:
  - o Ziv anuncia onde se entra;
  - aceita que o Claude se cadastre sozinho como cliente;
  - a pessoa entra e permite;
  - o Claude recebe uma chave temporária que pode ser renovada e cancelada.

  No computador, isso só é testável pelo Claude Code. O teste no claude.ai fica para depois da publicação.
- **Códigos e chaves:**
  - ficam guardados só como resumo criptográfico (hash);
  - o código é mostrado uma única vez;
  - cada uso atualiza o "último uso";
  - nunca aparecem em registros (logs).
- **Segredos:** nenhuma resposta do MCP traz valor do cofre, de configuração secreta ou de variável de ambiente de app compartilhado.
- **Multiempresa:** o endereço decide a empresa, e uma chave ou código de uma empresa não vale em outra. Em produção é um domínio só, com as empresas em `/w/<empresa>`, como já funciona hoje. Por isso cada empresa anuncia seus próprios dados de entrada OAuth no endereço dela, com o sufixo de caminho que o padrão permite (decisão do cofundador).
- **Idioma:** as descrições das ferramentas e as mensagens ficam em português simples, porque o Claude as repete para a pessoa.

## 6. Etapas seguintes (resumo, cada uma com seu próprio desenho)

- **Etapa 2: registrar apps prontos.** Segue `docs/PROTOCOLO-REGISTRO.md`. O Claude lê a pasta do app **pelo Agente do Ziv** no computador da pessoa, com chaves e senhas mascaradas, então funciona nos dois Claudes. Ele monta o manifesto, que é validado pelo Ziv, e registra com responsável e reserva pelo mesmo caminho do botão Publicar. O app fica marcado "via Claude". Isso libera o teste de vários apps abertos juntos.
- **Etapa 3: criar apps para o Ziv.** O Claude recebe o formato do manifesto, as regras da empresa e exemplos, e entrega o app com o `ziv.yaml` pronto para a Etapa 2 registrar.
- **Etapa 4: agendar.** Primeiro o próprio Ziv aprende a agendar: horário ou repetição por app, rodando no servidor (apps compartilhados) ou no computador com o Agente ligado. Depois, o Claude ganha "agendar", "ver agendamentos" e "cancelar agendamento".

## 7. O que fica com quem constrói

- Os nomes exatos das ferramentas e o formato do que elas devolvem.
- Detalhes do OAuth: duração das chaves, telas de erro e quantos clientes cada pessoa pode ter.
- Como gravar a origem "pelo Claude" na execução.
- O texto exato do guia de visual, tirado de `DESIGN.md`.
- O teste de ponta a ponta com um cliente MCP de verdade.
