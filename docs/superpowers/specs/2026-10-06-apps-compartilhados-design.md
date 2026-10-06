# Apps compartilhados — desenho

**Data:** 2026-10-06 · **Status:** aprovado em conversa pelo Henry; aguardando revisão deste documento
**Especificação do produto:** v3, seções 1.3 ("Apps pessoais e compartilhados") e 2.0 a 2.6
**Primeiro caso de teste:** Sistema Alis v2 (`/home/henrykroppalisinvest/sistema-alis`), mais um app Streamlit pequeno sem receita

## 1. Objetivo

Um app marcado como **compartilhado** roda uma vez só, no mesmo servidor do Hub, numa caixa isolada (contêiner). Todos abrem o mesmo endereço pelo botão **Abrir** e veem os mesmos dados. O Hub cuida do banco do app, das configurações e senhas, de quem pode entrar, de religar se cair, de desligar sem uso, das cópias de segurança, da troca de versão e dos gastos.

**Sucesso:** registrar um app compartilhado, clicar em Abrir em dois navegadores diferentes e os dois verem os mesmos dados; deixar dormir e acordar; publicar uma versão nova e voltar.

Tudo é construído e testado no PC do Henry (Hub no WSL, Podman). **Subir para a nuvem é a última etapa e não faz parte deste desenho.** O desenho já funciona igual num servidor Linux na nuvem.

## 2. Decisões do Henry (2026-10-06)

| Tema | Decisão |
|---|---|
| Para que serve | Capacidade genérica do Hub; o Sistema Alis é só o primeiro exemplo. Colocá-lo de verdade para a equipe fica para depois |
| Onde roda | Na nuvem, junto com o Hub. Mas subir para a nuvem é a **última** etapa: antes, tudo roda e é testado no PC do Henry |
| Pouco trabalho para o criador | O Hub confere se o app está pronto e resolve sozinho o que der: receita automática para tipos conhecidos, teste de verdade antes de liberar. O que exige mudar código vira um texto pronto para o Claude |
| Versão nova | Botão "Publicar versão nova", clicado por quem cuida do app. Nunca automático |
| Login | O Hub é sempre o porteiro e informa ao app quem é a pessoa. Apps com login próprio continuam funcionando (dois logins) até serem adaptados |
| Caminho | A: o Hub liga os apps no próprio servidor, cada um num contêiner, com um espaço separado por app no banco |
| Economia | App dorme sem uso e acorda no primeiro acesso; limite de memória e processamento por app; motor trocável |
| Gastos | Tela para o administrador ver o uso do servidor e o custo estimado por app |

## 3. Peças

Cada peça é um pacote Go com uma função só, testável sozinho.

| Pacote | Função | Depende de |
|---|---|---|
| `internal/shared/readiness` | Checagem: lê a pasta e o manifesto e devolve a lista de itens (ok, resolvido pelo Hub, falta) | `scanner`, `manifest` |
| `internal/shared/recipes` | Monta a receita (Dockerfile) para Streamlit, Python web (Flask/FastAPI) e Node | `scanner` |
| `internal/shared/engine` | Motor: interface + implementação com o Podman (linha de comando) + motor de mentira para testes | — |
| `internal/shared/appdb` | Banco por app: cria e apaga o banco e o usuário; faz e restaura cópias (`pg_dump`/`pg_restore`) | Postgres |
| `internal/vault` | Guarda configurações e senhas por app; senhas criptografadas (AES-256-GCM) com a chave `HUB_SECRET_KEY` | Postgres |
| `internal/shared/gateway` | Porteiro: recebe os pedidos nos endereços dos apps, confere login e acesso, acorda o app e repassa | `access`, `engine` |
| `internal/shared/supervisor` | Vigia: saúde, religar, dormir, medir uso, cópias diárias | `engine`, `appdb` |
| `internal/shared/deploy` | Testar, publicar, voltar: orquestra receita, motor, banco e cofre | todos acima |
| `internal/handler` (arquivos novos) | Rotas da API para as telas | `deploy`, `vault`, `supervisor` |

