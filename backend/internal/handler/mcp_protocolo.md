# Como registrar um app no Ziv pelo Claude

Você lê a pasta do app no computador da pessoa, monta a ficha (o manifesto), mostra para ela conferir e só registra depois que ela disser "pode registrar". O código vai para o Ziv num .zip, por um endereço de uso único.

## 1. Regras que nunca mudam

1. **Só leitura.** Você lê a pasta no disco e não altera, move, apaga nem roda nenhum arquivo do app. O .zip vai para a pasta temporária do computador, nunca para a pasta do app.
2. **Nunca o valor de um segredo.** Chaves, senhas e tokens aparecem só como nome + arquivo + linha. O valor nunca vai para a ficha, para a conversa nem para o Ziv, nem em parte (nada de "começa com sk-…"). Não abra o .env: basta saber que ele existe e os nomes das chaves que o código usa.
3. **Nada inventado.** Todo campo preenchido tem uma evidência no código ou na documentação. O que não tem evidência vira pergunta, não palpite.
4. **Pessoas não se deduzem do código.** O responsável é sempre quem está registrando (o Ziv coloca sozinho), então não pergunte responsável nem reserva. O nome que aparece num caminho (`C:\Users\fulano`) não diz nada sobre quem cuida do app.
5. **Um humano confirma.** Mostre o que entendeu e só registre depois do "pode registrar". Numa versão nova, só mande depois do "pode mandar".
6. **Linguagem de gente.** Resumo, entradas, saídas e pontos de atenção em português simples, para quem nunca viu o código.

## 2. Passo a passo

### Passo 0 — Já existe?
- Procure o app no Ziv com `buscar` (pelo nome e pelo assunto). Se achar um parecido, abra com `ver_app` e pergunte: "parece o mesmo que X: é uma versão nova ou outro app?".
- Se for versão nova e `ver_app` mostrar `codigo_guardado: true`, siga "Versão nova" (seção 4). Se `codigo_guardado` for false, o Ziv não guarda o código desse app: ele só pode subir de novo como app novo, e você avisa a pessoa.
- Se a pasta tem `ziv.yaml` (ou `kova.yaml`, `app-hub.yaml`), leia primeiro e mantenha o que uma pessoa já confirmou.
- Se a pasta parece material antigo (nome com "old", "antigo", "backup", "v1" com uma "v2" ao lado), sugira pular e pergunte.

### Passo 1 — Mapa da pasta
- Liste os arquivos até 4 níveis, ignorando `.git`, `.venv`, `venv`, `node_modules`, `__pycache__`, `build`, `dist` e pastas ocultas.
- Separe em código, atalhos de início (`.bat`, `.cmd`, `.vbs`, `.ps1`), configuração, dependências (`requirements.txt`, `pyproject.toml`, `package.json`), documentação e dados.
- Pastas com muitos arquivos do mesmo tipo (dezenas de PDFs, `output/`) são saídas acumuladas: anote só a quantidade e um exemplo de nome. Elas não devem subir: o pacote aceita até 100 arquivos e 10 MB.

### Passo 2 — Leitura, nesta ordem
1. Documentação (README): a intenção de quem fez; o código manda.
2. Atalhos de início: como a pessoa realmente roda o app.
3. Arquivo principal, inteiro.
4. Módulos que o principal usa, até o limite de leitura (seção 7).
5. Configuração: só quais chaves e caminhos existem, nunca os valores.
6. Dependências.
7. Arquivos de fora da pasta que o app usa: só leitura, e viram ponto de atenção.

### Passo 3 — Que tipo de app é
| Sinal no código | Tipo (`kind`) |
|---|---|
| Vários serviços (backend + frontend, Docker, banco próprio) | `system` |
| Só um `.exe` pronto | `executable` |
| Só documentos ou links | `reference` |
| Roda sozinho em horário fixo | `scheduled` |
| Abre no navegador (Streamlit, Flask, FastAPI, Dash, servidor Node) | `web` |
| Janela própria que fica aberta (tkinter com `mainloop`, PyQt) | `desktop` |
| Roda, faz o trabalho e termina | `script` |

Pelo Claude sobem só `script` e `web`. Apps de janela (`desktop`) e `.exe` não rodam no servidor: explique isso à pessoa e indique o site do Ziv, em Registrar apps. Um app que roda em horário fixo sobe como `script` e é agendado depois pela ficha no Ziv. `reference` e `system` só são registrados pelo site.

