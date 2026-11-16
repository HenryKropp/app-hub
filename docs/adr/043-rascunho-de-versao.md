# 043 - Rascunho de versão em tabela própria

**Status:** Accepted

## Context
O projeto "Testar e publicar sem o Henry" (desenho de 2026-11-16) pede que uma
versão nova chegue ao Ziv sem mudar o que está no ar, para ser testada antes.
Até aqui, mandar código (`POST /api/apps/{id}/sources`) trocava na hora o
código guardado, a pasta do app e a versão em `app_versions`. Muita coisa lê
"a versão mais recente" de `app_versions`: a aprovação de segurança, a
conferência da pasta, o histórico, a versão de cada execução.

## Decision
- O rascunho vive numa tabela própria, `app_drafts`: arquivos, mudanças de
  ficha, nota, situação (`rascunho`, `testando`, `aprovada`, `reprovada`,
  `no_ar`, `cancelada`) e o número da versão quando vai ao ar.
- `app_versions` continua sendo só o que esteve no ar. O rascunho só ganha
  número quando é publicado, e a publicação passa pelo mesmo miolo de
  "mandar código" (`applySources`).
- As mudanças de ficha do rascunho valem sobre a ficha do momento da
  publicação, campo a campo: uma edição feita no site enquanto isso não se perde.
- Um rascunho novo cancela o anterior do mesmo app ("substituída por versão
  mais nova").
- Criar rascunho é aceito durante uma execução; publicar continua esperando a
  execução terminar.
- `app_versions.tested`: nulo nas versões antigas e na primeira de cada app,
  falso ("não testada") para código enviado direto, mudança na pasta do PC e
  rascunho publicado à mão; verdadeiro ("testada") fica para a Etapa 2.
- Nesta etapa, todo "Publicar" de app com tela avisa "Esta versão vai ao ar sem
  teste", porque nenhum app tem plano de teste ainda.

## Rationale
Com a tabela própria, nada do que hoje lê "a versão mais recente" precisa
mudar: a aprovação de segurança, a pasta e o histórico continuam significando
"a versão no ar". Um rascunho nunca publicado não deixa rastro no app.

## Trade-offs
**Pros:**
- Nenhuma mudança na aprovação, na pasta, nas execuções nem no histórico.
- Publicar um rascunho e mandar código direto usam o mesmo código.

**Cons:**
- Os arquivos do rascunho ficam guardados inteiros em JSON até ele ser
  publicado ou substituído (sem limpeza automática por enquanto).
- A receita ainda pertence ao app, não à versão; o desenho pede que pertença à
  versão, e isso entra quando o teste precisar ler a receita do rascunho.

## Alternatives Considered
- Rascunho como linha de `app_versions` com situação: descartado porque tudo o
  que lê "a versão mais recente" teria de filtrar a situação, e um esquecimento
  faria a aprovação ou as execuções usarem um rascunho.
- Pasta separada para o rascunho já nesta etapa: desnecessário até existir o
  teste (Etapa 2).
