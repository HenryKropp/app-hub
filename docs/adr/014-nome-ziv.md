# ADR 014 — O Kova vira Ziv, com três gerações de nomes convivendo

Data: 2026-10-29. Status: aceita. Amplia a [ADR 013](013-nome-kova-com-convivencia.md).

## Contexto
No mesmo dia em que virou Kova, o Henry percebeu que "Kova", dito em voz alta, soa como "cova". Para um produto cuja promessa é "um lugar seguro", isso pesa contra. Depois de várias rodadas de nomes, ele escolheu **Ziv** (hebraico para "brilho"): curto, fácil de falar em português e inglês. Ele também trocou o símbolo: a enseada com o ponto lia como um "C". O novo logo é o nome "ziv" desenhado, com o pingo do i coral e três raios finos, como um farol; o ícone é o quadrado petróleo só com esse i. O Henry pediu para trocar também os nomes técnicos, e não só o que aparece na tela.

## Decisão
Mesmo mecanismo da ADR 013, agora com três gerações. O Ziv **escreve** o nome novo e **lê** os dois antigos, do mais novo ao mais velho. Se existirem vários, vale o mais novo. Nada é recriado: adotar é sempre renomear.

| O quê | Novo | Antigos, ainda reconhecidos |
|---|---|---|
| Manifesto do app | `ziv.yaml` | `kova.yaml`, `app-hub.yaml` (um app que só tem um antigo continua salvando nele) |
| Marcadores "escrito pelo Ziv" | `# Ziv — manifesto do app`, `<!-- ziv:ficha -->` | os do Kova e do App Hub |
| Lixeira e temporários das versões | `.ziv-lixeira`, `.ziv-tmp` | `.kova-*`, `.app-hub-*` (ficam fora das versões) |
| Etiquetas das caixas e imagens | `ziv.app`, `ziv.deployment`, `localhost/ziv/` | `kova.*`, `app-hub.*`, `localhost/kova/`, `localhost/apphub/` |
| Rede de cada app | `ziv-apps-<id>` | `kova-apps-<id>`, `app-hub-apps-<id>` (removidas ao republicar ou deixar de compartilhar) |
| Nome das caixas | `ziv-<slug>-<id>` | — (as antigas são reconhecidas pela etiqueta) |
| Contêiner do Postgres | `ziv-db` | `kova-db`, depois `app-hub-db`, renomeado sozinho na subida |
| Pasta de dados dos compartilhados | `~/.local/share/ziv/shared` | `kova/shared`, depois `app-hub/shared`, renomeada sozinha na subida |
| Arquivo do agente baixado | `ZivAgente.exe` | — |
| Pacote do frontend | `ziv` | — |

As listas de nomes antigos ficam em um lugar por assunto (`manifest.LegacyFileNames`, `versions.LegacyTrashDirs`, `engine.LegacyLabels` e vizinhos). A adoção do banco e da pasta (`adopt.NewestContainer`, `adopt.NewestDir`) tenta o mais novo primeiro e para no primeiro que encontra.

Continua igual ao que a ADR 013 deixou de propósito: `apphub-*` no navegador, a instalação do agente no Windows (`AppHubAgente*.exe`, `%LOCALAPPDATA%\AppHub`), `APPHUB_*`, `X-Apphub-*`, `apphub_test` e o módulo Go.

Conferido na primeira subida nesta máquina (2026-10-29): `kova-db` virou `ziv-db` e `~/.local/share/kova/shared` virou `~/.local/share/ziv/shared`, sem perda.

## Rationale
O nome de marca precisa ser seguro para falar em voz alta. Trocar os nomes técnicos junto evita que o produto mostre "kova" em arquivos que as pessoas veem nas pastas dos apps. Manter o padrão de leitura em listas deixa uma próxima troca barata.

## Trade-offs
**Prós:** um nome só para tudo; nenhum app registrado quebra; a regra de adoção é a mesma de antes.

**Contras:** o código carrega três gerações de nomes até a migração forçada; a marca ainda não passou pelo INPI e os domínios `ziv.com` e `ziv.com.br` têm dono.

## Alternativas consideradas
- **Trocar só o que aparece na tela e deixar `kova` nos nomes técnicos:** recomendado, mas o Henry preferiu trocar tudo.
- **Manter Kova:** descartado pelo som de "cova".
- **Atrium, Domus:** o Henry gostou, mas já existem.

## Quando a convivência pode acabar
Igual à ADR 013, agora para as duas gerações antigas: quando nenhum app registrado tiver só `kova.yaml` ou `app-hub.yaml`, nenhuma caixa no ar usar etiqueta antiga e todos os servidores já tiverem subido com o Ziv.
