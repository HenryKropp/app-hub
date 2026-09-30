# Fase 4 — Agente do Hub e execução de apps com janela (desenho)

**Data:** 2026-09-30 · **Status:** aprovado em conversa, aguardando revisão do documento
**Primeiro alvo:** Eq. Research Files Prep (`Ferramenta CVM.exe`)

## 1. Objetivo

A pessoa clica em **Rodar** no Hub, o app abre no Windows do computador dela, ela usa a janela normalmente e fecha. O Hub então mostra os arquivos que a execução gerou, em grupos, com download, e guarda o registro da execução (quem, quando, duração, pasta, computador, versão, arquivos).

A execução passa por um **Agente do Hub**, um programa pequeno instalado no Windows. É o mesmo formato que o Módulo 2 (nuvem) vai precisar, porque um Hub na internet não consegue abrir janelas no computador de ninguém.

### Decisões do Henry (2026-09-30)

| Tema | Decisão |
|---|---|
| Onde salvar | Por padrão, na pasta original do app (como hoje). A pessoa pode escolher outra pasta antes de abrir. Guardar os resultados dentro do Hub fica para a nuvem, e o desenho deixa esse caminho aberto |
| Outra pasta | O Agente copia o `.exe` para a pasta escolhida, abre a partir dela e apaga a cópia ao fechar. O app não muda |
| Resultado | Todo arquivo novo ou alterado, agrupado pelas `outputs` do manifesto, mais um grupo "Outros arquivos" |
| Abordagem | Agente no Windows já nesta fase (opção B), em vez de o Hub chamar o Windows direto |
| Aparência do Agente | Invisível, sem ícone. O status aparece só no Hub |
| Downloads | Os arquivos ficam só na pasta. Ao clicar em Baixar, o Agente envia na hora |

## 2. Como fica para a pessoa

**Instalar (uma vez por computador)**
1. Em **Meu computador**, a pessoa clica em **Instalar o Agente** e baixa `AppHubAgente.exe`.
2. Abre o arquivo uma vez. O Agente se instala, abre o navegador em `/conectar?codigo=XXXX` e o Hub pergunta "Conectar este computador à sua conta?". Ela clica em **Conectar**, sem digitar nada.
3. Daí em diante o Agente liga sozinho com o Windows, sem janela nem ícone. O Hub mostra "Seu computador está conectado".

**Rodar**
1. Na página do app, **Rodar** abre a caixa "Onde salvar os resultados?", preenchida com a pasta do app. **Escolher outra…** abre a janela de pastas do Windows (pelo Agente).
2. Depois de **Abrir app**, o Hub leva para a página da execução: "Rodando, a janela está aberta no seu computador", com o tempo correndo.
3. A pessoa usa o app e fecha a janela.
4. A página mostra os arquivos em grupos (Data Pack, Documentos da CVM, FRE cortado, Outros arquivos), cada um com **Baixar**, e cada grupo com **Baixar tudo** (.zip).

**Quando algo dá errado**
- Sem Agente conectado: o Rodar avisa "Seu computador não está conectado" e leva para Meu computador.
- O computador reinicia, ou o Agente reinicia, com a janela aberta: a execução fica **Interrompida**.
- A pasta escolhida não existe ou não aceita gravação: o Agente recusa antes de abrir e a execução fica **Erro**, com a mensagem.
- Já existe na pasta escolhida um arquivo com o nome do `.exe`: **Erro** ("Já existe um Ferramenta CVM.exe nessa pasta").
- O arquivo foi apagado ou movido depois: Baixar mostra "Esse arquivo não está mais na pasta".
- O app sempre abre no computador de quem clicou em Rodar (o computador conectado mais recente da pessoa).

## 3. Peças

```
Navegador ──► Hub (Go, WSL) ◄── Agente (Go, Windows)
                 │   ▲              │
                 ▼   │              ▼
              Postgres          Ferramenta CVM.exe + pasta
```

### 3.1 Agente (`agent/`, novo módulo Go no repositório)

