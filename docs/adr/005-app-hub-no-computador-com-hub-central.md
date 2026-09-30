# 005 - App Hub no computador, com o Hub sempre num servidor central

**Status:** Accepted

## Context
O Henry quer que o Hub tenha cara de produto, seja fácil de instalar e atualizar e mexa bem no computador (abrir apps, escolher pastas, ler arquivos). Hoje o Hub é um servidor Go + Postgres no WSL, usado pelo navegador, e o Agente do Hub (ADR 004) é um programa invisível no Windows. No piloto só o Henry usa. Na venda, ainda não está decidido se o Hub fica na nossa nuvem ou num computador do cliente (plano "Local").

## Decision
- O "cérebro" do Hub (catálogo, fichas, histórico, cofre, MCP) fica **sempre num servidor central**: o WSL do Henry no piloto; na venda, a nossa nuvem ou um computador do cliente. O código é o mesmo nos dois casos.
- O **App Hub** é o programa instalado no Windows. É o Agente crescido: mostra o Hub numa janela própria, **com as mesmas telas do site** (sem telas próprias), e continua abrindo os apps no computador.
- O navegador continua funcionando (celular, quem não instalou).
- Etapas, cada uma com desenho, plano e teste próprios: (1) atualização automática do Agente; (2) ícone perto do relógio e janela própria; (3) instalador único; (4) assinatura digital. Mac só se algum cliente pedir.
- Ordem no roadmap (combinada em 2026-09-30): etapa 1 agora; depois Fase 5 (histórico), acabamento da ficha e execução de scripts; etapas 2 e 3 antes de o primeiro colega entrar; etapa 4 antes de vender.

## Rationale
- Um servidor central é o que resolve as dores de compartilhamento e continuidade: todos veem o mesmo catálogo, e nada some quando alguém sai.
- A janela com as mesmas telas do site dá cara de produto sem manter dois conjuntos de telas.
- A atualização automática vem primeiro porque, depois dela, todas as melhorias do App Hub chegam sozinhas.
- As etapas 2 e 3 não fazem nenhum app novo funcionar; enquanto só o Henry usa, função vem antes de aparência.

## Trade-offs
**Pros:**
- Cara de produto, instalação fácil e acesso ao computador, sem perder nuvem, celular e MCP.
- Onde o servidor fica na venda (nuvem ou cliente) pode ser decidido depois.

**Cons:**
- Dois programas para manter: o servidor e o App Hub.
- Sem assinatura digital (paga), o Windows mostra o aviso "Windows protegeu seu computador". Antivírus podem desconfiar de um programa que abre `.exe`.
- Só Windows, por enquanto.

## Alternatives Considered
- Só site no navegador: funciona e é o mais simples, mas não tem cara de produto e o Agente continua sem rosto.
- Hub inteiro instalado em cada computador: descartado. Cada PC teria o próprio catálogo, o que quebra compartilhamento e continuidade, multiplica as atualizações e separa nuvem e celular num outro produto.
- App com telas próprias (fora do navegador): descartado, porque dobra o trabalho de telas.