### 3.1 Motor (interface)

```go
type Engine interface {
    Build(ctx context.Context, b BuildSpec) (imageID string, log string, err error)
    Start(ctx context.Context, s StartSpec) (containerID string, err error) // limites, env, volume, rede
    Stop(ctx context.Context, containerID string) error
    Remove(ctx context.Context, containerID string) error
    RemoveImage(ctx context.Context, imageID string) error
    Inspect(ctx context.Context, containerID string) (State, error)          // rodando, saiu, código de saída
    Stats(ctx context.Context, containerIDs []string) ([]Usage, error)        // memória e CPU
    Logs(ctx context.Context, containerID string, lines int) (string, error)
}
```

A implementação com o Podman chama a linha de comando (`podman build/run/stop/rm/inspect/stats/logs`), com tempo limite em cada chamada. Nenhuma outra parte do Hub conhece o Podman. O motor de mentira guarda o estado em memória e permite simular falhas.

### 3.2 Banco por app

- Mesmo servidor Postgres do Hub (economia), num **banco separado por app** (`app_<id>`) com um **usuário próprio** (`app_<id>`), senha gerada pelo Hub e guardada no cofre.
- O usuário do app é dono só do próprio banco: `REVOKE CONNECT ... FROM PUBLIC` em todos os bancos de apps e no banco do Hub; `CONNECTION LIMIT 20`.
- O app recebe `DATABASE_URL` pronto. O motor de troca permite levar os bancos para outro servidor depois.
- Para criar bancos e usuários, o Hub usa uma conexão de administrador própria (`APPS_DB_ADMIN_URL`), separada da conexão normal do Hub.

### 3.3 Porteiro (endereço e entrada)

- Cada app compartilhado tem um **endereço próprio por subdomínio:** `<slug>.<host do Hub>`. No PC: `sistema-alis.localhost:8080` (o Chrome e o Edge já mandam qualquer `*.localhost` para o próprio computador). Na nuvem: `sistema-alis.<domínio do Hub>`. O endereço vem de `SHARED_APPS_DOMAIN` (padrão `localhost:8080`).
- O próprio servidor do Hub atende esses endereços: se o `Host` do pedido é o subdomínio de um app, o pedido vai para o porteiro. Os demais seguem para o Hub, como hoje.
- **Entrada sem segundo login no Hub:** cookies do Hub não valem nos subdomínios. Quando a pessoa chega sem a sessão do app, o porteiro manda para `/api/shared/authorize?app=<slug>&volta=<caminho>` no Hub. O Hub confere a sessão e o acesso (`access`: quem enxerga o app), gera um código de uso único (vale 60 s) e devolve para o subdomínio. O porteiro troca o código por um cookie próprio daquele app (HttpOnly, 12 h). Sem sessão no Hub, a pessoa faz login no Hub e volta sozinha.
- **Acesso conferido de novo** a cada 5 minutos por sessão: quem perdeu o acesso (app restrito depois, pessoa removida) é barrado sem esperar o cookie vencer.
- **Quem é a pessoa:** o porteiro apaga cabeçalhos `X-AppHub-*` vindos do navegador e envia `X-AppHub-User-Id`, `X-AppHub-User-Email`, `X-AppHub-User-Name` e `X-AppHub-User-Role`. Apps novos podem usar isso no lugar de login próprio.
- **Repasse:** proxy reverso (`httputil.ReverseProxy`), com suporte a WebSocket (o Streamlit precisa). Sem limite de tamanho de envio além do do app.
- **App dormindo:** o porteiro pede ao vigia para ligar e mostra a página "Ligando o app…" (atualiza sozinha a cada 2 s), até a saúde responder. Pedidos de API (não HTML) recebem 503 com `Retry-After`.
- **App que caiu ou está parado:** página clara em português, com o nome de quem cuida do app.
- Cada pedido atualiza o "último acesso" do app (em memória, gravado a cada minuto), usado para dormir.

