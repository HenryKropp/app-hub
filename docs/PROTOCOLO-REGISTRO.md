# Protocolo de registro de apps pelo Claude

**Versão:** 0.2 (rascunho para teste local) · **Data:** 2026-09-30 · Mudanças da v0.1 para a v0.2: marcadas com **(v0.2)**, vindas da rodada 1 (seção 8)

Roteiro que o Claude segue para ler a pasta de um app e montar o manifesto dele (`app-hub.yaml`), sem ninguém preencher nada à mão. Primeiro é testado localmente, com o Claude lendo as pastas direto. Depois vira a base do MCP (Fase 2b): as ferramentas do MCP entregam ao Claude exatamente o que este protocolo pede, e as instruções do MCP são este texto.

---

## 1. Regras que nunca mudam

1. **Só leitura.** O Claude não altera, move, apaga nem roda nenhum arquivo do app. O único lugar em que ele escreve é o Hub (no teste, uma pasta de rascunho).
2. **Nunca o valor de um segredo.** Chaves, senhas e tokens são registrados só como *nome + arquivo + linha*. O valor nunca aparece no manifesto, no relatório nem na conversa, nem em parte (nada de "começa com sk-…").
3. **Nada inventado.** Todo campo preenchido tem uma evidência no código ou na documentação (arquivo e linha). O que não tem evidência vira pergunta, não palpite.
4. **Pessoas não se deduzem do código.** Responsável, reserva e área vêm de quem está registrando. O nome de quem aparece no caminho de uma pasta (`C:\Users\fulano`) não faz dessa pessoa a responsável.
5. **Um humano confirma.** O Claude mostra o que entendeu e só registra depois do "pode registrar".
6. **Linguagem de gente.** Resumo, entradas, saídas e pontos de atenção são escritos em português simples, para quem nunca viu o código.

---

## 2. O que o Claude recebe antes de começar

| Item | Para quê | No teste local | No MCP |
|---|---|---|---|
| Caminho da pasta do app | O que vai ser lido | Informado na conversa | Ferramenta "varrer pasta" |
| Diagnóstico por regras do Hub | Ponto de partida (tipo, arquivo de início, segredos) | `POST /api/scan` ou o próprio scanner | Mesma ferramenta |
| Catálogo atual | Evitar registrar duas vezes | `GET /api/apps` | Ferramenta "listar catálogo" |
| Lista de pessoas | Sugerir responsável e reserva | `GET /api/people` | Ferramenta "listar pessoas" |
| Formato do manifesto | Saber os campos e valores aceitos | Seção 6 deste documento | Ferramenta "formato do manifesto" |

O diagnóstico por regras é **ponto de partida, não verdade**. O Claude confirma ou corrige cada campo lendo o código.

---

## 3. Passo a passo

### Passo 0 — Já existe?
- Se a pasta já tem `app-hub.yaml`, ele é lido primeiro. O trabalho vira **atualização**: manter o que uma pessoa já confirmou e só propor mudanças onde o código mudou.
- Se a pasta já está no catálogo, ou o nome é muito parecido com um app do catálogo, isso é avisado antes de continuar ("parece o mesmo que *X*: é uma versão nova ou outro app?").
- Se a pasta parece material antigo (nome com "old", "antigo", "backup", "arquivo", "v1" com uma "v2" ao lado), o Claude sugere pular e pergunta.

### Passo 1 — Mapa da pasta
- Lista os arquivos até 4 níveis de profundidade, ignorando `.git`, `.venv`, `venv`, `node_modules`, `__pycache__`, `build`, `dist` e pastas ocultas.
- Separa em: **código** (`.py`, `.js`, `.ts`, `.go`…), **atalhos de início** (`.bat`, `.cmd`, `.vbs`, `.ps1`, `.lnk`), **configuração** (`config.*`, `.env`, `settings.*`, `*.ini`, `*.toml`), **dependências** (`requirements.txt`, `pyproject.toml`, `package.json`, `go.mod`), **documentação** (`README*`, `*.md`, `*.docx`, `*.pdf`) e **dados** (`.xlsx`, `.csv`, `.json` grandes, pastas `input/` e `output/`).
- Pastas de dados de exemplo mostram o formato esperado de entrada e saída: vale olhar os nomes, sem abrir o conteúdo de planilhas de cliente.
- **(v0.2) Separar programa de dados gerados antes de ler qualquer coisa.** Pastas com muitos arquivos do mesmo tipo (dezenas de PDFs, uma pasta por empresa, `MD_Output/`, `output/`) são quase sempre saídas acumuladas: registrar só a quantidade, o tamanho total e um exemplo de nome. Isso não conta no limite de leitura, e vira ponto de atenção quando as saídas ficam misturadas com o programa.

