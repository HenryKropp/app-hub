# INFRASTRUCTURE — App Hub

## Ambiente atual — Replit (26/10/2026)

| Componente | Execução | Configuração |
|---|---|---|
| Interface React/Vite | Arquivos compilados servidos pelo backend | Mesma origem das APIs |
| Backend Go | Workflow `Start application` | `bash scripts/run-replit.sh`, porta 5000 neste ambiente |
| PostgreSQL | Banco conectado ao projeto Replit | `DATABASE_URL`, sem valor no repositório |
| Agente Windows | Computador de cada usuário | Binário compilado para o endereço do Hub |

- Consulte [replit.md](../replit.md) para inicialização e requisitos.
- `BASE_URL` precisa corresponder à origem HTTPS acessível do Hub; é usada,
  inclusive, no link de aprovação da conexão do agente. Não reutilize o endereço
  de outro ambiente.
- `AGENT_BINARY_PATH` aponta para o binário disponibilizado pelo Hub.
- O endereço do agente é incorporado na compilação por `HUB_URL` (ou pelo domínio
  de desenvolvimento) e pode ser substituído por `APPHUB_URL` no computador.
  Ao mudar de endereço, recompile o agente; a verificação de datas do script de
  inicialização não detecta, por si só, uma mudança de domínio.
- O servidor não acessa as pastas Windows por `/mnt/c` no Replit. Scripts
  importados pelo navegador ficam no PostgreSQL e são enviados ao agente.
- A inicialização aplica as migrações embutidas. Publicação e migrações de
  produção exigem planejamento separado; um push ao GitHub não as executa.

### Acesso externo ao ambiente de desenvolvimento

No teste Windows, uma chamada externa ao endereço de desenvolvimento recebeu
redirecionamento para a proteção de acesso do Replit (`__replshield`), enquanto o
pedido feito do workspace chegou ao Hub. O navegador autenticado no Replit não
comprova que o agente tem acesso ao mesmo endereço.

Ajustar a privacidade do endereço de desenvolvimento é uma decisão de exposição
do ambiente, não uma correção das permissões do Hub. Se houver liberação pública
temporária para teste, mantenha a autenticação do Hub e restaure a proteção ao
terminar. Não trate esse endereço temporário como URL de produção.

## Piloto local original — histórico

As informações abaixo descrevem o ambiente WSL anterior, não os requisitos do
workflow atual no Replit.

| Name | Image | Local Port | Env Var | Type |
|------|-------|-----------|---------|------|
| db | supabase/postgres:17.6.1.160 | 5433 | DATABASE_URL | backend |

Notas:
- Container local: `app-hub-db` (porta 5433, porque a 5432 é usada por outro projeto nesta máquina).
- O Hub lê e escreve nas pastas dos apps no Windows via `/mnt/c` (ver ADR 001). Isso não é um serviço, mas é uma dependência do ambiente.
- **Agente do Hub** (`AppHubAgente.exe`, ADR 004): instalado em cada computador Windows, não é um container. Fala com o Hub pela porta 8080. Compilado por `scripts/build-agent.sh` em `dist/agent/`; o Hub serve o arquivo pelo caminho em `AGENT_BINARY_PATH` (`.env`). Desde a 0.2.0, o Agente baixa esse arquivo sozinho por `GET /api/agent/binary` (chave do computador) quando a versão do Hub muda. Guarda a versão anterior em `%LOCALAPPDATA%\AppHub\AppHubAgente.anterior.exe` e escreve o registro em `agente.log` na mesma pasta. Não ligue um Hub de teste na porta 8080: o Agente real conversa com ele.
- Testes do backend usam o banco `apphub_test` no mesmo container (criado sozinho), nunca o banco de desenvolvimento.

## Apps compartilhados

- **Podman** roda as caixas dos apps (`CONTAINER_ENGINE`, padrão `podman`). Rede `app-hub-apps`; toda caixa e imagem criada pelo Hub leva o rótulo `app-hub.app=<id do app>` (nome `apphub-<slug>-<implantação>`, imagem `localhost/apphub/<slug>:<implantação>`). O Hub nunca mexe em caixa sem esse rótulo; a única exceção é entrar (`exec`) no `app-hub-db` para fazer e restaurar cópias de banco.
- **`SHARED_DATA_DIR`** (padrão `~/.local/share/app-hub/shared`): `stage/` (pasta montada antes de cada publicação), `apps/<id>/data` (montada em `/data` no app) e `backups/<id>/` (cópias diárias, antes de publicar/restaurar e manuais).
- **Bancos dos apps**: cada app que pede banco ganha o banco `app_<id>` (com usuário próprio) no `app-hub-db`. Ao ligar, o Hub tira de `PUBLIC` o acesso ao seu próprio banco, para nenhum app conseguir abri-lo.
- **Variáveis** (`.env`, ver `.env.example`): `HUB_SECRET_KEY` (obrigatória para os compartilhados; gerar com `cd backend && mise x -- go run ./cmd/newkey`), `APPS_DB_ADMIN_URL` (padrão `DATABASE_URL`), `APPS_DB_HOST_FOR_CONTAINERS` (padrão `host.containers.internal:5433`), `APPS_DB_CONTAINER` (padrão `app-hub-db`), `SHARED_APPS_DOMAIN` (padrão `localhost:8080`), `SHARED_DATA_DIR`, `CONTAINER_ENGINE`. `BASE_URL` precisa estar preenchida: é para lá que o endereço do app manda a pessoa entrar (`/api/shared/authorize`).
- **Endereços**: cada app responde em `<slug>.localhost:8080` (e a caixa de teste em `<slug>--teste.localhost:8080`), servidos pelo próprio Hub na 8080. Subdomínios `*.localhost:8080` funcionam no Chrome/Edge sem configurar nada.
- Sem `HUB_SECRET_KEY` ou sem Podman o Hub liga normalmente; só os apps compartilhados ficam indisponíveis.