### 3.4 Vigia

Um laço único, a cada 30 s:

1. **Saúde:** para cada app que deveria estar no ar, confere o contêiner (`Inspect`) e o endereço de saúde do manifesto (padrão `/`; responde < 500 em até 5 s).
2. **Religar:** se caiu, religa. Até 3 tentativas em 10 min; depois marca **caiu**, para de tentar e mostra o aviso na ficha e no catálogo para quem cuida do app. "Religar" na ficha zera a contagem.
3. **Dormir:** app sem acesso há 30 min (`idle_minutes`, ajustável por app) e sem "sempre ligado" é desligado. O banco nunca dorme.
4. **Medir:** a cada minuto, memória e CPU de cada app ligado entram em `shared_usage` (uma linha por app por minuto; guarda 90 dias, depois resume por hora).
5. **Cópias diárias:** às 3h, uma cópia do banco de cada app; apaga as diárias com mais de 14 dias. Cópias "antes da versão N" e "antes de restaurar" ficam 30 dias.

Ao ligar o Hub, o vigia reconcilia: o que deveria estar no ar e não está, liga; contêineres do Hub que não correspondem a nenhum app, para e remove.

### 3.5 Configurações e senhas (começo do cofre)

- O manifesto declara as configurações de que o app precisa (`shared.env`): nome, rótulo, se é senha, se é obrigatória, valor padrão.
- Quem cuida do app preenche os valores na ficha. Senhas são guardadas criptografadas e nunca voltam para a tela (aparece "••••• · trocar").
- O Hub preenche sozinho: `DATABASE_URL` (se o app pede banco), `PORT`, `BASE_URL` (o endereço do app), `APPHUB_DATA_DIR` (pasta que não se perde).
- Trocar uma configuração pede religar o app; a ficha avisa e oferece "Religar agora".
- `HUB_SECRET_KEY` vem do ambiente do Hub (32 bytes em base64). Sem ela, o Hub liga, mas apps compartilhados ficam indisponíveis, com uma mensagem clara para o administrador.

### 3.6 Receitas automáticas

Usadas só quando a pasta não tem `Dockerfile`. Ficam guardadas no Hub (`shared_deployments.recipe`) e **nunca** são gravadas na pasta do app.

| Tipo detectado | Como reconhece | Receita |
|---|---|---|
| Streamlit | `streamlit` nas bibliotecas e um `.py` que importa streamlit | Python fixado no manifesto (padrão 3.12), `pip install -r requirements.txt`, `streamlit run <entrada> --server.port $PORT --server.address 0.0.0.0 --server.headless true` |
| Python web | `flask`, `fastapi` ou `django` | Mesmo começo; comando pelo tipo (`gunicorn`, `uvicorn`), confirmado pelo criador na checagem |
| Node | `package.json` com script `start` | Node fixado (padrão 22), `npm ci`, `npm run build` se existir, `npm start` |

Sem `requirements.txt`, o Hub monta a lista a partir das bibliotecas que o diagnóstico encontrou e mostra para o criador confirmar.

### 3.7 Checagem

Roda ao marcar "compartilhado", ao abrir a parte de publicação da ficha e antes de testar. Cada item tem estado e, quando falta algo, o que fazer:

