# MCP Etapa 2 — subir apps prontos pelo Claude

**Data:** 2026-11-06 · **Status:** desenho para aprovação · Base: `docs/PROTOCOLO-REGISTRO.md` v0.2, `docs/superpowers/specs/2026-11-04-mcp-design.md` (Etapa 2), ADR 010 (apps compartilhados), ADR 021 (MCP)

## 1. Por que

O Henry tentou subir um programa em Python para o Ziv pelo Claude Code e ouviu que não dava: o MCP do Ziv só publica artifacts. Hoje um app só entra no Ziv no ar pelo site (Registrar apps › script), preenchendo tudo à mão. A promessa do Ziv é que quem fez uma ferramenta com IA não precisa entender de servidor para colocá-la à disposição da equipe: "sobe esse programa para o Ziv" deve bastar.

**Sucesso:** no Claude Code do PC, o Henry aponta uma pasta, o Claude mostra na conversa o que entendeu, ele responde "pode registrar" e o app aparece no Ziv pronto para usar. Um script, rodando pelo Agente; um app com tela (Streamlit, Flask), no ar no servidor com link próprio.

## 2. O que entra (decidido pelo Henry)

1. **Onde:** Claude Code no PC da pessoa. O Claude lê a pasta direto no disco, seguindo o Protocolo de Registro.
2. **Conferir na conversa.** O Claude mostra o resumo (nome, descrição, tipo, o que o app pede e entrega, área, pontos de atenção, chaves que faltam). Só com o "pode registrar" da pessoa ele chama a ferramenta que registra. Depois dá para editar tudo pelo site.
3. **Quatro casos, todos nesta entrega:**
   - **Script novo:** o código sobe para o Ziv e o app roda no PC de quem clica Rodar, pelo Agente (como os scripts importados pelo site hoje).
   - **Atualizar um app existente:** o código novo vira versão nova do mesmo app (aparece no histórico de versões), sem criar outro app.
   - **Vários de uma vez:** uma pasta com vários programas; o Claude faz um resumo e pede confirmação para cada um, um por vez.
   - **App com tela (Streamlit, Flask e afins):** o código sobe para o servidor do Ziv e roda lá para todos, como app compartilhado, com link `https://<app>.zivapp.duckdns.org`.
4. **Atualizar app com tela publica sozinho.** A versão nova sobe ao lado da atual (o "publicar lado a lado" do ADR 010); se abrir direito, troca; se falhar, a antiga continua no ar e o Claude mostra o erro na conversa.
5. **Apps com tela em todas as empresas.** Hoje os apps compartilhados só funcionam na empresa principal; nas outras o Ziv responde "A execução de apps compartilhados está desativada para esta empresa" (`handler/shared_gateway.go`, `workspace/portal.go`). Esta entrega liga os apps com tela em todas as empresas, com isolamento entre elas: caixas (contêineres), redes, bancos de app, cópias e endereços de uma empresa nunca se misturam com os de outra, e uma empresa não alcança os apps da outra. Hoje os nomes usam o id do app (`app-hub.app=<id>`, `app-hub-apps-<id>`, banco `app_<id>`, endereço `<slug>.zivapp...`), que se repete entre empresas: tudo isso passa a carregar a empresa. O teste do Henry é na empresa Teste.

## 3. Fora desta entrega

- Registrar pelo claude.ai/app do Claude (chat sem acesso ao disco). Fica para depois, lendo a pasta pelo Agente, como previa o desenho do MCP de 2026-11-04.
- Apps de janela (tkinter com `mainloop`, PyQt) e `.exe`: não rodam no servidor. Se o Claude encontrar um, explica que esse tipo ainda não sobe pelo Claude e indica o site.
- Criar apps do zero (Etapa 3).
- Agendar pelo Claude.
- Servidor maior (os apps com tela de todas as empresas dividem o mesmo servidor): o servidor grátis tem 1 GB de RAM, então cabem poucos apps com tela ligados ao mesmo tempo (eles dormem depois de 30 minutos sem uso, ADR 010).

## 4. Regras que não mudam

- **Só leitura no PC.** O Claude não altera, move, apaga nem roda nada na pasta do app.
- **Senhas e chaves nunca sobem.** `.env`, `.pem`, `.key` e afins ficam de fora do pacote (o Ziv já recusa esses arquivos). Se o código usa chaves, o Claude lista só os nomes e diz onde preencher no Ziv: no cofre do app compartilhado, pelo site. Valores nunca aparecem na conversa.
- **Possível senha no código:** o Ziv avisa e o Claude pergunta à pessoa antes de mandar de novo confirmando (o mesmo que o site e o `publicar_artifact` fazem); se a política da empresa bloquear, não há como confirmar.
- **Nada inventado.** O que não tem evidência no código vira pergunta na conversa, não palpite (Protocolo, seção 4).
- **Permissões de sempre.** Registrar app novo exige "registrar apps"; atualizar exige poder editar aquele app; publicar app compartilhado exige o que o botão Publicar exige hoje. O Ziv confere, não o Claude.
- **Marca "via Claude".** O histórico do app registra que a versão veio pelo Claude, e de quem.

