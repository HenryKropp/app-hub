# 042 - Montagem sempre baixa as imagens da internet

**Status:** Accepted

## Context
A revisão de segurança de 2026-11-13 confirmou que o Dockerfile trazido por um
app é montado no depósito de imagens que todas as empresas compartilham no
servidor. Com `FROM`, `COPY --from` ou `RUN --mount=...,from=` apontando para
`localhost/ziv/<outra empresa>/...`, o app de uma empresa lia o código e os
arquivos do app de outra. Os testes no Podman 4.9 mostraram mais três caminhos:
nome curto (`ziv/...` acha a imagem local), número da imagem (prefixo de 6
letras basta) e `ONBUILD` de uma imagem pública que aponta para uma imagem local.

## Decision
`Podman.Build` passa sempre `--pull=always`. Toda imagem citada pela receita
vem do registro na internet; uma imagem só local, ou citada pelo número, faz a
montagem falhar. Etapas do próprio Dockerfile (`FROM ... AS a`, `COPY --from=a`)
continuam funcionando. A falha ganha uma mensagem própria: "A receita usa uma
imagem guardada no servidor...".

## Rationale
Fecha todos os caminhos testados com uma linha, inclusive os que uma análise do
texto do Dockerfile deixaria passar (variáveis em `FROM`, `ONBUILD` de imagem
baixada). As receitas do Ziv já usam imagens completas do docker.io, e o cache
de camadas continua valendo quando a imagem baixada é a mesma.

## Trade-offs
**Pros:**
- Uma mudança pequena, testada de verdade contra os cinco jeitos de vazar.
- Vale para receitas do Ziv e Dockerfiles trazidos pelo app.

**Cons:**
- Toda montagem consulta o registro; se o Docker Hub estiver fora, a montagem
  falha (já falhava para instalar bibliotecas).
- Ninguém consegue montar sobre uma imagem própria guardada só no servidor.

## Alternatives Considered
- Ler o Dockerfile e recusar `localhost/ziv/*`: frágil (variáveis, nome curto,
  número da imagem, `ONBUILD`).
- Um depósito de imagens por empresa: isola tudo, mas muda todas as operações do
  motor e duplica as imagens base (o navegador tem ~1,5 GB) no disco do e2-micro.
  Fica para quando houver Agente na nuvem do cliente.