### Passo 2 — Leitura, nesta ordem
1. **Documentação:** README e notas. Diz a intenção de quem fez, mas pode estar desatualizada: o código manda.
2. **Atalhos de início** (`.bat`, `.vbs`, `.ps1`): mostram como a pessoa realmente roda o app (qual arquivo, com `python` ou `pythonw`, com `streamlit run`, se agenda com `schtasks`).
3. **Arquivo principal:** o que o atalho chama, ou o que tem `if __name__ == "__main__"`, `app.run`, `mainloop()`, `streamlit`. É lido inteiro.
4. **Módulos que o principal usa**, na ordem em que aparecem, até o limite de leitura (seção 7).
5. **Configuração:** só para saber *quais* chaves e caminhos existem. Valores de segredo nunca são copiados.
6. **Dependências:** o arquivo de dependências, ou os `import` de bibliotecas de fora quando esse arquivo não existe.
7. **(v0.2) Arquivos de fora da pasta que o app usa.** Se o código importa ou chama um arquivo de uma pasta vizinha (por exemplo, `sys.path.insert` para a pasta de outro app), esse arquivo pode ser lido, **só leitura**, apenas nas partes que explicam o que o app faz. Isso entra no relatório como "arquivo lido fora da pasta" e vira dependência (passo 11).

### Passo 3 — Que tipo de app é
Decidido nesta ordem (o primeiro que se encaixar):

| Sinal no código | Tipo (`kind`) |
|---|---|
| Vários serviços (backend + frontend, Docker, banco próprio) | `system` |
| Só um `.exe` pronto, sem código-fonte | `executable` |
| Só documentos ou links, nada roda | `reference` |
| Roda sozinho em horário fixo (`schtasks`, agendador, loop com `sleep` longo) | `scheduled` |
| Abre no navegador (Streamlit, Flask, FastAPI, Dash, servidor Node) | `web` |
| Abre janela própria que fica aberta (tkinter com `mainloop`, PyQt) | `desktop` |
| Roda, faz o trabalho e termina (mesmo que abra uma janela só para escolher arquivo) | `script` |

Um script que usa tkinter só para `askopenfilename` e depois fecha (`withdraw`) é **script**, não desktop.

### Passo 4 — Como inicia
- **`entrypoint`:** o arquivo que inicia, relativo à pasta do app.
- **`run.command`:** o comando completo, como o atalho faz (`python main.py`, `streamlit run app.py`). Se há um `.bat`, o comando é o que o `.bat` executa.
- **`run.url`:** o endereço, se for web e a porta estiver no código.
- Se houver dois candidatos a arquivo principal, isso vira pergunta.
- **(v0.2) Código + `.exe` na mesma pasta:** compare as datas. Se o `.exe` é mais novo que o código e há um atalho que o gera (`pyinstaller` num `.bat`, por exemplo), o arquivo de início é o `.exe`. O código e o jeito de recompilar vão em `build` (`source` e `command`). Se o código é mais novo que o `.exe`, isso vira pergunta ("o .exe está desatualizado?").

### Passo 5 — O que o app recebe (`inputs`)
Cada informação que vem de fora é uma entrada:

