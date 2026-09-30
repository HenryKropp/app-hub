# Relatório de leitura — Henry Files Prep

**Rodada:** 1 · **Data:** 2026-09-30 · **Protocolo:** v0.1
**Pasta:** `C:\Users\HenryKroppAlisInvest\OneDrive - Alis\ALIS\5. Operacional\8. Programação\2026 06 23 - Henry Files Prep`
**Nada foi escrito na pasta do app.** Arquivos lidos: 4 dentro da pasta e 1 fora (o motor `cvm_downloader.py`, lido em partes).

## O que eu entendi

É um programa com janela própria, o **"Pipeline IA — Equity Research"**, aberto pelo `Ferramenta CVM.exe`. Funciona em duas fases:
1. Você digita o nome ou o CNPJ de uma empresa aberta, quantos anos de histórico quer e quais documentos, e ele baixa tudo da CVM para uma pasta da empresa.
2. Você marca numa lista quais PDFs entram, e ele junta tudo num único arquivo de texto (`..._DataPack_Consolidado.md`) para colar numa IA.

Tem também um botão para cortar um FRE inteiro só nas seções que interessam. O motor de download não está nesta pasta: vem do app **CVM Downloader**.

## Campo a campo

| Campo | Valor | Confiança | Evidência |
|---|---|---|---|
| Tipo | App com janela própria (`desktop`) | Confirmado | `Files Prep.py:118` (`class CVMApp(tk.Tk)`), `:682` (`mainloop`) |
| Arquivo de início | `Ferramenta CVM.exe` | Deduzido | Existe na raiz, gerado por `Recompilar Ferramenta CVM.bat:36-46` e `:55`. O `.exe` (13/08 17:54) é mais novo que o código (13/08 17:33), então está atualizado. O código sozinho não roda (ver pontos de atenção) |
| Nome | Pipeline IA — Equity Research | Deduzido | Título da janela, `Files Prep.py:121` e `:186`. Outros nomes em uso: "Henry Files Prep" (pasta) e "Ferramenta CVM" (.exe) |
| Resumo | Baixa da CVM… junta num Data Pack | Confirmado | `:188` ("Extração CVM · Curadoria humana · Data Pack para LLM"), fases em `:321-385` e `:604-662` |
| Entrada: empresa | Nome ou CNPJ, texto | Confirmado | `:199-202`, validação em `:322-328` |
| Entrada: anos | Número, padrão 3, de 1 a 20 | Confirmado | `:206-209` |
| Entrada: documentos | FRE, Apresentações, Fatos Relevantes (marcados); ITR, DFP (desmarcados) | Confirmado | `:219-234` |
| Entrada: pasta raiz | Pasta, janela de escolha; padrão é a pasta do app | Confirmado | `:211-216`, `:293-295`, `_dir_app` em `:26-32` |
| Entrada: pasta de trabalho | Pasta, janela de escolha; opcional | Confirmado | `:400-409` |
| Entrada: FRE único | Arquivo PDF, janela de escolha; opcional | Confirmado | `:440-442` |
| Saída: PDFs da CVM | `<pasta>/<Empresa>/<n. Tipo>/...pdf` | Confirmado | `cvm_downloader.py:64-75` (nomes das pastas), `:959` (pasta da empresa) |
| Saída: Data Pack | `<pasta>/MD_Output/<pasta>_DataPack_Consolidado.md` | Confirmado | `Files Prep.py:606-609`, `:657-658`. Exemplo real: `AZZAS 2154 S.A/MD_Output/` |
| Saída: FRE cortado | `<nome>_cortado_secoes.pdf`, ao lado do original | Confirmado | `:485` |
| Bibliotecas | pdfplumber, pypdf, requests, tqdm, curl_cffi | Confirmado | `Código-fonte/requirements.txt` (o `pyinstaller` só serve para gerar o .exe, por isso ficou de fora) |
| Chaves e senhas | Nenhuma | Confirmado | Nada encontrado no `Files Prep.py` nem no `.bat`. O motor acessa só dados públicos da CVM, `cvm_downloader.py:55-59` |
| Agenda | Nenhuma | Confirmado | Sem `schtasks` ou agendador |
| Área | Research | Deduzido | Assunto (equity research, FRE, fatos relevantes) |
| Responsável | Henry Kropp | Sugestão | Quem pediu o registro. **Precisa confirmar** |
| Reserva | — | Desconhecido | **Precisa definir** |

