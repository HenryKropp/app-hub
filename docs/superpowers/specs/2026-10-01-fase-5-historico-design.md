# Fase 5: histórico e voltar atrás (desenho)

**Data:** 2026-10-01 · **Status:** desenho aprovado em conversa, aguardando revisão do documento
**Contexto:** PRD 3.6 (Versões e histórico) e fluxo C (Atualizar um app). Item 2 da ordem combinada depois da ADR 005.

## 1. Objetivo

Cada app ganha uma linha do tempo só, com **mudanças no código**, **execuções** e **mudanças na ficha**. Servem igualmente para duas coisas: (1) voltar rápido para um código que funcionava quando alguém, ou a IA, quebrou o app; (2) saber o que mudou, quando, e qual versão gerou cada resultado.

### Decisões do Henry (2026-10-01)

| Tema | Decisão |
|---|---|
| Objetivo | Voltar atrás e rastrear têm o mesmo peso |
| O que entra numa versão | Só código-fonte (arquivos de texto/código). De binários, como o `.exe`, só a impressão digital |
| Como nasce uma versão | Sozinha, sem perguntar. O que mais importa é ter as mudanças registradas. Descrever é opcional, a qualquer momento |
| Quem mudou | O responsável do app, com a marca "provável". Admin/criador corrige |
| Voltar atrás | Reescreve o código na pasta original, depois de confirmar. Arquivos a mais vão para uma lixeira, nunca são apagados |
| Ver como estava | Como no Google Docs: abrir uma versão, ver os arquivos inteiros e restaurar dali |
| O que se restaura | Só o código. Mudanças na ficha ficam só registradas |
| Quem lê a pasta | O Hub, direto (como hoje, pelo WSL). Isso fica atrás de uma interface, para trocar pelo Agente quando o Hub for para o servidor central (ADR 005) |

## 2. Como fica para a pessoa

- **Aba "Histórico" na ficha do app.** Uma lista da mais recente para a mais antiga, com filtro Tudo / Código / Execuções / Ficha. Mostra 50 itens e um botão "Carregar mais".
  - **Versão N:** "Mudança na pasta" ou "Restaurada da versão M"; a pessoa (ou "provável: Henry Kropp"); a data; a descrição, se houver; um resumo ("2 arquivos mudaram, 1 novo", "o .exe mudou"). Botões: **Ver mudanças**, **Ver como estava**, **Descrever**, **Voltar o código para esta versão**. A versão atual não mostra o botão de voltar.
  - **Execução:** quem rodou, quando, o resultado, "versão N", com link para a tela da execução.
  - **Ficha:** quem mudou, quando e o quê ("Responsável: Ana → Henry", "Resumo alterado"). A primeira entrada é "Ficha criada".
- **Ver mudanças:** compara com a versão anterior, arquivo por arquivo. Linhas removidas em vermelho e adicionadas em verde. Arquivo novo e arquivo removido aparecem como tal. Binários aparecem só como "mudou (75,6 MB → 75,8 MB)".
- **Ver como estava:** lista os arquivos daquela versão; ao clicar, mostra o conteúdo inteiro. O botão de restaurar fica ali.
- **Descrever:** um campo de texto e a escolha da pessoa que fez a mudança (começa com o responsável). Ao salvar, sai a marca "provável".
- Chaves escritas no código aparecem mascaradas (`••••`) em todas as telas, pela mesma regra do diagnóstico (`scanner`). A cópia guardada é a original, porque a restauração precisa dela.
- **Na ficha:** o campo "Versão" mostra "versão N · data". **Na tela da execução:** a versão usada. Execuções antigas, de antes desta fase, continuam mostrando o texto antigo (data e tamanho do `.exe`).

## 3. Como funciona

### 3.1 O que entra numa versão
- **Ficam de fora:** arquivos que batem com os padrões de `outputs` do manifesto; `__pycache__`, `.venv`, `venv`, `node_modules`, `.git`; a lixeira do Hub (`.app-hub-lixeira`); `app-hub.yaml` e `FICHA.md` (escritos pelo Hub, entram no histórico pela ficha).
- **Código (com cópia):** extensão de código/texto conhecida (`.py`, `.js`, `.ts`, `.bat`, `.cmd`, `.ps1`, `.sh`, `.vbs`, `.json`, `.yaml`, `.yml`, `.toml`, `.ini`, `.cfg`, `.txt`, `.md`, `.html`, `.css`, `.sql`, `.r`, `.ipynb`, `.spec`, `.go` e parecidas), até 1 MB, sem byte nulo nos primeiros 8 KB.
- **Binário (só impressão digital):** todo o resto, e textos acima de 1 MB. Guarda caminho, tamanho, data e SHA-256.
- Cada conteúdo é guardado uma vez só, pelo SHA-256. Uma versão nova aponta para cópias que já existem.

