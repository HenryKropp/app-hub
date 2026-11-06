# Agendar apps e tarefas — desenho

Data: 2026-11-06. Pedido do Henry: "cria opção de agendar apps/tarefas".

## Por que

O objetivo do Ziv é as pessoas pararem de rodar apps no próprio PC. Muitas ferramentas da empresa rodam sozinhas num horário, como o News Scrapper (tarefa agendada do Windows a cada 5 min) e relatórios diários. Hoje, no Ziv, tudo depende de alguém clicar em Rodar. Com o agendamento, o app roda sozinho e o resultado fica registrado no Ziv. A especificação já previa isso (Especificação v3, 1.3 e 2.3), e a Etapa 4 do MCP ("agendar") depende do Ziv saber agendar.

## O que o Henry decidiu

- **Os dois tipos de tarefa:** de minuto em minuto (News Scrapper) e em horário fixo (relatório todo dia às 7h).
- **Onde roda:** o ideal é no servidor, sem PC ligado. Nem todo app consegue rodar lá, então app que precisa do Windows roda pelo Agente num PC.
- **Resultado:** toda execução agendada aparece sempre no histórico. Cada agendamento tem a opção de mandar ou não um e-mail.
- **Quem agenda:** qualquer pessoa que pode rodar o app cria os seus próprios agendamentos.
- **Como escolhe o horário:** opções prontas, sem código: a cada N minutos, a cada N horas, todo dia às X, em dias da semana às X e todo mês no dia N às X. O mínimo é 5 minutos. A tela mostra as próximas execuções.
- **Ordem de entrega:**
  1. Agenda + rodar pelo Agente.
  2. Rodar no servidor.
  3. E-mail.

## Objetivos

- Uma pessoa abre um app que pode rodar, escolhe "Agendar", preenche os campos do app uma vez e escolhe o horário. A partir daí o app roda sozinho.
- Cada execução agendada aparece no histórico do app e em Minhas execuções com o selo "agendada". Ela tem o mesmo detalhe e os mesmos downloads de uma execução manual.
- A pessoa vê, pausa, edita e apaga os seus agendamentos. Dono, reserva e administrador veem todos os agendamentos do app.
- Quando o agendamento não consegue rodar (PC desligado, app suspenso, acesso retirado), isso fica visível. Nada some em silêncio.

## Fora do escopo agora

- **Ferramentas do MCP para agendar** (o Claude criar agendamentos). Ficam para a Etapa 4 do MCP, depois deste trabalho.
- **Expressão cron / modo avançado.** O Henry escolheu só opções prontas.
- **Apps com tela (web) e apps com janela própria (desktop).** Esses esperam alguém usar a tela, então agendar não faz sentido. O App compartilhado "sempre ligado" é outro trabalho (TASKS, News Scrapper).
- **Repetir automaticamente uma execução que falhou.**

## O que não pode acontecer

- **Uma execução não pode sumir sem explicação.** Isso vale para o PC desligado, o servidor reiniciando e o app suspenso.
- **Um agendamento não pode rodar algo que a pessoa não pode mais rodar.** Os casos são: acesso retirado, app suspenso ou não aprovado, pessoa removida.
- **Um agendamento não pode empilhar execuções.** Se a anterior ainda está rodando, a nova é pulada e o pulo fica registrado.
- **O servidor pequeno (e2-micro, 1 GB) não pode ser derrubado** por muitos agendamentos.

## Peça 1 — Agenda + rodar pelo Agente

### Quais apps podem ser agendados

Nesta peça só os **Scripts** (`script`) podem ser agendados. Hoje o Ziv só roda pelo Agente dois tipos de app: os Scripts e os apps com janela própria que são `.exe` (`canRunKind`, `runs.go`). Os tipos `scheduled` e `executable` ainda não rodam.

Um `.exe` com janela abre uma janela no PC da pessoa e espera alguém usar, então não é agendado. Isso é decisão do builder.