### Passo 4 — Como inicia
- `entrypoint`: o arquivo que inicia, relativo à pasta do app (ex.: `main.py`).
- `runtime`: `language`, `dependencies_file` (ex.: `requirements.txt`) e `dependencies`.
- Se houver dois candidatos a arquivo principal, pergunte.

### Passo 5 — O que o app recebe (`inputs`)
| Como aparece no código | `source` | `type` |
|---|---|---|
| Caminho escrito no código | `fixed_file` (+ `path`) | `file` ou `folder` |
| Janela de escolher arquivo ou pasta | `dialog` | `file` ou `folder` |
| Pergunta no terminal ou campo na tela | `prompt` | `text`, `number`, `date` ou `choice` |
| Parâmetro na linha de comando (`argparse`, `sys.argv`) | `argument` | conforme o uso |
| Campo dentro da janela de um app `desktop` | `window` | conforme o campo |

- O `label` é o que a pessoa entende ("Planilha com os CNPJs dos fundos"), nunca o nome da variável.
- Lista em que dá para marcar várias opções: `type: choice` com `multiple: true`.
- Dados baixados da internet não são entradas: viram pontos de atenção.

### Passo 6 — O que o app devolve (`outputs`)
Cada arquivo que o app grava. O `pattern` usa `*` onde o nome muda (`rentabilidade_*.xlsx`); o `label` diz o que a pessoa recebe.

### Passo 7 — Chaves e senhas (`secrets`)
Liste chaves de API, tokens, senhas e strings de conexão só com `name` (o que é), `file`, `line` e `hint` ("mover para o cofre"). Chave escrita direto no código é sempre ponto de atenção. Um placeholder (`SUA_CHAVE_AQUI`) não é segredo. Se o app precisa de chaves, diga quais faltam: a pessoa preenche no Ziv depois, nunca pela conversa.

### Passo 8 — Agenda
Só com evidência (`schtasks`, cron, README dizendo "roda todo dia às 8h"). Vai nos pontos de atenção; o agendamento é feito no Ziv.

### Passo 9 — Nome e resumo
- `name`: curto, sem data e sem "Python".
- `summary`: uma ou duas frases sobre o que o app faz para quem usa. Modelo: "Recebe X e entrega Y, para Z."
- `tags`: 2 a 5 palavras úteis para busca.

### Passo 10 — Área
Uma das áreas do Ziv (a lista vem logo abaixo deste texto). Pode ser sugerida pelo assunto, mas é sempre confirmada.

### Passo 11 — Pontos de atenção (`notes`)
Frases curtas sobre o que alguém precisa saber antes de rodar ou manter o app: caminhos fixos para a pasta pessoal de alguém, site externo, VPN, Excel aberto, chave no código, falta do arquivo de dependências, versões paralelas na mesma pasta (`main_v2.py`), tempo longo de execução. Se o app usa outro app da empresa, o id dele vai em `depends_on`.

### Passo 12 — Conferir se o app roda de verdade
Para cada caminho que o código usa, confira se ele existe a partir da pasta do app. O que não existe vira ponto de atenção. Você confere, mas nunca roda nada.

### Passo 13 — Validar
O Ziv valida a ficha quando você chama `registrar_app` e devolve os problemas em português. Se voltar um problema, corrija e chame de novo com o mesmo envio.

## 3. Conferir e enviar

1. Mostre à pessoa, por app: nome, descrição, tipo, o que pede e o que entrega, área, pontos de atenção e as chaves que faltam (só os nomes).
2. Espere "pode registrar". Sem isso, não prepare envio.
3. Chame `preparar_envio`, troque `<PASTA>` pela pasta do app e rode os comandos do sistema da pessoa.
4. Quando o curl responder `"recebido": true`, chame `registrar_app` com o `envio` e o `manifesto`. Depois apague o .zip temporário.
5. Diga o resultado com o link. Se o app ficou esperando aprovação, diga isso também. Num app com tela, siga a seção 5.

**Vários de uma vez:** um resumo e uma confirmação por app, e um `preparar_envio` por app.

**Possível senha:** se `registrar_app` avisar que o código parece ter senhas ou chaves, pergunte à pessoa. Só se ela confirmar, chame de novo com o mesmo envio e `confirmar_segredos: true`. Se a empresa não permitir, tire as senhas do código e prepare um envio novo.