### 3.2 Conferência
- Acontece: ao abrir a ficha (`GET /api/apps/{id}`), ao abrir o histórico, antes de cada execução (`startRun`) e no registro de um app novo.
- Compara a pasta com a última versão. Se tamanho e data são iguais, considera igual sem ler o arquivo. Isso é importante no OneDrive, com arquivos "só na nuvem". Se mudaram, calcula o SHA-256. Arquivo novo, removido ou com SHA-256 diferente gera versão nova.
- **Versão 1:** na primeira conferência de um app que ainda não tem versão, origem "estado original".
- Uma conferência por app de cada vez (trava em memória por app, mais `UNIQUE(app_id, number)` no banco).
- Pasta inacessível: a conferência não cria nada e devolve o aviso. A ficha e o histórico continuam abrindo.
- A execução grava `version_id` com a versão conferida logo antes de rodar.

### 3.3 Mudanças na ficha
- No `saveApp` (publicar/atualizar ficha), o Hub compara a ficha antiga com a nova, campo por campo: nome, resumo, tipo, linguagem, área, responsável, reserva, e as partes do manifesto (entradas, saídas, como roda, ambiente, observações, tags, `depends_on`, `build`).
- Grava uma entrada com a pessoa, a data e a lista do que mudou. Campos curtos guardam antes e depois; partes do manifesto guardam antes e depois em JSON e aparecem como "Entradas alteradas", com detalhe ao abrir.
- Publicar sem mudar nada não gera entrada.

### 3.4 Voltar o código para a versão M
1. **Prévia** (`GET .../versions/M/restore-preview`): lista o que volta ao que era, o que volta a existir e o que vai para a lixeira; avisa se algum binário atual é diferente do daquela versão ("o .exe não volta; recompile depois").
2. Na confirmação (`POST .../versions/M/restore`), o Hub confere a pasta de novo. Se mudou, registra o estado atual como versão, a de segurança.
3. Reescreve os arquivos de código da versão M. Os arquivos de código que existem agora e não existiam em M são movidos para `.app-hub-lixeira/AAAA-MM-DD HHMMSS/<caminho>`. Nada é apagado. Binários não são tocados.
4. Confere de novo e cria a versão "Restaurada da versão M", com a pessoa. Se o resultado for igual à versão de segurança, a versão nova é criada assim mesmo, para o registro da restauração.
- Só admin e criador. Bloqueado se o app tem execução esperando ou rodando ("Espere a execução terminar").
- Falha no meio (arquivo travado, OneDrive): para, devolve quais arquivos voltaram e quais não, e cria a versão com o que ficou na pasta. A versão de segurança continua lá para refazer.

### 3.5 Partes do código
- **`internal/versions`** (novo): regras de o que entra (3.1), conferência (3.2), comparação linha a linha, prévia e restauração (3.4). Não sabe de HTTP.
- **Acesso à pasta** por uma interface pequena (listar, ler, escrever, mover). A implementação de hoje usa o disco local via `hostpath`. Depois, a do Agente implementa a mesma interface.
- **`internal/handler`**: rotas finas que chamam `versions` e o banco.
- **Mascarar chaves:** `scanner` passa a exportar uma função que mascara os valores encontrados num texto, usada nas telas de mudanças e de "ver como estava".

### 3.6 Banco (migração 005)
- `code_blobs (sha256 PK, content BYTEA, size)`: conteúdo dos arquivos de código.
- `app_versions (id, app_id, number, origin 'original'|'folder'|'restore', restored_from, author_id, author_probable, description, described_by, described_at, files_changed_at, created_at, UNIQUE(app_id, number))`.
- `app_version_files (version_id, rel_path, sha256, size, modified_at, kind 'code'|'binary')`. Só `code` tem cópia em `code_blobs`.
- `app_changes (id, app_id, person_id, created_at, changes JSONB)`.
- `runs.version_id BIGINT REFERENCES app_versions(id)`.

### 3.7 Rotas
- `GET /api/apps/{id}/history?type=&before=`: linha do tempo, de 50 em 50.
- `GET /api/apps/{id}/versions/{n}/changes`: comparação com a anterior.
- `GET /api/apps/{id}/versions/{n}/files` e `GET .../files/content?path=`: ver como estava.
- `PATCH /api/apps/{id}/versions/{n}`: descrição e autor (admin/criador).
- `GET .../versions/{n}/restore-preview` e `POST .../versions/{n}/restore` (admin/criador).

## 4. Testes
- **Go:** regras de o que entra (saídas, lixo, texto ou binário, limite de 1 MB); conferência sem mudança não cria versão; mudança de data sem mudança de conteúdo não cria versão; arquivo novo, removido e alterado; versão 1; duas conferências ao mesmo tempo criam uma versão só; comparação linha a linha; máscara de chaves; prévia; restauração com lixeira e versão de segurança; bloqueio com execução rodando; mudanças na ficha campo por campo; execução ligada à versão; linha do tempo misturando os três tipos e paginando.
- **Telas (Vitest):** histórico com filtro, ver mudanças, ver como estava, descrever, janela de confirmação da restauração.
- **De verdade, com o Henry, no Files Prep, passo a passo:** abrir o histórico (versão 1); mudar uma linha do `.py`; ver a versão 2 e a mudança; voltar para a versão 1 e conferir o arquivo na pasta.

## 5. Fora desta fase
- Restaurar a ficha.
- Conferência pelo Agente (quando o Hub for para o servidor central).
- Guardar cópia de binários e recompilar sozinho.
- Saber quem editou o arquivo no disco.
- Limpar versões antigas.
