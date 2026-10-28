# 011 - Parar um app aberto pelo Hub

**Status:** Accepted

## Context

O Henry pediu um botão Parar em 2026-10-05. Até aqui, um app aberto pelo Hub só fechava quando a pessoa fechava a janela no Windows. Os apps da empresa costumam abrir outros programas (um `.exe` que chama Python, um `.bat` que abre o Streamlit). Fechar só o programa principal deixaria esses outros abertos, e a execução ficaria presa no Hub.

## Decision

- Um pedido novo para o Agente, `stop`, com o número da execução. O Agente 0.3.1 avisa no `hello` que sabe parar (`can_stop`), e o Hub só oferece o botão para esses computadores.
- No Windows, o Agente põe o app num *job object* logo depois de abri-lo. Parar encerra o job inteiro, ou seja, o app e tudo o que ele abriu. O job não fecha nada se o Agente sair: os apps continuam abertos, como antes.
- Parar fecha na hora, sem pedir ao app para salvar. Antes, o Hub pede confirmação.
- A execução parada vira o estado novo `stopped` ("Parada"). O Agente ainda lista os arquivos que o app criou até ali, e eles ficam disponíveis como numa execução normal.
- Um pedido que o Agente ainda não pegou é cancelado só no Hub.
- Só a pessoa que rodou pode parar, porque o app está aberto no computador dela.
- O Hub espera até 15 s pela resposta do Agente. Se passar disso, responde que o pedido foi enviado, e a tela mostra "Parando…" até a execução fechar.

## Rationale

O job object é o jeito do Windows de agrupar processos que um programa abre, mesmo quando o processo do meio já terminou. O `taskkill /T` depende da relação pai e filho, que se perde nesses casos. Fechar com delicadeza (pedir para a janela fechar) não funciona com scripts sem janela e pode ficar preso num "Deseja salvar?" que ninguém vê.

## Trade-offs

**Pros:**
- Para qualquer tipo de app (janela, `.exe`, script) do mesmo jeito.
- Os arquivos gerados até a parada não se perdem.

**Cons:**
- O que o app não salvou se perde. A confirmação avisa isso.
- Um programa que o app abre nos primeiros milissegundos, antes de entrar no job, pode escapar. O Agente também fecha o processo principal diretamente.
- O Agente antigo precisa se atualizar (sozinho, quando nenhum app estiver aberto) para o botão aparecer.

## Alternatives Considered

- `taskkill /T /F`: perde os processos cujo pai já fechou.
- Pedir para a janela fechar e, se não fechar, forçar: mais lento, não serve para scripts, e uma pergunta escondida trava a parada.
- Reaproveitar o estado "Interrompida": ele indica uma falha (computador desligado). Parar é escolha da pessoa e merece texto e cor próprios.