## Perguntas para você

1. **Nome no catálogo:** "Pipeline IA — Equity Research" (o nome da janela), "Ferramenta CVM" (o nome do .exe) ou outro?
2. **Responsável e reserva:** você é o responsável? Quem é o reserva?
3. **Área:** Research está certo?
4. **Relação com o CVM Downloader:** este app substitui o CVM Downloader para quem usa, ou os dois continuam separados no catálogo?

## Diagnóstico automático do Hub × leitura pelo protocolo

| Campo | Regras do Hub | Protocolo | Ganho |
|---|---|---|---|
| Nome | "Henry Files Prep" | "Pipeline IA — Equity Research" | Nome de quem usa, não da pasta |
| Resumo | "CVM Downloader & Markdown Converter — Interface Gráfica Unificada" (a primeira linha do código) | Frase em português do que o app entrega | Grande |
| Tipo | desktop, com pergunta sobre o .exe | desktop | Igual |
| Arquivo de início | `Código-fonte/Files Prep.py` com `pythonw` | `Ferramenta CVM.exe` | **Crítico: pelas regras, o Hub tentaria rodar o código, que dá erro** |
| Entradas | Nenhuma | 6 | Grande: as regras não reconhecem campos de janela (Entry, Spinbox, Checkbutton) |
| Saídas | 1 (FRE cortado) | 3 | Faltavam as duas principais |
| Arquivo de dependências | `requirements.txt` (caminho errado) | `Código-fonte/requirements.txt` | Correção |
| Pontos de atenção | Nenhum | 8 | Dependência de outro app, código quebrado, caminho de outro usuário, 2,3 GB no OneDrive |

## O que esta rodada ensinou para o protocolo (proposta da v0.2)

1. **Apps com janela pedem as coisas dentro da janela, não antes de abrir.** As "entradas" de um app desktop descrevem o que a janela vai pedir, e não um formulário do Hub. O manifesto precisa saber diferenciar (sugestão: `source: window`).
2. **Escolha múltipla** ("pode marcar vários") não existe no formato. Sugestão: `type: choice` com `multiple: true`.
3. **Dependência de outro app** não tem campo próprio, e hoje só aparece nos pontos de atenção. Sugestão: `depends_on: [cvm-downloader]`, o que também ajuda na continuidade (1.9).
4. **Código + .exe:** quando o .exe é mais novo que o código, o arquivo de início é o .exe. O código e o jeito de recompilar deveriam ter campo próprio (sugestão: `build: { source, command }`).
5. **Leitura fora da pasta:** para entender o app, precisei ler um arquivo de outro app. O protocolo e o futuro MCP precisam permitir leitura, só leitura, de arquivos que o app importa de pastas vizinhas.
6. **Pasta com muitos dados:** 2,3 GB e 600 arquivos, quase tudo PDF de saída. O mapa da pasta precisa separar "programa" de "dados gerados" logo no passo 1, para não gastar leitura com isso.
7. **Checar se o código roda de verdade:** o caminho quebrado só apareceu porque conferi para onde apontavam os caminhos relativos. Vale virar um passo explícito: "conferir se os caminhos que o código usa existem".
8. **Validar a ficha no próprio Hub antes de mostrar.** O meu primeiro rascunho tinha um erro de formato: um texto com "ex.: PETROBRAS" quebrava o YAML. Com o MCP, a ferramenta de registro valida sozinha. No teste, o passo final passa a ser "validar com o leitor do Hub".