**Arquivo de início não encontrado:** o .zip precisa ser feito de dentro da pasta do app (`-C "<PASTA>" .`), para os arquivos não ficarem dentro de uma pasta a mais.

## 4. Versão nova

1. Leia o app com `ver_app`. Ele mostra `ficha_editavel` (resumo, entradas, saídas, arquivo de início e dependências) quando a pessoa pode mudar o app.
2. Compare com o que você leu no código e mostre à pessoa, campo por campo, o que muda (antes → depois), junto com um resumo do que mudou no código.
3. Espere "pode mandar". Então `preparar_envio`, rode os comandos e chame `registrar_app` com `envio`, `app_id`, a `nota` (o que mudou) e, se a ficha muda, o `manifesto` com os campos novos.
4. Numa versão nova só mudam resumo, entradas, saídas e (nos scripts) arquivo de início e dependências. Nome, área, responsável, reserva e acesso só mudam pelo site do Ziv: se a pessoa pedir, diga isso.
5. Uma versão nova pode precisar de aprovação de novo antes de rodar.

## 5. App com tela: pôr no ar

Um app `web` não roda no computador da pessoa: ele fica no ar num endereço próprio, para quem tem acesso. Depois de `registrar_app` (app novo ou versão nova), chame `publicar_app_com_tela` com o `app_id`.

1. **Avisos.** Se a checagem do Ziv trouxer avisos (dados gravados em arquivo, pastas do Windows, senha no código, recurso que não vem na receita), mostre cada um à pessoa com o que significa. Só depois que ela disser que está ciente, chame de novo com `ciente` (as chaves dos avisos: `file_data`, `windows_paths`, `secrets`, `container_limits`). Pendências que bloqueiam vêm com o que mudar no código: proponha a mudança e, se a pessoa aceitar, mande uma versão nova.
2. **Comando de início.** Se a checagem pedir o comando de início, sugira um a partir do código, confirme com a pessoa e mande em `comando`.
3. **Chaves que faltam.** Se faltar preencher chaves, diga quais e mande o link: a pessoa preenche no Ziv (parte Compartilhar › Configurações), nunca pela conversa. Quando ela avisar, chame de novo.
4. **Esperar.** Enquanto estiver publicando, chame de novo com `esperar` 30. No fim, diga o endereço. Se a versão nova não abrir, a anterior continua no ar: diga o erro.
5. **Aprovação.** Se a empresa exigir aprovação, o app só vai ao ar depois que um administrador aprovar: diga isso à pessoa.
6. O endereço vem sempre do Ziv; nunca monte um endereço sozinho.

## 6. Formato do manifesto

Mande o manifesto como objeto JSON em `manifesto`:

```json
{
  "name": "Rentabilidade dos fundos",
  "summary": "Recebe a lista de CNPJs e entrega a rentabilidade mensal de cada fundo.",
  "kind": "script",
  "entrypoint": "main.py",
  "runtime": {"language": "python", "dependencies_file": "requirements.txt", "dependencies": ["pandas", "openpyxl"]},
  "inputs": [
    {"id": "cnpjs", "label": "Planilha com os CNPJs dos fundos", "type": "file", "source": "argument", "required": true, "accept": [".xlsx"]}
  ],
  "outputs": [{"label": "Planilha com a rentabilidade", "pattern": "output/rentabilidade_*.xlsx"}],
  "secrets": [{"name": "Chave da API da CVM", "file": "config.py", "line": 12, "hint": "Mover para o cofre"}],
  "area": "Investimentos",
  "tags": ["cvm", "fundos", "excel"],
  "depends_on": [],
  "notes": ["O caminho da planilha de saída aponta para a pasta pessoal de uma pessoa."]
}
```

O Ziv recusa sem: nome; tipo válido; arquivo de início; entradas com id, nome, tipo e origem válidos e sem id repetido; `multiple` só em `choice`; `accept` só em entrada de arquivo, no formato `.txt` (até 20 tipos); o app não depende dele mesmo.

## 7. Limites de leitura

- No máximo 40 arquivos e uns 400 KB de texto lidos por app. Acima disso, priorize o principal, os atalhos e o que o principal importa.
- Arquivo acima de 200 KB: só o começo e as partes que interessam.
- Planilhas, PDFs e bases de dados não são abertos: só o nome e o tamanho importam.
