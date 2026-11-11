# 037 - MCP: o Claude fazer e falar do jeito Ziv (A + B + C + D)

**Status:** Accepted

## Context
O Claude só recebia três linhas sobre o Ziv. O manual completo chegava apenas na hora de registrar um programa pronto. Por isso, ao escrever um programa para o Henry, o Claude abriu o navegador e gravou em caminho fixo, até ouvir "integrado ao Ziv". A linguagem com a pessoa também variava, com termos técnicos (manifesto, deploy).

## Decision
- **A. Apresentação nova** (instruções do servidor, que o Claude sempre lê): o que o Ziv dá pronto (app, app com tela, artifact, cofre, agenda) e cinco regras:
  1. programa para a empresa é feito para o Ziv desde o começo, lendo a receita antes;
  2. registrar segue `formato_do_app`;
  3. chamar `problemas` antes de mexer num app e conferir o resultado depois;
  4. ler o log antes de mandar outra versão;
  5. falar nas palavras do Ziv e repassar a `mensagem_para_pessoa`.
- **B. Ferramenta `receita_app_ziv`**, com:
  - o guia de escolha entre app, app com tela e artifact;
  - uma tabela certo × errado (entradas por argumento declarado, resultado em caminho relativo, `print` em vez de janela, senha no cofre, tela em `0.0.0.0:$PORT`, agenda em vez de loop, nada de `parents[N]` ou `C:\`);
  - onde roda (PC pelo Agente com limite de 15 min; servidor com Python 3.12, `/app`, `PORT`, sem navegador pronto);
  - o teste numa cópia limpa antes de mandar.
- **C. Passo "adaptar" no protocolo** (2b): detectar esses padrões, mostrar à pessoa, mudar só com o OK, testar numa cópia. A regra "só leitura" virou "nada muda sem o OK; rodar só na cópia". Também entrou a regra de, após uma falha ao publicar, ler o log e corrigir antes de mandar de novo.
- **D. Jeito Ziv de falar:**
  - `registrar_app`, `publicar_app_com_tela`, `ver_execucao` (ao terminar) e `publicar_artifact` devolvem `mensagem_para_pessoa`, uma frase pronta ("Pronto: o app X está no Ziv. Ele pede … Entrega … Abrir: link").
  - Um teste falha se essa frase tiver palavra técnica (manifesto, entrypoint, MCP, token, deploy, container, endpoint, JSON, API).
  - A ficha padrão (nome com verbo + objeto, resumo "para quem e para quê") entrou no protocolo.

## Rationale
O Claude age pelo que lê. A apresentação é curta, para ser lida sempre, e aponta para a receita, que só é lida quando ele vai escrever código. A frase pronta deixa o padrão de linguagem no Ziv, testado, em vez de depender de cada conversa.

## Trade-offs
**Pros:**
- O Claude faz certo desde o começo, sem a pessoa saber pedir.
- A linguagem com a pessoa fica igual em toda conversa.

**Cons:**
- O teste de palavras vale só para a `mensagem_para_pessoa`. As descrições das ferramentas são para o Claude e usam nomes de campo (manifesto).
- O passo de testar numa cópia depende do Claude seguir a receita. O ensaio no servidor (F4c) vem depois.

## Alternatives Considered
- Pôr a receita inteira nas instruções do servidor: descartado. Ficaria longa e seria lida em toda conversa, mesmo quando não há código.
- Traduzir as respostas no próprio Claude, sem frase pronta: descartado. Não dá para testar.
