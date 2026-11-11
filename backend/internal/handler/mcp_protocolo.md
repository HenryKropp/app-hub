# Como registrar um app no Ziv pelo Claude

Você lê a pasta do app no computador da pessoa, monta a ficha (o manifesto), mostra para ela conferir e só registra depois que ela disser "pode registrar". Com a pessoa, diga sempre "ficha", nunca "manifesto". O código vai para o Ziv num .zip, por um endereço de uso único.

## 1. Regras que nunca mudam

1. **Nada muda sem o OK.** Você lê a pasta no disco e não altera, move nem apaga nenhum arquivo do app sem o OK da pessoa (adaptar ao jeito Ziv, Passo 2b, só depois do OK). Para testar, copie a pasta para uma pasta temporária e rode lá, nunca na pasta original. O .zip também vai para a pasta temporária do computador, nunca para a pasta do app.
2. **Nunca o valor de um segredo.** Chaves, senhas e tokens aparecem só como nome + arquivo + linha. O valor nunca vai para a ficha, para a conversa nem para o Ziv, nem em parte (nada de "começa com sk-…"). Não abra o .env: basta saber que ele existe e os nomes das chaves que o código usa. Para levar os valores do .env ao cofre, use `enviar_chaves` (Passo 7): o curl manda o arquivo direto ao Ziv, sem você ler.
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