- Compilado para Windows (`GOOS=windows`, sem cgo, `-H=windowsgui` para não abrir console). Endereço do Hub fixado na compilação (`-X main.hubURL`), `http://localhost:8080` no piloto.
- **Instalação** (primeira execução fora da pasta de instalação): copia a si mesmo para `%LOCALAPPDATA%\AppHub\AppHubAgente.exe`, registra a inicialização com o Windows em `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, inicia a cópia instalada e sai.
- **Conexão** (sem chave salva): `POST /api/agent/pair/start` com o nome do computador → recebe `code`, `secret` e a `approve_url` (montada pelo Hub com `BASE_URL`), abre o navegador nessa URL e consulta `POST /api/agent/pair/poll` com o `secret` até receber a chave. Grava a chave em `%LOCALAPPDATA%\AppHub\agente.json`.
- **Laço principal:** `GET /api/agent/next` com a chave. O Hub segura o pedido por até 25 s e devolve o próximo pedido ou nada. Cada chamada conta como sinal de vida.
- Ao iniciar, avisa `POST /api/agent/hello` (versão, nome do computador). O Hub marca como Interrompida qualquer execução desse computador que estava Rodando.
- Um pedido por vez para `run`. `choose_folder` e `send_file` podem acontecer enquanto um `run` está aberto (cada pedido roda na sua goroutine; só um `run` por vez).
- A parte que não depende do Windows (foto da pasta, comparação, zip, cliente HTTP) fica em pacotes testáveis no Linux. Registro, diálogo de pasta e abertura do navegador ficam atrás de build tags `windows`.

### 3.2 Pedidos (Hub → Agente)

| Tipo | Conteúdo | O Agente faz | Resposta |
|---|---|---|---|
| `choose_folder` | pasta inicial | Abre a janela de pastas do Windows (PowerShell `FolderBrowserDialog`, `-STA`) | caminho escolhido ou cancelado |
| `run` | id da execução, pasta do app, nome do `.exe`, pasta de saída (formato Windows) | Confere a pasta (existe, aceita gravação com um arquivo de teste), fotografa, copia o `.exe` se a pasta for outra, avisa `started`, abre com `exec.Command(...).Wait()` e diretório de trabalho = pasta de saída, fotografa de novo, apaga a cópia e envia a lista | `started` e depois `finished` (código de saída, lista de arquivos) ou `failed` (mensagem) |
| `send_file` | id da execução, caminho relativo (ou um grupo, para zip) | Confere que o caminho está na lista que ele mesmo enviou naquela execução e que o arquivo existe. Envia o conteúdo em streaming para `POST /api/agent/jobs/{id}/upload` | conteúdo, ou `missing` |

**Foto da pasta:** percorre a pasta de saída recursivamente e guarda caminho relativo, tamanho e data de modificação dos arquivos. **Novo** = caminho que não existia. **Alterado** = tamanho ou data diferente. Ignora a cópia temporária do `.exe`. A lista de cada execução fica guardada pelo Agente em `%LOCALAPPDATA%\AppHub\runs\{id}.json` para a checagem do `send_file`.

**Cópia do .exe:** só vale para `.exe` de arquivo único (PyInstaller `--onefile`, caso deste app). Um `.exe` que precise de DLLs ao lado não funciona copiado. Fica anotado como limite da fase.

### 3.3 Hub

**Banco (migration `003_agents_runs.sql`)**
- `computers`: id, person_id, name, token_hash (SHA-256; a chave nunca fica guardada), agent_version, last_seen_at, created_at, revoked_at.
- `pairings`: code, secret_hash, computer_name, person_id (preenchido na aprovação), expires_at (10 min).
- `agent_jobs`: id, computer_id, kind, payload JSONB, status (`pending` / `taken` / `done` / `failed`), result JSONB, datas.
- `runs`: id, app_id, person_id, computer_id, output_folder, status (`waiting` / `running` / `done` / `no_files` / `error` / `interrupted`), exit_code, error_message, app_version (data e tamanho do `.exe`), started_at, finished_at, created_at.
- `run_files`: id, run_id, rel_path, size, modified_at, change (`new` / `changed`), group_label.

**Grupos:** ao receber `finished`, o Hub compara cada `rel_path` com as `outputs[].pattern` do manifesto (glob por pasta; `*` não atravessa `/`; `**` = qualquer número de pastas; grupos calculados pela ficha atual). O primeiro padrão que casa dá o grupo. Os que não casam vão para "Outros arquivos".

**Conectado** = computador não revogado com `last_seen_at` há menos de 60 s.

**Pode rodar** = app `kind: desktop` cujo `entrypoint` termina em `.exe`. Os outros tipos continuam sem botão nesta fase.

**Rota de download:** o navegador chama `GET /api/runs/{id}/files/{fileId}` (ou `.../groups/{n}/zip`). O Hub confere a permissão, cria o pedido `send_file` e espera o Agente enviar (até 60 s para começar). Um encontro em memória liga o upload do Agente à resposta do navegador. O Hub é um processo só no piloto.

**Endereços**
- Pessoa (sessão atual): `GET /api/computers/me`, `DELETE /api/computers/{id}`, `POST /api/pairings/{code}/approve`, `GET /api/pairings/{code}`, `GET /api/agent/download`, `POST /api/choose-folder`, `POST /api/apps/{id}/runs`, `GET /api/apps/{id}/runs`, `GET /api/runs/{id}`, downloads acima.
- Agente (chave do computador, cabeçalho `Authorization: Bearer`): `pair/start`, `pair/poll`, `hello`, `next`, `jobs/{id}/result`, `jobs/{id}/upload`.

**Distribuição do Agente:** `scripts/build-agent.sh` compila `agent/` para Windows em `dist/agent/AppHubAgente.exe` (ignorado pelo git). `GET /api/agent/download` serve esse arquivo (`AGENT_BINARY_PATH`). O Hub conhece a versão esperada do Agente e avisa "Atualize o Agente" quando a versão conectada é outra.

**Endereço do Hub visto do Windows:** o WSL2 repassa `localhost` do Windows para o WSL, então o Agente fala com `http://localhost:8080` (o Go) e a `approve_url` usa o `BASE_URL` (o Vite, `http://localhost:5173`, no desenvolvimento). A primeira tarefa do plano confirma isso na máquina.

