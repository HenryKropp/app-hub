# 006 - Histórico do código guardado no Hub, não na pasta do app

**Status:** Accepted

## Context
A Fase 5 (PRD 3.6) pede um histórico de cada app: mudanças no código, execuções e mudanças na ficha, numa linha do tempo só, com um botão para voltar o código a uma versão anterior. As pastas dos apps ficam no OneDrive. A do Files Prep tem 2,3 GB, quase tudo PDFs que o próprio app baixa, e muitos arquivos ficam "só na nuvem". O Hub hoje lê a pasta direto, pelo WSL (ADR 001). Depois vai para um servidor central (ADR 005).

## Decision
- O Hub guarda o histórico no próprio banco. Cada versão tem a lista de arquivos (caminho, tamanho, data, SHA-256, código ou binário) num campo JSON. O conteúdo dos arquivos de código fica em `code_blobs`, guardado uma vez só por SHA-256.
- Só o código-fonte tem cópia (texto até 1 MB). De binários, como o `.exe`, o Hub guarda só a impressão digital. As saídas do manifesto e as pastas de lixo (`.venv`, `node_modules`…) ficam de fora.
- A conferência compara tamanho e data. Só lê o arquivo quando um dos dois mudou, para não baixar arquivos "só na nuvem".
- Para voltar uma versão, o Hub reescreve os arquivos de código na pasta. Antes disso, guarda o estado atual como versão de segurança. Arquivos a mais vão para `.app-hub-lixeira`, nunca são apagados. Um arquivo que vai ser reescrito, mas cujo conteúdo atual não tem cópia no Hub (virou binário, ou ficou de fora da conferência), também vai para a lixeira antes.
- Todo acesso à pasta passa pela interface `versions.Folder`. Quando o Hub for para o servidor central, o Agente implementa a mesma interface.
- No Linux, a varredura abre cada pasta pelo nome relativo à pasta de cima, não pelo caminho inteiro. No `/mnt/c` do WSL, cada busca pelo caminho inteiro custa uns 10 ms. Com essa mudança, abrir a ficha do Files Prep caiu de 7–9 s para cerca de 2 s.

## Rationale
- O Hub não pode bagunçar a pasta da pessoa ("não destrutivo"), e o histórico precisa continuar existindo mesmo se a pasta mudar de lugar.
- Guardando por SHA-256, uma versão nova custa só os arquivos que mudaram.
- O Henry escolheu guardar só o código-fonte: o `.exe` sai dele, recompilando.

## Trade-offs
**Pros:**
- Nenhum arquivo do Hub dentro da pasta do app, fora a lixeira, que só aparece quando alguém restaura.
- Funciona com o OneDrive sem baixar os arquivos que estão só na nuvem.
- Fica pronto para o Agente ler a pasta no lugar do Hub.

**Cons:**
- Voltar a versão não traz o `.exe` de volta: é preciso recompilar. O Hub avisa na prévia.
- O banco cresce com o código guardado. Não há limpeza de versões antigas nesta fase.
- O Hub não sabe quem editou um arquivo no disco: marca o responsável do app como "provável".

## Alternatives Considered
- Repositório git dentro da pasta do app: mexe na pasta da pessoa e no OneDrive (milhares de arquivos `.git` sincronizando). Descartado pela regra "não destrutivo".
- Copiar a pasta inteira a cada versão: 2,3 GB de saídas por cópia.
- Guardar também o `.exe`: 75 MB por versão. O Henry escolheu só o código-fonte.
