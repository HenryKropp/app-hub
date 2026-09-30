# INFRASTRUCTURE — App Hub

| Name | Image | Local Port | Env Var | Type |
|------|-------|-----------|---------|------|
| db | supabase/postgres:17.6.1.160 | 5433 | DATABASE_URL | backend |

Notas:
- Container local: `app-hub-db` (porta 5433, porque a 5432 é usada por outro projeto nesta máquina).
- O Hub lê e escreve nas pastas dos apps no Windows via `/mnt/c` (ver ADR 001). Isso não é um serviço, mas é uma dependência do ambiente.
- **Agente do Hub** (`AppHubAgente.exe`, ADR 004): instalado em cada computador Windows, não é um container. Fala com o Hub pela porta 8080. Compilado por `scripts/build-agent.sh` em `dist/agent/`; o Hub serve o arquivo pelo caminho em `AGENT_BINARY_PATH` (`.env`). Desde a 0.2.0, o Agente baixa esse arquivo sozinho por `GET /api/agent/binary` (chave do computador) quando a versão do Hub muda. Guarda a versão anterior em `%LOCALAPPDATA%\AppHub\AppHubAgente.anterior.exe` e escreve o registro em `agente.log` na mesma pasta. Não ligue um Hub de teste na porta 8080: o Agente real conversa com ele.
- Testes do backend usam o banco `apphub_test` no mesmo container (criado sozinho), nunca o banco de desenvolvimento.
