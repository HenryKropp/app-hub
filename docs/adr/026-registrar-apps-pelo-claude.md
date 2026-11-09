# 026 - Registrar e atualizar apps pelo Claude (MCP Etapa 2)

**Status:** Accepted

## Context
Na Etapa 1 do MCP (ADR 021), o Claude da pessoa já busca, roda e publica artifacts no Ziv. Faltava o principal: pegar um app que está numa pasta do computador e colocar no Ziv sem ninguém preencher a ficha à mão, e depois mandar versões novas dele.

Em 2026-11-06 o Henry decidiu:
- o Claude Code roda no PC e lê a pasta; a pessoa confere a ficha na conversa, e o registro só acontece depois do "pode registrar";
- vale para scripts novos, versões novas, vários apps de uma vez e apps com tela;
- uma versão nova pode mudar o código, o resumo, as entradas, as saídas e (nos scripts) o arquivo de início e as dependências. Nome, área, responsável, reserva e acesso só mudam pelo site, e o Claude avisa isso;
- os apps com tela existem por empresa e quem liga é o responsável pelo Ziv;
- publicar espera a aprovação quando a empresa exige aprovação, também pelo botão do site.

Desenho completo: `docs/superpowers/specs/2026-11-06-mcp-registrar-apps-design.md`.

## Decision
- **O código vai por um endereço de uso único, não dentro da chamada.** Um zip de até 12 MB não cabe numa chamada de ferramenta. `preparar_envio` cria um endereço `PUT /api/mcp/envios/{token}` que vale 30 minutos e uma vez. O token tem 256 bits e o banco guarda só o hash. Cada pessoa tem no máximo 5 envios abertos. O shell do Claude não tem a sessão da pessoa, então o endereço é a única credencial. A rota fica fora das checagens de navegador (só o `PUT`).
- **Tabela `mcp_uploads`** (migração 021) guarda o zip recebido até `registrar_app` usar. Um envio recusado pode ser mandado de novo no mesmo endereço.
- **O que não sobe é cortado nos dois lados.** No computador, os comandos (cmd, Git Bash e Mac/Linux) montam o zip na pasta temporária, nunca na pasta do app, e deixam de fora `.git`, `venv`, `node_modules`, `.env`, `*.pem` e `*.key`. No servidor, o pacote passa pela mesma lista e pelos mesmos limites do site (100 arquivos, 10 MB), e a resposta diz o que ficou de fora.
- **As versões dizem de onde vieram.** `origin = 'upload'` ("Código enviado") e `channel = 'mcp'` (selo "pelo Claude") aparecem no histórico e na página da versão.
- **Só apps com código guardado recebem versão nova.** Um app que roda de uma pasta do computador não tem o código no Ziv: ele só pode subir de novo como app novo, e `ver_app` diz isso (`codigo_guardado`). Voltar uma versão também volta o código guardado.
- **Apps com tela pelo mesmo caminho do import.** O import aceita `web` quando a empresa tem os apps com tela ligados. Com eles desligados, a resposta é "Os apps com tela ainda não estão ligados nesta empresa. Peça ao responsável pelo Ziv para ligar. Scripts sobem normalmente." O código vai para a pasta do servidor da empresa e o endereço legível vem do nome do app.
- **Publicar pelo Claude é o botão Publicar do site.** `publicar_app_com_tela` faz a mesma checagem, usa as mesmas marcas de "estou ciente" (só depois que a pessoa confirma) e passa pela mesma aprovação.
  - Os valores das chaves são preenchidos só no site. A rota que grava valores não está liberada para o Claude.
  - O endereço vem sempre do serviço (`AppURL`).
  - Se a versão nova não abrir, a anterior continua no ar.
- **Toda mudança feita pelo Claude passa pela auditoria** (`mcp.request.completed`), e só as rotas listadas em `mcpRoutes` podem ser chamadas.
- **Ferramentas novas:**
  - `preparar_envio`, `registrar_app` e `publicar_app_com_tela` mudam coisas;
  - `formato_do_app` só lê e traz o protocolo, as áreas e os limites;
  - `ver_app` passa a mostrar `codigo_guardado` e a ficha que dá para mudar.

## Consequences
- Um app sai da pasta e chega ao Ziv numa conversa, sem formulário. A pessoa confere tudo antes, e nenhum valor de chave passa pela conversa.
- Um endereço de envio vazado vale no máximo 30 minutos e uma vez, e o envio pertence só a quem o pediu.
- A ficha tem duas portas, o site e o Claude, e as regras dela ficam num lugar só (`checkSentFiles` e `recordSentVersion`), usado pelas duas.
- Os apps que rodam de uma pasta do computador continuam sem versão pelo Claude, até serem registrados de novo com o código guardado.

Relacionados: ADR 010 (apps compartilhados), ADR 021 (MCP dentro do Ziv).