### Passo 2b — Adaptar ao jeito Ziv
Compare o que leu com a receita (`receita_app_ziv`). Procure, em especial:
- `input()` no terminal, janela de escolher arquivo ou caminho de entrada escrito no código → entrada declarada, lida por argumento;
- resultado gravado em `C:\`, na pasta pessoal, na Área de Trabalho ou no OneDrive → caminho relativo na pasta do app;
- `Path(__file__).parents[N]` com N maior que 1, ou qualquer caminho que sai da pasta do app → caminho dentro da pasta;
- senha ou chave no código → `os.environ["NOME"]` + `keys` (Passo 7);
- `webbrowser.open`, servidor em `localhost` ou `127.0.0.1` → app com tela, ouvindo em `0.0.0.0` na porta `PORT`;
- loop com `sleep` para repetir → rodar uma vez e terminar; a pessoa agenda na ficha;
- configuração de demonstração lida de variável de ambiente → constante no código.

Mostre à pessoa o que achou, em linguagem simples, e proponha a troca. Só mude o código com o OK dela; sem o OK, registre como está e anote cada ponto nos pontos de atenção. Depois de mudar, teste numa cópia limpa da pasta (receita, seção 4) antes de registrar.

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
Liste chaves de API, tokens, senhas e strings de conexão só com `name` (o que é), `file`, `line` e `hint` ("mover para o cofre"). Chave escrita direto no código é sempre ponto de atenção. Um placeholder (`SUA_CHAVE_AQUI`) não é segredo. 

**Cofre do app (`keys`).** Todo app que roda (script, app com tela, app com janela, executável) guarda as senhas de verdade no cofre do app, no Ziv. A mesma senha vale no computador da pessoa e na versão ligada 24h no servidor:
1. No código, troque o valor pela leitura do ambiente: `os.environ["BTG_SENHA"]` (em Node, `process.env.BTG_SENHA`). Mostre a mudança à pessoa antes.
2. Na ficha, liste cada nome em `keys`: `[{"name": "BTG_SENHA", "label": "Senha do BTG", "required": true}]`. Os nomes têm só letras maiúsculas, números e `_`, começando por letra, e não podem começar com `PYTHON`, `APP_HUB_`, `NODE_`, `NPM_`, `PIP_` ou `PATH`.
3. **Se a pasta tem um `.env`, depois do registro chame `enviar_chaves` com o `app_id`** e rode o curl que ele devolve, com o caminho do .env. Não abra o arquivo: o curl manda direto ao Ziv, e a resposta traz só os nomes guardados. Diga à pessoa quais chaves foram guardadas.
4. O que não estava no .env, o dono do app ou um administrador preenche no link do cofre, nunca pela conversa. Até lá, um app com chave obrigatória não abre.

Quando alguém roda o app, o Agente entrega as chaves só àquela execução: ao script, ou ao programa que ele abre (o app com tela recebe as chaves no ambiente, como se viessem do .env).

### Passo 8 — Agenda
Só com evidência (`schtasks`, cron, README dizendo "roda todo dia às 8h"). Vai nos pontos de atenção; o agendamento é feito no Ziv.

### Passo 9 — Nome e resumo (ficha padrão)
- `name`: o que o app faz, verbo + objeto ("Calcula a rentabilidade dos fundos"), curto, sem data e sem "Python".
- `summary`: uma frase, para quem e para quê. Modelo: "Recebe X e entrega Y, para Z."
- Entradas e saídas com `label` que a pessoa entende; os pontos de atenção (Passo 11) dizem o que alguém precisa saber antes de usar.
- `tags`: 2 a 5 palavras úteis para busca.

### Passo 10 — Área
Uma das áreas do Ziv (a lista vem logo abaixo deste texto). Pode ser sugerida pelo assunto, mas é sempre confirmada.

### Passo 11 — Pontos de atenção (`notes`)
Frases curtas sobre o que alguém precisa saber antes de rodar ou manter o app: caminhos fixos para a pasta pessoal de alguém, site externo, VPN, Excel aberto, chave no código, falta do arquivo de dependências, versões paralelas na mesma pasta (`main_v2.py`), tempo longo de execução. Se o app usa outro app da empresa, o id dele vai em `depends_on`.

### Passo 12 — Conferir se o app roda de verdade
Para cada caminho que o código usa, confira se ele existe a partir da pasta do app. O que não existe vira ponto de atenção. Você confere lendo; rodar, só na cópia de teste (Passo 2b).

### Passo 13 — Validar
O Ziv valida a ficha quando você chama `registrar_app` e devolve os problemas em português. Se voltar um problema, corrija e chame de novo com o mesmo envio.

## 3. Conferir e enviar

1. Mostre à pessoa, por app: nome, descrição, tipo, o que pede e o que entrega, área, pontos de atenção e as chaves que faltam (só os nomes).
2. Espere "pode registrar". Sem isso, não prepare envio.
3. Chame `preparar_envio`, troque `<PASTA>` pela pasta do app e rode os comandos do sistema da pessoa.
4. Quando o curl responder `"recebido": true`, chame `registrar_app` com o `envio` e o `manifesto`. Depois apague o .zip temporário.
5. Diga o resultado com o link, usando a `mensagem_para_pessoa` da resposta. Se o app ficou esperando aprovação, diga isso também. Num app com tela, siga a seção 5.

**Vários de uma vez:** um resumo e uma confirmação por app, e um `preparar_envio` por app.

**Possível senha:** se `registrar_app` avisar que o código parece ter senhas ou chaves, pergunte à pessoa. Só se ela confirmar, chame de novo com o mesmo envio e `confirmar_segredos: true`. O aviso diz o arquivo e a linha de cada trecho. Se for senha de verdade, mova para o cofre (`keys`, Passo 7) e prepare um envio novo. Se a empresa não permitir confirmar, esse é o único caminho.

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
2. **Como conferir.** Na primeira publicação, combine com a pessoa um texto que a página do app sempre mostra quando está funcionando (um título, por exemplo) e mande em `conferir_texto` (e a página em `conferir_caminho`, se não for a inicial). O Ziv só troca para uma versão nova se ela mostrar esse texto; senão, a anterior segue no ar. Não serve para Streamlit: a página dele só aparece com JavaScript, e o Ziv lê a página sem rodar JavaScript.
3. **Comando de início.** Se a checagem pedir o comando de início, sugira um a partir do código, confirme com a pessoa e mande em `comando`.
4. **Chaves que faltam.** Se faltar preencher chaves e a pasta tem um `.env`, use `enviar_chaves`. O resto, diga quais e mande o link: a pessoa preenche no Ziv (parte Cofre ou Compartilhar › Configurações), nunca pela conversa. Quando ela avisar, chame de novo.
5. **Esperar.** Enquanto estiver publicando, chame de novo com `esperar` 30. No fim, diga o endereço. Se a versão nova não abrir, a resposta diz se a anterior continua no ar e traz o fim do log.
6. **Se falhar.** Leia o log da resposta (ou `ver_log_app`), ache a causa no código, mostre à pessoa e só então mande uma versão nova. Nunca mande a mesma versão de novo por tentativa.
7. **Aprovação.** Se a empresa exigir aprovação, o app só vai ao ar depois que um administrador aprovar: diga isso à pessoa.
8. O endereço vem sempre do Ziv; nunca monte um endereço sozinho.

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
  "keys": [{"name": "CVM_API_KEY", "label": "Chave da API da CVM", "required": true}],
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