Campos do tipo `window` (o app pergunta depois de abrir) impedem o agendamento, e a tela explica o motivo. Os outros tipos não mostram a opção.

Um app registrado como "Tarefa recorrente" que na verdade é um script Python, como o News Scrapper, entra registrado como Script.

### O agendamento

Cada agendamento guarda:

- **App, pessoa que criou e nome opcional.** Exemplo de nome: "Fechamento da manhã".
- **Frequência.**
  - Um dos tipos: `minutes` (a cada N, N ≥ 5), `hours` (a cada N, 1–24), `daily` (às HH:MM), `weekdays` (dias escolhidos às HH:MM) ou `monthly` (dia 1–28 às HH:MM).
  - Fuso sempre America/Sao_Paulo.
  - O dia vai até 28 para não existir "dia 31 em fevereiro". É decisão do builder: é simples e cobre o caso "todo começo de mês".
- **Valores dos campos do app**, exatamente como o formulário de Rodar manda hoje (`inputs` e `files`).
  - Os arquivos enviados ficam guardados com o agendamento, com o mesmo limite de 10 MB.
  - Também fica guardada a pasta onde salvar.
- **Computador:** fica fixo no agendamento. O padrão é o computador da pessoa que está conectado na hora de criar, e ela pode trocar por outro computador seu. Isso é decisão do builder: é previsível, porque a pessoa pode ter mais de um PC.
- **Estado:**
  - ativo ou pausado, com o motivo quando foi o Ziv que pausou;
  - a última situação, que é o texto curto mostrado no agendamento (exemplo: "Esperando aprovação do app").
- **Avisar por e-mail:** um campo guardado desde já. Ele só aparece na tela quando a Peça 3 existir.
- **Próxima execução e última execução.**

### Como dispara

Um relógio dentro do Ziv confere a cada 30 s os agendamentos vencidos, em todas as empresas, percorrendo os esquemas de cada empresa do portal. Para cada agendamento, faz as mesmas checagens do Rodar (`startRun` e `allowExecution`). A lógica do Rodar é separada da requisição HTTP e vira uma função que os dois usam. Assim um agendamento nunca passa por um caminho com menos checagens do que o botão.

O relógio também roda a cada volta a expiração de pedidos parados (`ExpireStaleRuns`). Hoje ela só acontece quando alguém abre uma tela ou um Agente pede trabalho.

- **Um disparo por vez:** dois servidores ou um reinício não podem disparar o mesmo horário duas vezes. A trava fica no banco: quem atualiza `next_run_at` primeiro dispara.
- **Servidor desligado na hora:** quando o Ziv volta, roda **uma** vez o que ficou para trás, se o atraso for de até 1 hora. Os horários mais antigos são registrados como perdidos e o agendamento segue para o próximo. Isso é decisão do builder.

**O que acontece em cada situação** (decisões do builder):

| Situação | Cria execução no histórico? | Conta como falha? | Ação |
|---|---|---|---|
| Rodou (com resultado, sem arquivos ou com erro do app) | Sim, como hoje | Sim, só se terminou em erro | — |
| Computador desligado ou desconectado | Sim, status novo **Perdida** ("O computador X estava desligado") | Sim | — |
| Agente desatualizado | Sim, **Perdida** ("Atualize o Agente") | Sim | — |
| A anterior ainda está rodando, ou a pasta está em uso por outro app | Não, só a situação do agendamento ("Pulada: a anterior ainda estava rodando") | Não | — |
| Execução de apps pausada pela empresa (Admin), ou app esperando aprovação | Não, só a situação ("Esperando aprovação do app") | Não | Não pausa: volta a rodar sozinho quando o bloqueio acabar |
| A pessoa perdeu o acesso, foi removida, ou o app foi suspenso | Não | — | **Pausa** com o motivo |
| O app mudou e os campos guardados não servem mais | Não | — | **Pausa** com o motivo |
| 5 falhas seguidas | — | — | **Pausa** ("Falhou 5 vezes seguidas") |

