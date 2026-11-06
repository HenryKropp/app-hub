# 024 - Agendar scripts para rodar sozinhos no computador da pessoa

**Status:** Accepted

## Context
O objetivo do Ziv é as pessoas pararem de rodar apps à mão no PC. Tarefas como o News Scrapper (a cada 5 minutos) e relatórios diários hoje dependem do Agendador de Tarefas do Windows, fora do Ziv: ninguém vê se rodou, se deu erro ou quem cuida.

Em 2026-11-06 o Henry pediu "agendar apps/tarefas". Ele decidiu:
- valer para os dois tipos de tarefa (a cada poucos minutos e em horário fixo);
- rodar no servidor quando der e, quando não der, no PC pelo Agente;
- mostrar toda execução no histórico, com e-mail opcional;
- deixar agendar quem pode rodar o app;
- oferecer opções prontas com mínimo de 5 minutos e prévia das próximas execuções.

A entrega 1 é a agenda com execução no PC pelo Agente. As entregas 2 (servidor) e 3 (e-mail) vêm depois.

Desenho completo: `docs/superpowers/specs/2026-11-06-agendar-apps-design.md`.

## Decision
- **Só scripts por enquanto.** Hoje só scripts recebem os dados antes de rodar. Um app que pergunta coisas numa janela depois de abrir não pode ser agendado ("Este app pergunta coisas depois de abrir, então não dá para agendar.").
- **Frequências prontas, em horário de Brasília:**
  - a cada N minutos (mínimo 5, máximo 1440) ou a cada N horas (1 a 24), contados a partir da meia-noite: "a cada 15 minutos" roda às :00, :15, :30 e :45;
  - todo dia às HH:MM;
  - em dias da semana às HH:MM;
  - todo mês no dia 1 a 28 às HH:MM.
  
  A regra existe em Go (`internal/schedule`) e em TypeScript (`lib/schedules.ts`), para a tela mostrar a prévia sem esperar o servidor.
- **Computador fixo por agendamento.** A pessoa escolhe um dos computadores dela. Se ele estiver desligado na hora, a execução fica **Perdida** (status novo `missed`) no histórico. Não procuramos outro computador, porque a pasta de saída e os arquivos são daquela máquina.
- **O mesmo caminho do Rodar.** `launchRun` (em `handler/launch.go`) faz todas as checagens do botão:
  - bloqueio da empresa ou do app;
  - campos;
  - versão do Agente;
  - pasta em uso;
  - código igual ao pacote salvo.
  
  O relógio usa a mesma função. Assim um agendamento nunca pula uma checagem que o botão faz.
- **Relógio por empresa.** Cada empresa ligada ao portal tem um relógio que olha os agendamentos a cada 30 segundos.
  - Cada horário é "reservado" no banco antes de disparar (`ClaimSchedule`, que só muda `next_run_at` se ele ainda for o antigo). Assim dois relógios nunca disparam o mesmo horário duas vezes.
  - Antes de olhar, o relógio encerra as execuções vencidas, para "a anterior ainda está rodando" não durar para sempre.
- **Atraso de até 1 hora.** Se o Ziv estava desligado, ao voltar ele roda uma vez o que ficou para trás, desde que o atraso seja de até 1 hora. Com atraso maior, o agendamento só anota "Perdeu o horário de dd/mm HH:MM: o Ziv estava desligado" e segue para o próximo horário.
- **5 falhas seguidas pausam.** Contam como falha as execuções com Erro, Interrompida e Perdida. A sequência é contada a partir das últimas execuções do agendamento desde a última mudança dele (editar ou reativar zera a conta). Execução parada à mão não conta e não quebra a sequência.

### O que acontece em cada situação

| Situação | Cria execução no histórico? | Conta como falha? | Ação |
|---|---|---|---|
| Rodou (com resultado, sem arquivos ou com erro do app) | Sim | Só se deu erro | — |
| Computador desligado | Sim, **Perdida** ("O computador X estava desligado") | Sim | — |
| Agente desatualizado | Sim, **Perdida** ("Atualize o Agente em Configurações › Meu computador") | Sim | — |
| A anterior ainda está rodando, ou a pasta está em uso | Não. Só a situação ("Pulada: …") | Não | — |
| Empresa pausou as execuções, ou o app espera aprovação | Não. Só a situação ("Esperando aprovação do app") | Não | Volta sozinho quando o bloqueio acaba |
| A pessoa perdeu o acesso, ou o app foi suspenso | Não | — | **Pausa** com o motivo |
| O app mudou e os campos guardados não servem mais | Não | — | **Pausa** ("O app mudou e os campos guardados não servem mais. Edite o agendamento.") |
| 5 falhas seguidas | — | — | **Pausa** ("Falhou 5 vezes seguidas") |

### Quem vê e mexe
- Cada pessoa vê, edita, reativa e apaga os agendamentos dela.
- Responsável, reserva, líder da área e administrador veem todos os agendamentos do app e podem pausar ou apagar qualquer um. Eles não veem os dados de outra pessoa: campos, arquivos e pasta só vão para o dono.
- Uma pausa automática aparece na Home do dono ("{app}: agendamento pausado — {motivo}"). Quem reativa é o dono.
- O que o Ziv faz sozinho (pausar, registrar Perdida) entra na auditoria com a rota `schedule` e sem pessoa.

### Telas
- **Botão Agendar** ao lado de Rodar, só em scripts que a pessoa pode rodar. Ele abre o diálogo com os mesmos campos do Rodar, a pasta, "Quando rodar" (com a prévia das próximas 3 execuções), o computador e um nome opcional.
- **Seção Agendamentos** logo abaixo de Execuções, na página do app. Ela some quando o app não tem agendamentos.
- **Selo "agendada"** na página da execução.

## Consequences
- O PC precisa estar ligado, com o Agente aberto, na hora de rodar. Rodar no servidor (entrega 2) tira essa dependência dos apps que não precisam do PC.
- Uma execução com erro que termina depois do disparo só pausa o agendamento no horário seguinte: pode haver um disparo a mais depois da 5ª falha.
- Uma execução agendada que vence sem o PC pegar fica Interrompida, com o texto de hoje. O texto "estava desligado" aparece só quando o PC já está offline na hora de disparar.
- A faixa de execuções abertas (RunStrip) ainda não distingue a execução agendada.
- Pendentes: rodar no servidor (entrega 2), e-mail opcional por agendamento (entrega 3, precisa escolher um serviço de envio) e as ferramentas de agendar pelo Claude (MCP).