## 5. Como funciona (decisões técnicas — chamadas minhas)

- **Pacote por upload, não pela conversa.** O código não vai dentro da chamada da ferramenta: um app de algumas centenas de KB viraria dezenas de milhares de palavras que o Claude teria de escrever. Em vez disso, uma ferramenta devolve um endereço de envio de uso único (curto prazo, preso à pessoa e à empresa) e o Claude Code manda um `.zip` da pasta com `curl` (existe no Windows 10+). Depois a ferramenta que registra aponta para esse envio. O zip deixa de fora `.git`, `.venv`, `venv`, `node_modules`, `__pycache__`, saídas acumuladas e arquivos de senha; o Ziv confere de novo do lado dele.
- **Mesmas regras do site.** O registro pelo Claude passa pela mesma validação do `POST /api/scripts/import` (manifesto válido, arquivo de início presente, limites de tamanho, recusa de arquivos de senha, varredura de segredos). Apps com tela usam o mesmo caminho de guardar o código no banco e montar a pasta no servidor, e depois o fluxo de apps compartilhados (checagem, teste, publicação).
- **Limites:** os do site hoje (até 100 arquivos e 10 MB somados). Se um app com tela real não couber, o limite pode subir nessa entrega, desde que caiba na memória do servidor.
- **Ferramentas novas no MCP (nomes provisórios):**
  - `formato_do_app`: devolve o formato do manifesto, os tipos aceitos, as áreas da empresa e o Protocolo de Registro como instrução.
  - `preparar_envio`: devolve o endereço de envio e o comando para mandar o zip.
  - `registrar_app`: com o envio e o manifesto, cria o app (script ou com tela) ou, com `app_id`, manda versão nova de um app existente. Para app com tela, publica e espera o resultado (ou devolve onde acompanhar).
  - `buscar`/`ver_app` atuais servem para o Passo 0 do Protocolo ("já existe?").
- **App com tela no servidor:** o Ziv monta a pasta a partir do código guardado, roda a checagem do ADR 010 (receitas Streamlit/Python web/Node, banco automático quando o código lê `DATABASE_URL`) e publica. Pendências da checagem voltam para o Claude em linguagem simples, para ele explicar ou corrigir com a pessoa.

## 5b. Respostas da revisão (chamadas minhas, para o Henry conferir)

- **Quais apps o Claude atualiza:** só os que têm o código guardado no Ziv (importados pelo site ou subidos pelo Claude). Um app registrado por pasta do PC recebe uma recusa explicando que ele precisa ser subido de novo. Hoje não existe atualização de código guardado (`ImportScript` só cria): a entrega cria esse caminho.
- **Aprovação da empresa:** se a empresa exige aprovação de versões novas, a versão vinda pelo Claude espera aprovação como qualquer outra. O Claude avisa "esperando aprovação de um administrador" em vez de dizer que está pronto.
- **Chaves e avisos da checagem (app com tela):** o Claude registra o app e, antes de publicar, mostra na conversa cada aviso da checagem (ex.: usa caminho do Windows) e pergunta se a pessoa está ciente; a resposta é gravada como o "estou ciente" do site. Se o app precisa de chaves, o Claude para depois de registrar, diz quais chaves preencher no cofre do app pelo site (com o link) e publica quando a pessoa avisar que preencheu. Para isso existe uma ferramenta de publicar separada do registrar.
- **Caminho de registro para apps com tela:** uma variante do import de scripts (que hoje força `kind=script` e apaga o comando), com as mesmas recusas e varredura de segredos.
- **"Via Claude" no histórico:** precisa de campo novo nas versões (hoje a origem aceita só `original | folder | restore`).
- **Voltar versão de script importado:** hoje voltar uma versão reescreve a pasta mas não o código guardado, e o próximo Rodar falha com "código mudou". Com versões vindas do Claude isso fica comum, então a entrega corrige.

## 6. Como saberemos que deu certo

Teste de verdade do Henry, no Ziv no ar, empresa Teste:
1. Subir um script em Python novo pelo Claude Code e rodá-lo pelo Ziv.
2. Mudar uma linha do mesmo script e pedir para atualizar: aparece versão 2 no histórico, sem app duplicado.
3. Subir uma pasta com dois programas de uma vez, confirmando cada um.
4. Subir um app Streamlit simples e abri-lo pelo link próprio; atualizar e ver a versão nova no ar.
5. Tentar subir com um `.env` na pasta: o `.env` não sobe e o Claude diz qual chave preencher no Ziv.

## 7. Deixado para quem constrói

Nomes finais das ferramentas, formato exato do endereço de envio, como o Ziv guarda o código dos apps com tela, quanto tempo o envio vale, o texto das instruções do MCP (a partir do Protocolo), e os detalhes de teste automático.