Uma execução que alguém parou à mão não conta como falha. A pessoa vê o motivo da pausa e reativa com um clique depois de corrigir.

O status **Perdida** é novo em `runs`. Ele aparece no histórico, em Minhas execuções e nos contadores como as Interrompidas. `computer_id` continua obrigatório, porque o agendamento tem computador fixo.

### Quem vê e mexe

- Cada pessoa vê e edita os seus agendamentos.
- Quem cuida do app (dono e reserva) e o administrador veem todos os agendamentos do app e podem pausar ou apagar qualquer um. Para isso usam a mesma checagem de quem cuida do app que já existe (`Can.Access` mais administrador).
- Ninguém vê os arquivos guardados no agendamento de outra pessoa.

### Telas

- **Página do app:** um botão "Agendar" ao lado de Rodar. Ele abre o mesmo formulário de Rodar com a seção "Quando rodar" (frequência, horário, dias, próximas 3 execuções) e o botão "Salvar agendamento".
- **Página do app, seção "Agendamentos":** lista os agendamentos com:
  - o nome ou a frequência por extenso ("Todo dia às 7h", "A cada 5 minutos");
  - quem criou;
  - próxima e última execução, com o resultado;
  - os botões Pausar, Editar e Apagar.

  Quem vê o quê está em "Quem vê e mexe".
- **Histórico e Minhas execuções:** selo "agendada", como hoje existe "pelo Claude".
- **Início (Home):** quem tem agendamento pausado pelo Ziv vê um aviso.

O visual segue o DESIGN.md (preto e branco, ADR 020). Antes de fechar, o Henry vê uma prévia local, como na landing.

### Auditoria

Criar, editar, pausar, reativar e apagar agendamentos gera registro de auditoria. As pausas feitas pelo próprio Ziv são registradas com o autor "Ziv (agendamento)". Hoje a auditoria está presa a requisições HTTP; como adaptar fica com o builder.

## Peça 2 — Rodar no servidor (desenho resumido, detalhar antes de começar)

- O Ziv decide sozinho se o app pode rodar no servidor. Pode quando é um Script em Python ou Node:
  - sem `.exe`;
  - sem campo `folder` com caminho do Windows;
  - sem `fixed_file` fora da pasta do app;
  - sem dependência do Windows.

  Quando não pode, explica o motivo e o agendamento vai para o PC.
- O agendamento ganha o campo "Onde roda: servidor / computador de <pessoa>". O padrão é o servidor quando possível.
- Cada execução roda numa caixa (Podman) descartável, criada a partir do código da versão aprovada que o Ziv já guarda.
  - Tem limite de memória e de tempo, e uma de cada vez, porque o servidor tem 1 GB.
  - Os resultados ficam guardados no servidor por um tempo definido e são baixados direto do Ziv.
- Abrir para todas as empresas ou só para a principal fica para decidir nesse momento (hoje os apps compartilhados são só da principal, ADR 015).
- O News Scrapper vira o primeiro caso real.

## Peça 3 — E-mail (resumida)

- Cada agendamento tem "Mandar e-mail: não / sempre / só se der erro" e a lista de pessoas da empresa que recebem.
- O e-mail leva o resultado e o link para a execução. Arquivos pequenos podem ir anexos.
- Precisa de um serviço de envio. O Ziv não envia e-mail hoje. O Henry escolhe e cria a conta no momento da Peça 3.

## O que fica a critério do builder

- Os nomes de tabelas e rotas e a migração.
- Como calcular a próxima execução e os testes de virada de dia, mês e fuso.
- Como o relógio percorre as empresas.
- Como separar a lógica do Rodar para os dois usarem.
- O texto exato das mensagens, seguindo o tom do Ziv. A revisão de texto passa pelo plugin, como na landing.
