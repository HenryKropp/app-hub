# Atualização automática do Agente (desenho)

**Data:** 2026-09-30 · **Status:** aprovado em conversa, aguardando revisão do documento
**Contexto:** etapa 1 do App Hub (ADR 005). O Agente hoje é a versão 0.1.1 (ADR 004).

## 1. Objetivo

Quando sai uma versão nova do Agente no Hub, o Agente de cada computador percebe, baixa, se troca e volta a se conectar, sem a pessoa fazer nada. Se a versão nova não funcionar, a anterior volta sozinha.

### Decisões do Henry (2026-09-30)

| Tema | Decisão |
|---|---|
| Quem avisa | O Hub: quando o Agente se conecta, o Hub compara a versão e deixa um pedido "atualize-se" na mesma fila de pedidos (opção 1 de 3) |
| Quando troca | Só quando não há app aberto pelo Hub; nunca no meio de uma execução |
| Se der errado | A versão anterior fica guardada e volta sozinha se a nova não se conectar em 2 minutos. Depois de 3 falhas na mesma versão, o Hub para de tentar e mostra o botão "Atualize o Agente" |
| Primeira vez | A versão 0.1.1 não sabe se atualizar. O Henry instala à mão, uma última vez, a primeira versão que sabe (0.2.0) |

## 2. Como fica para a pessoa

- Nada muda no uso. Numa troca, o Agente fica desconectado por alguns segundos.
- Em **Meu computador**:
  - durante a troca: "Atualizando o Agente para a versão X…";
  - depois: a versão nova e "atualizado há N minutos" (por 24 horas);
  - depois de 3 falhas: "Não conseguimos atualizar o Agente sozinho. Baixe e instale de novo", com o botão de hoje.
- Se um pedido de Rodar chegar durante a troca, ele espera na fila como hoje (vence em 2 minutos).

## 3. Como funciona

### 3.1 Versão e arquivo
- A versão continua em `agentproto.Version`, que passa para `0.2.0`. **Toda versão publicada do Agente precisa de um número novo.** Sem número novo, nada é atualizado.
- O Hub serve o arquivo que está em `AGENT_BINARY_PATH` (como hoje) e calcula o SHA-256 e o tamanho dele quando monta o pedido de atualização.
- A regra "a versão do arquivo é a mesma do Hub" se mantém porque `scripts/build-agent.sh` compila o Agente a partir do mesmo código do Hub.

### 3.2 Quem sabe se atualizar
- `Hello` ganha o campo `can_update: true`, que a 0.2.0 em diante envia. O Hub grava isso em `computers.can_update` (migração 004).
- O Hub **só** pede atualização a computadores com `can_update`. Um 0.1.1 continua recebendo só o aviso "Atualize o Agente".

### 3.3 Quando o Hub pede
O Hub cria um pedido `update` (novo tipo em `agent_jobs`, migração 004) para o computador quando **tudo** isto vale:
- a versão do Agente é diferente de `agentproto.Version` e `can_update` está ligado;
- o arquivo do Agente existe em `AGENT_BINARY_PATH`;
- não há pedido `update` pendente ou em andamento para esse computador;
- há menos de 3 pedidos `update` com falha para essa mesma versão alvo.

O Hub confere isso em dois momentos:
- no `hello`, quando o Agente liga;
- no começo de cada `next`, porque, depois de o Hub ser reiniciado com uma versão nova, o Agente continua pedindo pedidos sem mandar `hello` de novo.

O pedido leva `{version, sha256, size}`.

### 3.4 O que o Agente faz
1. Recebe o pedido `update` e espera até não ter nenhum app aberto pelo Hub. Outros pedidos continuam sendo atendidos enquanto isso.
2. Baixa o arquivo em `GET /api/agent/binary` (novo, autenticado com a chave do computador, não com o login da pessoa) para `%LOCALAPPDATA%\AppHub\update\AppHubAgente.exe`.
3. Confere o tamanho e o SHA-256. Se não baterem, apaga o arquivo e responde `failed` ("O arquivo baixado veio diferente do esperado").
4. Copia o Agente atual para `AppHubAgente.anterior.exe`.
5. Abre o arquivo baixado com `--update` e fica esperando ser fechado.

O arquivo baixado, aberto com `--update`, faz a troca e vigia o resultado:
1. Usa o `install()` de hoje: pede ao Agente antigo para fechar (`/quit`), copia-se por cima de `AppHubAgente.exe` e abre a cópia instalada.
2. Pergunta à cópia instalada, pela porta local (`127.0.0.1:47631`, rota nova `GET /status`), se ela já conseguiu se conectar ao Hub (`hello` com sucesso). Tenta por até 2 minutos.
3. Se deu certo, termina.
4. Se não deu, fecha a cópia instalada (`/quit`, ou encerra o processo se ela não responder), copia `AppHubAgente.anterior.exe` de volta para `AppHubAgente.exe` e abre a anterior.

### 3.5 Como o Hub fecha o pedido
Hoje o `hello` marca como `failed` todo pedido em andamento, porque o Agente reiniciou. Para o pedido `update`, o `hello` passa a decidir antes:
- se a versão do `hello` é a versão alvo do pedido: **done**;
- se não é (a anterior voltou): **failed**, e conta como uma falha para essa versão alvo.

Contam como falha só os pedidos `update` que o Agente chegou a pegar (`taken_at` preenchido). Um pedido que venceu na fila, com o computador desligado, não conta. Se o computador reiniciar enquanto o Agente espera um app fechar, o pedido também fecha como failed e conta. Isso é raro, e o limite de 3 absorve.

## 4. Segurança
- O arquivo só vem do Hub ao qual o Agente está conectado, pela rota com a chave do computador. A conferência do SHA-256 garante que ele chegou inteiro e é o que o Hub mandou.
- Computador desconectado no Hub não baixa nada (a chave já é recusada hoje).
- Nenhum outro programa pode pedir atualização: o pedido só nasce no Hub.

## 5. Fora do escopo
- Ícone perto do relógio, janela própria e instalador (etapas 2 e 3 da ADR 005).
- Assinatura digital (etapa 4).
- Botão "Procurar atualizações" e escolha de quando atualizar: a troca é sempre automática.
- Voltar a uma versão mais antiga de propósito: o Agente segue a versão do Hub, mas o Hub nunca pede uma versão que já falhou 3 vezes.

## 6. Testes
**Automáticos (Go):**
- Hub:
  - cria o pedido só quando a versão é diferente, `can_update` está ligado e o arquivo existe;
  - não duplica o pedido;
  - para depois de 3 falhas;
  - o `hello` fecha o pedido como done ou failed conforme a versão;
  - `/api/agent/binary` recusa quem não tem chave válida;
  - Meu computador mostra os estados de atualização.
- Agente:
  - espera o app fechar antes de trocar;
  - recusa arquivo com SHA-256 ou tamanho errado;
  - o vigia volta para a versão anterior quando a nova não confirma a conexão, e termina quando confirma (com lançador e cliente falsos, como nos testes de hoje).
- Tela (Vitest): os três estados em Meu computador.

**De verdade, no PC do Henry:**
1. Instalar a 0.2.0 à mão.
2. Publicar a 0.2.1 e ver em Meu computador que ela se trocou sozinha.
3. Publicar uma versão de teste que não se conecta e ver a anterior voltar.
