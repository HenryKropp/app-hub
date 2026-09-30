# 004 - Execução pelo Agente do Hub no Windows

**Status:** Accepted

## Context
O Hub roda no WSL (ADR 001), mas os apps com janela, como o Eq. Research Files Prep (`Ferramenta CVM.exe`), precisam do Windows. No Módulo 2 o Hub vai estar num servidor na internet, e de lá não dá para abrir uma janela no computador de ninguém. Nos dois casos, alguém no próprio computador da pessoa precisa abrir o app, esperar ele fechar e dizer ao Hub o que foi gerado.

## Decision
Um programa pequeno, o **Agente do Hub** (`AppHubAgente.exe`), fica instalado no Windows de cada pessoa:
- **Invisível**, sem ícone. Liga sozinho com o Windows (chave `Run` do usuário) e se instala em `%LOCALAPPDATA%\AppHub`.
- **Quem começa a conversa é sempre o Agente:** ele pergunta ao Hub "tem pedido para mim?" (`GET /api/agent/next`, com espera de até 25 s). Assim a conexão atravessa firewall e funciona igual na nuvem.
- **Conexão sem digitar código:** na primeira vez, o Agente abre o navegador no Hub e a pessoa, já logada, clica em Conectar. Cada computador recebe uma chave própria. O Hub guarda só o hash dela, e desconectar corta na hora.
- **Pedidos guardados no Postgres** (`agent_jobs`): abrir app, escolher pasta, enviar arquivo. Pedidos que ninguém pegou em 2 min vencem e a execução fica Interrompida.
- **Onde salvar:** por padrão na pasta do app. Em outra pasta, o Agente copia o `.exe` para lá, abre a partir dela e apaga a cópia ao fechar. O app não muda.
- **Resultados:** o Agente compara a pasta antes e depois (nome, tamanho, data) e manda a lista. O Hub agrupa pelas `outputs` do manifesto.
- **Downloads sob demanda:** os arquivos ficam na pasta. Ao clicar em Baixar, o Hub pede ao Agente e repassa o que ele envia para o navegador. O Agente só envia arquivos que ele mesmo registrou como resultado daquela execução.
- O código do Agente fica no mesmo módulo Go do Hub (`backend/cmd/agent`, `backend/internal/agent`), para os dois usarem as mesmas definições (`internal/agentproto`).

## Rationale
- É o formato de que o Módulo 2 precisa. Na nuvem, troca-se só o endereço do Hub.
- Henry preferiu construir o Agente já nesta fase, em vez de o Hub chamar o Windows direto pelo WSL.
- Não duplica os resultados, que podem ter centenas de MB por execução.

## Trade-offs
**Pros:**
- Mesmo desenho no piloto e na nuvem.
- Nenhuma mudança nos apps.
- Não guarda cópias dos resultados nem expõe outros arquivos do computador.

**Cons:**
- É mais um programa para instalar e atualizar em cada computador. Atualiza sozinho desde a 0.2.0 (spec 2026-09-30); a 0.1.x precisa de uma última instalação manual.
- Só dá para baixar com o computador ligado e o arquivo ainda na pasta.
- A cópia do `.exe` para outra pasta só funciona para `.exe` de arquivo único (PyInstaller `--onefile`), que é o caso deste app.

## Alternatives Considered
- Hub chamando `powershell.exe` pelo WSL (interop): mais simples no piloto, mas não serve para a nuvem. O Henry escolheu o Agente.
- Ícone perto do relógio: adiado. O status aparece só no Hub (Meu computador).
- Agente enviando todos os resultados ao Hub: adiado para o Módulo 2, porque duplica espaço.
