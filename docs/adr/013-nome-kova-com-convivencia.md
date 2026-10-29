# ADR 013 — O App Hub vira Kova, com convivência dos nomes antigos

Data: 2026-10-29. Status: aceita. Desenho: `docs/superpowers/specs/2026-10-28-kova-identidade-design.md`.

## Contexto
O nome "App Hub" era genérico e não se sustentava para vender a outras empresas. O Henry escolheu **Kova** (de *cove*, enseada protegida), um símbolo novo e um visual neutro com uma só cor viva, o coral. Ele também decidiu trocar os nomes técnicos `app-hub` por `kova`, sem quebrar nenhum app já registrado ou compartilhado.

## Decisão
O nome do produto mora numa constante só: `frontend/src/lib/brand.ts` e `backend/internal/brand`. Nos nomes técnicos, o Kova **escreve** o nome novo e **lê** o novo e o antigo. Se existirem os dois, vale o novo. Nada é recriado: adotar é sempre renomear.

| O quê | Novo | Antigo, ainda reconhecido |
|---|---|---|
| Manifesto do app | `kova.yaml` | `app-hub.yaml` (um app que só tem o antigo continua salvando nele) |
| Marcadores "escrito pelo Kova" | `# Kova — manifesto do app`, `<!-- kova:ficha -->` | `# App Hub — manifesto do app`, `<!-- app-hub:ficha -->` |
| Lixeira e temporários das versões | `.kova-lixeira`, `.kova-tmp` | `.app-hub-lixeira`, `.app-hub-tmp` (ficam fora das versões) |
| Etiquetas das caixas e imagens | `kova.app`, `kova.deployment`, `localhost/kova/` | `app-hub.app`, `app-hub.deployment`, `localhost/apphub/` |
| Rede de cada app | `kova-apps-<id>` | `app-hub-apps-<id>` (removida junto ao deixar de compartilhar) |
| Nome das caixas | `kova-<slug>-<id>` | — (as antigas são reconhecidas pela etiqueta) |
| Contêiner do Postgres | `kova-db` | `app-hub-db`, renomeado sozinho na subida (`podman rename`) |
| Pasta de dados dos compartilhados | `~/.local/share/kova/shared` | `~/.local/share/app-hub/shared`, renomeada sozinha na subida |
| Arquivo do agente baixado | `KovaAgente.exe` | — |

A adoção na subida (`internal/shared/adopt`) só acontece quando `APPS_DB_CONTAINER` e `SHARED_DATA_DIR` não foram definidos. Se o nome novo já existe, ela não toca em nada.

**Ficam como estão de propósito**, porque ninguém os vê e trocá-los apagaria o estado das pessoas:
- as preferências salvas no navegador (`apphub-*`);
- a instalação do agente no Windows: pasta `%LOCALAPPDATA%\AppHub` (e `~/.apphub-agente`), chave de inicialização `AppHubAgente` e os arquivos `AppHubAgente*.exe` no disco;
- a variável `APPHUB_INITIAL`, a classe `AppHubFolderPicker`, os cabeçalhos `X-Apphub-*` que o porteiro manda aos apps e os arquivos `.apphub-*` nas pastas dos apps;
- o banco de testes `apphub_test` e o módulo Go `github.com/HenryKropp/app-hub` (até o Henry renomear o repositório).

## Quando a convivência pode acabar
Quando nenhum app registrado tiver mais só `app-hub.yaml`, nenhuma caixa no ar usar `app-hub.app` e todos os servidores já tiverem subido uma vez com o Kova. Aí uma migração forçada renomeia os arquivos que sobraram e o código de leitura dos nomes antigos sai.

## Consequências
- Qualquer comando ou script fora do app que use `app-hub-db` pelo nome (por exemplo `podman start app-hub-db`) precisa passar a usar `kova-db`.
- Documentos históricos (planos e desenhos antigos, ADRs 001 a 012, `Documentação/`) não foram reescritos.