| Como aparece no código | `source` | `type` |
|---|---|---|
| Caminho escrito no código (`pd.read_excel("C:/.../cnpjs.xlsx")`) | `fixed_file` (+ `path`) | `file` ou `folder` |
| Janela de escolher arquivo ou pasta (`askopenfilename`, `askdirectory`) | `dialog` | `file` ou `folder` |
| Pergunta no terminal (`input("Data:")`) ou campo na tela | `prompt` | `text`, `number`, `date` ou `choice` |
| Parâmetro na linha de comando (`argparse`, `sys.argv`) | `argument` | conforme o uso |
| **(v0.2)** Campo ou janela de escolha dentro da janela de um app `desktop` (`Entry`, `Spinbox`, `Checkbutton`, botão que abre `askdirectory`) | `window` | conforme o campo |

**(v0.2) Janela × formulário.** Em apps `desktop`, tudo o que a própria janela pede depois de aberta tem `source: window`: aparece na ficha, para quem vai usar saber o que ter em mãos, mas o Hub **não** pergunta nada antes de abrir. As outras origens (`fixed_file`, `dialog`, `prompt`, `argument`) são o que o Hub vai precisar fornecer ou perguntar ao rodar.

**(v0.2) Várias escolhas.** Uma lista em que dá para marcar mais de uma opção (vários `Checkbutton`, `listbox` com seleção múltipla) é `type: choice` com `multiple: true`. As opções que já vêm marcadas vão no `label` ou nos pontos de atenção.

- O `label` é o que a pessoa entende: "Planilha com os CNPJs dos fundos", e não "df_input".
- Se o código espera colunas ou formato específico (por exemplo, "coluna CNPJ na primeira aba"), isso vai para os pontos de atenção.
- Dados baixados da internet **não** são entradas: vão para os pontos de atenção ("depende do site da CVM estar no ar").

### Passo 6 — O que o app devolve (`outputs`)
- Cada arquivo que o app grava (`to_excel`, `savefig`, `open(..., "w")`, PDF, Markdown).
- O `pattern` usa `*` onde o nome muda a cada execução: `rentabilidade_*.xlsx`.
- O `label` diz o que a pessoa recebe: "Planilha com a rentabilidade de cada fundo".
- Um arquivo que o app lê e também grava é entrada, com nota nos pontos de atenção.

### Passo 7 — Chaves e senhas (`secrets`)
- Procura chaves de API, tokens, senhas, strings de conexão com senha e arquivos `.env` e `config`.
- Registra: `name` (o que é: "Chave da API da OpenAI"), `file`, `line` e `hint` (onde deveria ficar: "mover para o cofre").
- Chave escrita direto no código é **sempre** ponto de atenção, porque viola o padrão da empresa ("nada de chave no código").
- Um campo que só *parece* chave (texto de status, exemplo, placeholder `SUA_CHAVE_AQUI`) não é segredo. O Claude confere o uso antes de listar.

### Passo 8 — Agenda (`schedule`)
Só quando há evidência (`schtasks`, cron, agendador, documentação dizendo "roda todo dia às 8h"). Escrita em português: "Todo dia útil às 8h00".

### Passo 9 — Nome e resumo
- **`name`:** curto, sem data e sem "Python" ("Rentabilidade dos fundos", e não "2026 05 25 Python Rentabilidade fundos").
- **`summary`:** uma ou duas frases sobre **o que o app faz para quem usa**, não como faz. Modelo: *"Recebe X e entrega Y, para Z."* Exemplo: "Recebe a lista de CNPJs dos fundos e entrega uma planilha com a rentabilidade de cada um no mês, a partir dos dados da CVM."
- **`language`:** a linguagem principal (`python`, `javascript`, `go`…).
- **`tags`:** 2 a 5 palavras úteis para busca (fonte de dados, tipo de resultado): `cvm`, `fundos`, `excel`.

### Passo 10 — Pessoas e área
- **`owner`** (responsável): obrigatório. Sugestão: quem está registrando. Sempre confirmado.
- **`backup`** (reserva): outra pessoa da lista. Se ninguém for indicado, fica vazio e vira ponto de atenção ("sem reserva").
- **`area`:** uma de Investimentos, Operações, Relações com Investidores, Research, Compliance, Administrativo. Pode ser sugerida pelo assunto do app, mas sempre é confirmada.

