# 012 - Vários apps abertos ao mesmo tempo, em pastas separadas

**Status:** Accepted

## Context
Até o Agente 0.3.1 havia uma trava única: com um app aberto pelo Hub, o segundo era recusado ("Já tem um app aberto"). Henry quer rodar mais de um app por vez. O Agente descobre os arquivos de uma execução comparando a pasta antes e depois; com dois apps gravando na mesma pasta, essa comparação daria a um app os arquivos do outro.

## Decision
- O Agente (0.3.2) guarda as execuções abertas por número e aceita várias ao mesmo tempo. O Parar fecha só a execução pedida.
- Uma execução de app não abre numa pasta que seja igual, de dentro ou de fora da pasta de outra execução aberta no mesmo computador. A comparação ignora maiúsculas, barra final e o estilo da barra (`agentproto.FoldersOverlap`).
- O Hub confere isso ao receber o Rodar, olhando as execuções aguardando ou abertas naquele computador, e responde 409 com o nome do app que está na pasta. A tela mostra "Trocar pasta". O Agente confere de novo, porque dois cliques quase juntos podem passar pelo Hub.
- Scripts não conflitam entre si (cada um grava só na sua `Execucao-N`), mas conflitam com apps que olham a pasta de fora.
- A atualização automática espera um momento sem nenhuma execução aberta. Enquanto espera, novas execuções continuam abrindo; só nos segundos da troca o Agente recusa ("está se atualizando").

## Rationale
Recusar a pasta em uso é a única opção que nunca entrega arquivo errado e não muda onde os apps .exe gravam. Henry escolheu esse caminho.

## Trade-offs
**Pros:**
- A lista de arquivos de cada execução continua confiável.
- Apps em pastas diferentes rodam sem limite.
- A atualização não bloqueia o trabalho de ninguém.

**Cons:**
- Rodar o mesmo app duas vezes na mesma pasta exige escolher outra pasta.
- Quem deixa apps abertos o tempo todo só recebe a atualização quando fecha todos.
- Agentes 0.3.1 ou anteriores seguem com um app por vez até se atualizarem.

## Alternatives Considered
- **Deixar abrir na mesma pasta e avisar que os arquivos podem estar misturados:** a lista ficaria pouco confiável.
- **Subpasta própria por execução para apps .exe:** mudaria onde os arquivos aparecem hoje e quebraria apps que esperam a pasta escolhida.
- **Atualização que bloqueia novas execuções até todos os apps fecharem:** atrapalharia o trabalho do dia.