| Item | Resolve sozinho quando | Se faltar |
|---|---|---|
| Receita | Tipo conhecido (3.6) | Texto para o Claude: "escreva um Dockerfile que…" |
| Porta | Receita automática (usa `$PORT`) ou `EXPOSE` no Dockerfile | Perguntar a porta |
| Banco | App lê `DATABASE_URL` → Hub cria o banco | — |
| Dados em arquivo (SQLite, CSV, planilha gravados pelo app) | — | Aviso: duas pessoas gravando juntas perdem dados; texto para o Claude trocar por banco. Não bloqueia; pode marcar "estou ciente" |
| Pastas do Windows ou OneDrive no código (`C:\`, `/mnt/c`) | — | Aviso: no servidor essas pastas não existem; texto para o Claude usar `APPHUB_DATA_DIR` ou envio de arquivo |
| Configurações pedidas pelo código (`os.getenv`, `process.env`) | Vira lista em `shared.env` para preencher | — |
| Senha escrita no código | — | Aviso, como hoje, com texto para o Claude mover para uma configuração |
| Coisas que não cabem no contêiner (ex.: Playwright/Chromium chamado por script e ausente da receita) | — | Aviso específico, com texto para o Claude |

Os textos para o Claude ficam numa caixa com "Copiar". Quando o registro pelo Claude (MCP) existir, ele recebe esta mesma lista.

## 4. Fluxos

### 4.1 Tornar um app compartilhado
1. Na ficha (parte "Para quem cuida do app") ou no registro, quem cuida do app escolhe **"Compartilhado: roda uma vez só e todos usam o mesmo endereço"**. Só para apps do tipo "App com tela no navegador".
2. Aparece a checagem (3.7) e os campos: configurações, "sempre ligado", limites (padrão: 512 MB de memória, 1 processador).
3. **Testar:** o Hub copia a pasta da versão atual para uma área de montagem (sem `.git`, `.venv`, `node_modules` e as saídas do manifesto, respeitando `.dockerignore` se houver), monta a imagem e liga uma caixa de teste com um banco de teste vazio (apagado no fim). Mostra o andamento ("Montando…", "Ligando…", "Conferindo se abre…") e, no fim, "Abriu" com o link do teste (`<slug>--teste.<domínio>`, pelo porteiro, só para quem pode publicar; a caixa de teste é desligada em 30 min ou ao publicar) ou o erro em português com as últimas linhas da montagem.
4. **Publicar:** cria o banco do app, liga a caixa de verdade e mostra o endereço. O cartão do catálogo passa a ter **Abrir**.

### 4.2 Abrir
Cartão ou ficha → **Abrir** → nova aba no endereço do app → porteiro (3.3) → app. Pontinho de status no cartão: no ar · dormindo · ligando · caiu · parado.

### 4.3 Versão nova
1. A conferência de versões que já existe (ADR 006) detecta mudança no código. Para app compartilhado, a ficha mostra **"Versão nova pronta para publicar"** (versão no ar × versão na pasta).
2. **Publicar versão nova:** cópia do banco ("antes da versão N") → monta a nova imagem → liga a nova caixa ao lado, no mesmo banco → confere a saúde → o porteiro passa a mandar para a nova → para a antiga (guarda a imagem anterior).
3. Se a montagem ou a saúde falhar, a antiga continua no ar e a ficha mostra o erro.
4. **Voltar para a versão anterior:** religa a imagem anterior. Oferece também "voltar os dados para a cópia de antes da versão N", com o aviso de que o que foi gravado depois se perde.

### 4.4 Cópias de segurança
Lista na ficha (data, tipo, tamanho). **Restaurar:** faz uma cópia "antes de restaurar", para o app, restaura, religa. **Baixar** a cópia (quem cuida do app).

### 4.5 Servidor e gastos (administrador)
Item novo na barra lateral, para a permissão "cuidar do servidor":
- **Servidor:** memória total e em uso, disco total e em uso (Hub + bancos + cópias + imagens).
- **Por app:** estado, horas ligado no mês, memória média e máxima, CPU média, tamanho do banco, tamanho das cópias, **custo estimado no mês**.
- **Custo estimado:** o administrador informa o custo mensal do servidor (R$). O valor é dividido entre os apps pela memória reservada × horas ligado no mês; o que sobra aparece como "Hub e espaço livre".
- Ações: mudar limites, "sempre ligado", parar e religar qualquer app.

### 4.6 Parar e remover
- **Parar** (quem cuida do app): fica parado até "Ligar". O porteiro mostra "parado por <pessoa>".
- **Deixar de compartilhar:** para, remove as caixas e imagens; o banco e as cópias ficam por 30 dias e depois são apagados (aviso claro antes).

## 5. Quem pode o quê

Usa as regras da Etapa 2 (`internal/access`):

| Ação | Quem |
|---|---|
| Abrir | Quem enxerga o app |
| Marcar compartilhado, testar, publicar, versão nova, voltar, configurações, cópias, parar | Quem pode voltar versões do app (responsável, reserva, líder da área, acesso extra, cargo com `restore_any_app` nos apps que enxerga) |
| Ver as mensagens do app (últimas 200 linhas) e o uso | Quem vê a parte técnica do app |
| Servidor e gastos | Permissão nova de cargo **`manage_server`** ("Cuidar do servidor"). O Administrador recebe na migração |

As regras contra escalada continuam valendo para a permissão nova.

## 6. Dados (migração 008)

- `roles`: acrescenta `manage_server` ao cargo `admin`.
- `shared_apps` (um por app compartilhado): `app_id` (PK), `slug` (único), `status` (`draft`, `running`, `sleeping`, `starting`, `crashed`, `stopped`), `always_on`, `idle_minutes`, `memory_mb`, `cpus`, `health_path`, `port`, `wants_db`, `db_name`, `current_deployment_id`, `previous_deployment_id`, `restart_count`, `last_access_at`, `status_changed_at`, `status_message`.
- `shared_deployments`: `id`, `app_id`, `version_id` (liga ao histórico da ADR 006), `kind` (`test`, `live`), `recipe` (texto, se automática), `image_id`, `container_id`, `status` (`building`, `failed`, `ready`, `live`, `retired`), `build_log` (últimos 64 KB), `error_message`, `created_by`, `created_at`.
- `app_config`: `app_id`, `name`, `value_plain` ou `value_cipher` + `nonce`, `is_secret`, `updated_by`, `updated_at`.
- `shared_backups`: `id`, `app_id`, `kind` (`daily`, `pre_deploy`, `pre_restore`, `manual`), `path`, `size_bytes`, `deployment_id`, `expires_at`, `created_at`.
- `shared_usage`: `app_id`, `at` (minuto), `mem_bytes`, `cpu_pct`; índice por (`app_id`, `at`).
- `server_settings`: `monthly_cost_cents`, `updated_by`, `updated_at` (linha única).
- Linha do tempo do app (ADR 006): publicar, voltar, restaurar, parar e "caiu" entram como eventos.

## 7. Manifesto (`app-hub.yaml`, formato v0.3, campos opcionais)

```yaml
kind: web
shared:
  enabled: true
  port: 8501            # opcional com receita automática
  health_path: /
  database: postgres    # ou ausente
  always_on: false
  idle_minutes: 30
  memory_mb: 512
  cpus: 1
  env:
    - name: ADMIN_EMAILS
      label: E-mails dos administradores
      required: true
    - name: SMTP_PASSWORD
      label: Senha do e-mail
      secret: true
```

Valores nunca vão para o manifesto, só os nomes. `cmd/validate-manifest` passa a validar o bloco.

## 8. Telas

- **Catálogo:** apps compartilhados mostram **Abrir** (ícone de link externo) no lugar de Rodar e o pontinho de status. Mesmo estilo de hoje (violeta só no botão principal).
- **Ficha, parte de uso:** linha "No ar em `sistema-alis.localhost:8080` · Abrir" com o status; se caiu, o aviso e quem cuida.
- **Ficha, "Para quem cuida do app":** nova parte **"Compartilhado"** com: checagem, Testar/Publicar, versão no ar × versão nova, Voltar, Configurações, Cópias de segurança, "sempre ligado", limites, uso (gráfico simples de memória nas últimas 24 h), mensagens do app, Parar/Religar, Deixar de compartilhar.
- **Servidor e gastos:** tela nova (4.5).
- **Páginas do porteiro:** "Ligando o app…", "App parado", "App com problema", "Sem acesso", no estilo do Hub, servidas no endereço do app.
- Celular: todas as telas novas funcionam com 16 px de margem e sem rolagem lateral.

## 9. Ambiente

| Variável | Para quê | Padrão no PC |
|---|---|---|
| `HUB_SECRET_KEY` | Criptografia do cofre | gerada uma vez e guardada no `.env` |
| `APPS_DB_ADMIN_URL` | Criar bancos e usuários dos apps | superusuário do `app-hub-db` |
| `APPS_DB_HOST_FOR_CONTAINERS` | Como os contêineres chegam ao Postgres | `host.containers.internal:5433` |
| `SHARED_APPS_DOMAIN` | Domínio dos endereços dos apps | `localhost:8080` |
| `SHARED_DATA_DIR` | Áreas de montagem, pastas dos apps e cópias | `~/.local/share/app-hub/shared` |
| `CONTAINER_ENGINE` | Motor | `podman` |

Contêineres do Hub recebem o rótulo `app-hub.app=<id>` e a rede `app-hub-apps`. Pasta que não se perde por app: `SHARED_DATA_DIR/apps/<id>/data`, montada em `/data` (`APPHUB_DATA_DIR=/data`).

## 10. Erros

- Toda falha vira frase em português com o próximo passo. O detalhe técnico fica no `hub.log` e, para quem vê o técnico, em "Ver detalhes".
- Montagem falhou: últimas 40 linhas da montagem + "o que fazer" quando o Hub reconhece o erro (biblioteca que não instala, porta errada, arquivo que falta) + texto para o Claude.
- Podman ausente ou parado: os apps compartilhados mostram "O servidor de apps não está disponível"; o resto do Hub segue funcionando.
- Disco quase cheio (> 90%): aviso em Servidor e gastos; as cópias diárias continuam, apagando primeiro as mais antigas.

## 11. Testes

- **Go, unidade:** receitas (cada tipo), checagem (cada item, com pastas de exemplo), cofre (cifra/decifra, senha nunca volta), cálculo de custo, regras de permissão.
- **Go, integração com o banco de testes:** banco por app (criar, isolamento: o usuário de um app não conecta no banco de outro nem no do Hub), cópia e restauração, migração 008.
- **Go, com o motor de mentira:** publicar, versão nova com falha na saúde (antiga continua), voltar, vigia (religar 3 vezes e marcar caiu, dormir, acordar pelo porteiro, reconciliar ao ligar).
- **Go, porteiro:** sem sessão → autoriza → cookie; sem acesso → página "Sem acesso"; cabeçalhos `X-AppHub-*` vindos de fora são apagados; WebSocket repassado; perda de acesso barrada em até 5 min.
- **Go, Podman de verdade (opcional, pulado se não houver Podman):** montar e ligar um app mínimo, medir, parar.
- **Vitest:** parte "Compartilhado" da ficha, botão Abrir e status no catálogo, Servidor e gastos.
- **Fotos (Playwright, só leitura):** telas novas, claro/escuro/celular.
- **Teste de verdade com o Henry:** (1) app Streamlit de exemplo sem receita; (2) Sistema Alis v2 com receita e login próprios; dois navegadores veem os mesmos dados; dormir e acordar; versão nova e voltar; restaurar cópia; Servidor e gastos com o custo informado.

**Limite conhecido:** no Sistema Alis, gerar PDF não funciona no contêiner (a receita dele não traz Node/Playwright/Chromium). A checagem aponta, com o texto para o Claude.

## 12. Fora desta entrega

- Subir o Hub para a nuvem (última etapa, com endereço fixo e https, login de verdade, o Agente lendo as pastas no lugar do Hub e a montagem a partir do que o Agente envia).
- Avisos por e-mail; vários servidores; registro pelo Claude (MCP); tarefas agendadas; apps compartilhados de outros tipos além de "App com tela no navegador".
- Colocar o Sistema Alis de verdade para a equipe (escolher qual dos dois bancos vale).
