# Receita: como fazer um app para o Ziv

Um programa para a empresa é feito para o Ziv desde o começo. O Ziv já dá pronto a tela de entradas, a pasta de saída, o cofre de senhas, a agenda, o endereço 24h e a página para ler resultados. Não reconstrua nada disso dentro do programa.

## 1. Qual dos três

| A pessoa quer | Faça | Como a pessoa usa |
|---|---|---|
| Rodar uma tarefa e receber arquivos (planilha, PDF, relatório) | **app** (script, `kind: script`) | Aperta Rodar no Ziv, preenche a tela de entradas, recebe os arquivos na pasta de saída |
| Uma tela para usar no navegador, sempre ligada, para o time | **app com tela** (`kind: web`) | Abre o endereço do app; ele fica no ar 24h no servidor |
| Uma página só para ler (relatório, painel, explicação) | **artifact** (`publicar_artifact`) | Abre no Ziv, como uma página |

Na dúvida entre app e app com tela: se a pessoa só precisa do resultado, é app. App com tela é para quem precisa clicar, filtrar ou consultar a qualquer hora.

## 2. Certo e errado

| Assunto | Errado (não faça) | Certo (jeito Ziv) |
|---|---|---|
| Entradas | `input()` no terminal, janela de escolher arquivo, caminho escrito no código | Declare em `inputs` e leia por argumento: `--<id>=valor` (com `argparse`). O Ziv mostra a tela e passa o valor; um arquivo chega como caminho |
| Resultado | Gravar em `C:\Users\...`, na Área de Trabalho ou numa pasta fixa | Gravar com caminho relativo, na pasta do app (ex.: `saida/relatorio.xlsx`). Todo arquivo novo ou mudado vai sozinho para a pasta de saída da execução |
| Mensagens | Abrir janela ou notificação | `print()`: o que o programa escreve vira um arquivo de texto na saída |
| Senhas e chaves | Escritas no código ou num `config.py` | `os.environ["NOME"]` (Node: `process.env.NOME`), o nome em `keys` e o valor no cofre (`enviar_chaves`) |
| Tela | `webbrowser.open`, servidor local em `localhost`, tkinter | App com tela: escuta em `0.0.0.0` na porta da variável `PORT` e é publicado com `publicar_app_com_tela` |
| Relatório para ler | Gerar HTML e abrir no navegador | `publicar_artifact` |
| Repetir todo dia | Loop com `sleep`, Agendador do Windows | Um app que roda uma vez e termina; a pessoa agenda na ficha do app no Ziv |
| Caminhos | `Path(__file__).parents[3]`, `C:\`, OneDrive | Tudo relativo à pasta do app; nada fora dela |
| Configuração de demonstração | Ler de variável de ambiente que ninguém preenche | Constante no código (ou entrada declarada) |

## 3. Onde roda

- **App (script):** no computador de quem roda, pelo Agente do Ziv, numa cópia da pasta do app. Tempo máximo: 15 minutos. As chaves do cofre chegam no ambiente só dessa execução.
- **App com tela:** no servidor do Ziv, Python 3.12 (ou Node), código em `/app`, dependências do `requirements.txt` (ou `package.json`). Comando típico: `streamlit run app.py`, `uvicorn pacote.main:app` ou `gunicorn` (Flask). A porta vem em `PORT`. Não há OneDrive, `C:\` nem a pasta pessoal de ninguém. Playwright em Python vem pronto: com `playwright` no `requirements.txt` (ou importado), o Ziv monta o app com o Chromium, as bibliotecas dele e uma tela virtual (Xvfb; o app liga com `Xvfb :99` e `DISPLAY=:99` quando o site bloqueia o navegador invisível) e dá ao app pelo menos 512 MB. Não instale o navegador no código do app. Outros navegadores (Selenium, Puppeteer, Playwright em Node) não vêm prontos: avise a pessoa antes de usar.

## 4. Testar antes de mandar

Antes de mandar, rode o programa numa cópia limpa da pasta, como o Ziv vê: uma pasta curta e nova (fora do OneDrive), sem o `.env` da pessoa, com as entradas de exemplo. Só mande se funcionar. Isso pega caminho fixo, arquivo que faltou e dependência esquecida.

**Todo app (script) tem a seção `teste` na ficha.** O Ziv roda a versão nova uma vez, de verdade, no PC de teste, e só põe no ar se passar. Sem `teste`, `enviar_versao` recusa.

```json
"teste": {
  "acessa": ["BTG (login com as chaves BTG_USUARIO, BTG_SENHA)"],
  "pula": "mover os relatórios para a pasta oficial do OneDrive",
  "entradas": {"data": "hoje"},
  "espera": [{"arquivos": "*.pdf", "no_minimo": 1}, {"log_contem": "Concluído"}],
  "tempo_maximo_min": 10
}
```

- `acessa`: o que o teste toca, em português, para o dono do app confirmar (sites, sistemas, chaves). Diga a verdade: é isso que ele aprova.
- `pula`: o único passo irreversível que o modo teste não faz (mandar e-mail, mover para a pasta oficial, enviar uma ordem). `"nada"` se não há.
- `entradas`: o valor de cada entrada obrigatória na rodada de teste (pelo `id` da entrada).
- `espera`: o que o Ziv confere para dizer "passou". Precisa de pelo menos um `arquivos` (com `no_minimo`, 1 se faltar); `log_contem` sozinho não basta. O código de saída 0 também é exigido.
- `tempo_maximo_min`: até 15.

**Modo teste no código.** Na rodada de teste o Ziv põe `ZIV_TESTE=1` e `ZIV_PASTA_TESTE=<pasta>` no ambiente, e roda o app numa pasta só do teste (os arquivos com caminho relativo já caem nela). O app faz tudo de verdade (login, baixar, montar) e, com `ZIV_TESTE`, pula só o passo de `pula`:

```python
import os
if os.environ.get("ZIV_TESTE") == "1":
    print("Modo teste: não movi para a pasta oficial.")
else:
    mover_para_pasta_oficial(arquivos)
```

Na primeira vez, o dono do app confirma o plano de teste no site (o link vem na resposta de `enviar_versao`). Depois disso, os próximos testes rodam sozinhos, até o plano mudar.

## 5. Depois de mandar

- App (script): acompanhe com `ver_versao` até o fim. Se o teste falhou, leia as conferências e o log, corrija e mande outra versão com `enviar_versao`. Os arquivos do teste saem com `baixar_arquivo_teste`.
- App com tela: combine com a pessoa um texto que a página sempre mostra e mande em `conferir_texto` ao publicar; o Ziv só troca de versão se a nova mostrar esse texto (não serve para Streamlit).
- Depois de publicar ou rodar, confira o resultado sozinho (`ver_execucao`, `ver_app`); não espere a pessoa perguntar.
- Se algo falhou, leia o log (`ver_log_app` ou `ver_execucao`) e corrija a causa antes de mandar outra versão. Nunca mande a mesma versão de novo por tentativa.
- Para ver tudo o que deu errado nos apps da pessoa, use `problemas`.

## 6. Como falar com a pessoa

Use as palavras do Ziv: **app**, **app com tela**, **artifact**, **execução**, **cofre**, **agenda**, **pasta de saída**, **ficha do app**, **publicar**. Não use com a pessoa: manifesto (diga ficha), entrypoint (arquivo de início), deploy (publicar), container, MCP, token, endpoint. Frases curtas e no fim o link do Ziv.