## 4. Telas

- **Meu computador** (novo item no menu): conectado ou não, nome do computador, versão do Agente, "visto há X s", botões **Instalar o Agente** e **Desconectar**. Sem computador: passo a passo de instalação.
- **Conectar** (`/conectar?codigo=`): nome do computador e botões **Conectar** e **Cancelar**. Código vencido: aviso para abrir o Agente de novo.
- **Página do app:** botão **Rodar** (só para apps que podem rodar), caixa "Onde salvar", e seção **Execuções** com as últimas 20 (quando, quem, duração, situação, quantos arquivos).
- **Página da execução** (`/execucoes/:id`): cabeçalho (app, quem, quando, pasta, computador); enquanto `waiting`/`running`, "Rodando…" com o tempo correndo e atualização a cada 2 s; ao terminar, os grupos com contagem, **Baixar**, **Baixar tudo**, e a lista recolhida acima de 10 arquivos por grupo. Erro ou Interrompida mostram a mensagem em português.

Visual: segue o estilo de fichário atual e passa pelo `frontend-design`.

## 5. Segurança

- O Agente só executa o que vem do Hub autenticado com a sua chave, e o Hub só manda `run` para apps registrados que podem rodar.
- `send_file` só para arquivos em `run_files` daquela execução, checado no Hub **e** no Agente.
- A chave do computador vale para uma pessoa só. O Hub guarda só o hash. Desconectar revoga na hora.
- Aprovar uma conexão exige estar logado. O código vence em 10 min e é de uso único.
- Quem pode ver e baixar uma execução nesta fase: quem rodou e os administradores. As permissões por app entram na Fase 6.

## 6. Testes

**Automáticos**
- Go (Hub): conexão (start, approve, poll, código vencido e reuso), chave inválida ou revogada, `next` com espera, ciclo da execução (`waiting` → `running` → `done` / `no_files` / `error`), Interrompida no `hello`, grupos pelo manifesto, bloqueio de download fora de `run_files`, encontro do upload com o download, "pode rodar".
- Go (Agente, no Linux): foto e comparação (novo, alterado, ignorar a cópia), zip de grupo, recusa de `send_file` fora da lista, pasta inexistente ou sem gravação, cliente contra um Hub falso (`httptest`).
- Vitest: Meu computador (conectado ou não), Conectar, caixa Rodar, página da execução (rodando, concluída com grupos, erro).

**De verdade, no computador do Henry**
1. Compilar e baixar o Agente pelo Hub, instalar, conectar.
2. Rodar o Eq. Research Files Prep na pasta original. Gerar um Data Pack de uma empresa pequena, fechar, conferir os grupos e baixar o Data Pack.
3. Rodar de novo escolhendo outra pasta. Conferir que a janela abre com a pasta escolhida e que a cópia do `.exe` some ao fechar.
4. Desconectar e ver o Rodar avisar.

## 7. Fora desta fase

Scripts com formulário, apps web, ambiente de Python automático, repetir execução, ícone na bandeja, atualização automática do Agente, cópia dos resultados para o Hub, `.exe` que precisa de DLLs ao lado, mais de um computador por pessoa escolhido na hora.

## 8. Documentos a atualizar junto

- ADR 004: execução pelo Agente do Hub no Windows (substitui o "interop" previsto na ADR 001, que passa a apontar para a 004).
- PRD 3.3: Agente, pasta de saída escolhida, downloads pelo Agente.
- TASKS Fase 4: tarefas novas (Agente, conexão, execução, resultados, downloads).