### Passo 11 — Pontos de atenção (`notes`)
**(v0.2) Dependência de outro app (`depends_on`).** Se o app usa código, arquivos ou resultados de outro app da empresa, o `id` desse app vai em `depends_on` (e continua como ponto de atenção, explicando o impacto). Se o outro app ainda não está no catálogo, vale perguntar se ele deve ser registrado antes. Isso ajuda a continuidade: quem mexe num app vê quais outros podem quebrar.

O que alguém precisa saber antes de rodar ou manter o app. Frases curtas, uma por ponto. Procurar principalmente:
- Caminhos fixos para a pasta pessoal de alguém (`C:\Users\fulano\...`): o app quebra no computador de outra pessoa.
- Dependência de site externo, login em sistema, VPN ou Excel aberto.
- Chave ou senha no código.
- Falta do arquivo de dependências.
- Código duplicado ou versões paralelas na mesma pasta (`main_v2.py`, `main_final.py`).
- O que o README diz e o código não faz (ou o contrário).
- Tempo longo de execução, limites (por exemplo, "processa no máximo 50 PDFs").

### Passo 12 — (v0.2) Conferir se o app roda de verdade
Para cada caminho que o código usa (arquivos lidos, pastas de import, o Python chamado pelo `.bat`, o alvo de um atalho), conferir se ele existe a partir da pasta do app. O que não existe vira ponto de atenção com a consequência ("abrir pelo código dá erro, porque o motor fica numa pasta que não existe"). Isso pode mudar o arquivo de início (passo 4). O Claude confere que o caminho existe, mas **não roda** nada (regra 1).

### Passo 13 — (v0.2) Validar antes de mostrar
O rascunho passa pelo leitor e pelas regras do próprio Hub antes de ser mostrado. No teste local: `cd backend && mise x -- go run ./cmd/validate-manifest <rascunho>/app-hub.yaml`, que também mostra a ficha como as pessoas vão vê-la. No MCP, a ferramenta de registro valida sozinha e devolve os problemas em português. Rascunho com erro não é mostrado: corrige primeiro.

---

## 4. Confiança e perguntas

Cada campo recebe um nível:

| Nível | Quando | O que acontece |
|---|---|---|
| **Confirmado** | Evidência direta no código ou no atalho | Vai para o manifesto |
| **Deduzido** | Indício, mas não certeza (ex.: área pelo assunto) | Vai para o manifesto e aparece como "confira" |
| **Desconhecido** | Nenhuma evidência | Não é preenchido; vira pergunta |

**O Claude para e pergunta quando:**
- não há um arquivo principal claro;
- a pasta parece conter mais de um app, ou o mesmo app em versões diferentes;
- o app parece duplicado com outro do catálogo;
- responsável, reserva ou área não foram informados;
- uma entrada existe, mas não dá para saber o que a pessoa deve fornecer.

Perguntas são poucas e objetivas, com opções quando possível ("O arquivo principal é `main.py` ou `app_v2.py`?").

---

## 5. O que o Claude entrega

Para cada app, duas coisas:

**a) O manifesto** (`app-hub.yaml`), no formato da seção 6. É o que o Hub publica.

**b) O relatório de leitura**, que não é publicado e serve para a pessoa conferir. Contém:
- o resumo do que foi entendido, em 3 a 5 linhas;
- uma tabela *campo → valor → nível de confiança → evidência (arquivo:linha)*;
- as perguntas pendentes;
- o que o diagnóstico por regras tinha errado ou deixado de fora (só durante o teste, para medir o ganho).

Na conversa, o Claude mostra o resumo, as perguntas e os pontos de atenção. O detalhe fica no relatório.

---

## 6. Formato do manifesto

