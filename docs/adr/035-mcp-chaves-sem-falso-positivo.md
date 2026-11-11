# 035 - MCP: chaves sem falso positivo (F2)

**Status:** Accepted

## Context
No caso do "baixar-relatorios" (2026-11-11), o Ziv pediu chaves que o app não precisava e o cofre acumulou lixo:
- `os.environ.get("BR_RAIZ", "")` virou chave obrigatória, embora o código tenha um padrão.
- Uma constante (`os.environ.get(VAR_DEMO)`) virou uma chave chamada VAR_DEMO.
- A lista de ajustes do app com tela só crescia: um nome que o código deixou de ler continuava pedido.
- Numa versão nova de app com tela, o `keys` da ficha era ignorado (`mudou: false`).
- O `enviar_chaves` guardava todo nome do .env, inclusive de outros programas da mesma pasta, e o Claude não tinha como apagar.

## Decision
- **Leitura do código:** só conta nome entre aspas (`os.environ["X"]`, `os.getenv("X")`, `os.environ.get("X")`, `process.env.X`). Uma leitura com padrão (`, "a"` no Python, `||`/`??` no JS) é opcional. Atribuição (`os.environ["X"] = …`) não é leitura.
- **Lista refeita a cada versão:** ao publicar, a lista de ajustes junta o que o código lê agora, as `keys` declaradas e o que já tem valor ou padrão. Um nome que ninguém lê, sem valor e sem padrão, sai. As `keys` decidem o obrigatório e o rótulo.
- **`keys` numa versão nova** vale também para app com tela.
- **`enviar_chaves`** ignora o nome que o código não lê, não está declarado e ainda não está guardado ("o código do app não lê esse nome"). Se o código não pode ser lido, ou não lê nenhum nome, nada é ignorado por isso.
- **Ferramenta nova `apagar_chave`:** sem `confirmado`, só pergunta; com a confirmação da pessoa, apaga pela mesma rota do site (`DELETE /api/apps/{id}/keys/{name}`), que só o dono ou um administrador usa. Ela é marcada como destrutiva e avisa quando o app no ar precisa reiniciar.

## Rationale
Cada chave pedida à toa custa uma ida e volta com a pessoa, que não sabe se pode ignorar. Ler o código com as mesmas regras do Python e do Node elimina os casos vistos. Apagar com confirmação resolve o lixo que já existe.

## Trade-offs
**Pros:**
- O app com tela pede só o que o código usa de fato.
- O .env compartilhado entre programas não suja o cofre.

**Cons:**
- Um nome lido de forma indireta (montado em tempo de execução) não é visto; ele precisa ser declarado em `keys`.

## Alternatives Considered
- Botão "não é chave, ignorar" no cofre: adiado. Com a leitura corrigida e a lista refeita, ele vira raro; volta se aparecer caso real.
- Apagar sem confirmação: descartado. O valor apagado não volta.