```yaml
manifest_version: 1
id: rentabilidade-fundos            # gerado pelo Hub a partir do nome
name: Rentabilidade dos fundos
summary: Recebe a lista de CNPJs e entrega a rentabilidade mensal de cada fundo.
kind: script                        # script | web | desktop | scheduled | executable | reference | system
language: python
entrypoint: main.py                 # obrigatório para tipos que rodam
run:
  command: python main.py
  url: ""                           # só para web
runtime:
  language: python
  dependencies_file: requirements.txt
  dependencies: [pandas, openpyxl, requests]
inputs:
  - id: cnpjs
    label: Planilha com os CNPJs dos fundos
    type: file                      # file | folder | text | number | date | choice
    source: dialog                  # fixed_file | dialog | prompt | argument | window (v0.2)
    path: ""                        # só para fixed_file
    options: []                     # só para choice
    multiple: false                 # (v0.2) só para choice: pode marcar várias
    required: true
outputs:
  - label: Planilha com a rentabilidade
    pattern: output/rentabilidade_*.xlsx
secrets:
  - name: Chave da API da CVM
    file: config.py
    line: 12
    hint: Mover para o cofre
schedule: ""
area: Investimentos
owner: { name: Henry Kropp, email: henry.kropp@alisinvestimentos.com }
backup: null
tags: [cvm, fundos, excel]
depends_on: []                      # (v0.2) ids de outros apps do catálogo que este usa
build:                              # (v0.2) só quando o app abre por um .exe gerado de código
  source: src/main.py
  command: build.bat
notes:
  - O caminho da planilha de saída aponta para a pasta pessoal do Henry.
```

Regras de validação do Hub (o registro é recusado sem elas): nome; tipo válido; responsável; reserva diferente do responsável; arquivo de início para os tipos que rodam; entradas com nome, tipo e origem válidos e sem `id` repetido; **(v0.2)** `multiple` só em lista de opções; o app não depende dele mesmo; `build` sempre com `source`.

---

## 7. Limites de leitura

- No máximo **40 arquivos** e **~400 KB de texto** lidos por app. Acima disso, prioriza o principal, os atalhos e o que o principal importa, e anota "leitura parcial" no relatório.
- Arquivo acima de **200 KB**: só o começo e as partes que interessam (busca por `open`, `read_`, `to_`, `input`, chaves).
- Planilhas, PDFs e bases de dados **não** são abertos: só o nome e o tamanho importam.
- Tipo `system` (sistema grande): leitura só de README, estrutura e arquivos de início. O registro é de ficha, não de execução.

---

## 8. Teste local (antes do MCP)

**Como:** o Claude, nesta máquina, segue o protocolo lendo as pastas diretamente. Cada app gera `app-hub.yaml` + `LEITURA.md` numa pasta de rascunho fora das pastas da Alis. **Nada é escrito nas pastas dos apps.**

> Diferença para o MCP: no teste o Claude lê os arquivos crus, então vê os valores das chaves. A regra 2 continua valendo: nenhum valor é repetido em nenhum lugar. No MCP, o Hub mascara os valores antes de o Claude ler.

**Apps do teste (bem diferentes entre si):**

| App | Por que foi escolhido |
|---|---|
| Python Rentabilidade fundos | Script simples de entrada e saída |
| Research Platform | App web, com configuração e chaves |
| transcritor-calls | Provavelmente pede arquivo ao usuário |
| CVM Downloader | Baixa da internet, talvez agendado |

**Como avaliar cada ficha (a pessoa responde):**
1. O resumo explica o app para alguém que nunca o usou?
2. O tipo e o arquivo de início estão certos?
3. As entradas batem com o que você realmente fornece ao rodar?
4. As saídas batem com o que você recebe?
5. Os segredos estão todos lá, sem nenhum falso?
6. Os pontos de atenção são úteis ou é ruído?
7. Alguma coisa foi inventada?
8. As perguntas foram necessárias ou dava para descobrir sozinho?

**Registro das rodadas:**

| Rodada | Data | Apps | O que deu errado | O que mudou no protocolo |
|---|---|---|---|---|
| 1 | 2026-09-30 | Henry Files Prep | Pelas regras, o Hub rodaria o código quebrado em vez do .exe; nenhuma entrada detectada; faltavam 2 de 3 saídas Ficha aprovada pelo Henry. As 8 propostas foram aprovadas e viraram a **v0.2**: origem `window`, `multiple`, `depends_on`, `build`, leitura de pastas vizinhas, separar programa de dados, passo 12 (conferir caminhos), passo 13 (validar) |

O protocolo está pronto para virar MCP quando os 4 apps passarem nos 8 critérios sem nada inventado.
