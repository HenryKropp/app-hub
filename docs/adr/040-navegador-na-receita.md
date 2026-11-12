# ADR 040: Navegador na receita automática (Playwright em Python)

**Data:** 2026-11-12
**Status:** Aceito

## Contexto

O app de relatórios (BTG e BBA, empresa Teste) baixa os relatórios do BTG com o Playwright, num Chromium com janela, porque o BTG bloqueia o navegador invisível. A receita automática do Ziv montava o app sem navegador. Por isso o BTG falhava no servidor. A versão 9 do app tentou instalar o Chromium cada vez que o app acordava, dentro de um limite de 256 MB. O app parou de responder e o Ziv não conseguiu religá-lo.

Havia duas saídas: cada app trazer o próprio Dockerfile, ou o Ziv montar o navegador sozinho. O Henry escolheu a segunda: é o jeito produto, o criador não precisa fazer nada, e a parte pesada fica uma vez só no disco.

## Decisão

- Um app Python (Streamlit, FastAPI, Flask ou Django) com `playwright` no `requirements.txt`, ou que importa `playwright`, recebe a receita com navegador:
  - base `python:<versão>-slim-bookworm`, porque o Playwright só instala as bibliotecas do sistema nas versões do Debian que ele suporta;
  - um passo de navegador antes das bibliotecas do app: `libatomic1` (sem ela, o próprio Playwright não liga), `xvfb` e `xauth`, o Playwright e o Chromium com as dependências (`--with-deps`), em `PLAYWRIGHT_BROWSERS_PATH=/ms-playwright`. Esse passo tem o mesmo texto para todos os apps, então a montagem o reaproveita e o disco guarda uma cópia só;
  - depois das bibliotecas do app, `playwright install chromium` não faz nada quando a versão é a mesma, e baixa o navegador certo quando o app fixa outra versão.
- O Ziv não liga a tela virtual: o app chama `Xvfb :99` e põe `DISPLAY=:99` quando precisa de janela.
- Memória: app com navegador fica com pelo menos 512 MB (`recipes.BrowserMemoryMB`). Isso vale mesmo onde o teto da empresa por app é menor (256 MB na Teste) e mesmo se a memória gravada antes era menor, porque publicar pelo MCP reenvia a memória que o app já tinha. Pedir mais que 512 numa empresa com teto menor continua bloqueado.
- A checagem antes de publicar troca o aviso "a receita não traz navegador" por "Navegador: ok" nesses apps. Selenium, Puppeteer e Playwright em Node continuam com o aviso.
- No servidor, a memória somada dos apps ligados (`SHARED_APPS_MEMORY_BUDGET_MB`) passa de 512 para 768 MB. Assim cabem um app com navegador e um app comum de 256 MB ao mesmo tempo. O limite de 2 apps ligados ao mesmo tempo continua.

## Consequências

- A imagem de um app com navegador tem cerca de 1,8 GB. O passo do navegador é compartilhado entre os apps. Na primeira montagem no servidor gratuito, o download pode levar alguns minutos, dentro dos 15 minutos de limite.
- Conferido montando de verdade a receita gerada: o Chromium abriu com janela no Xvfb, com limite de 512 MB.
- Com 1 GB de RAM, cabe um app com navegador ligado por vez. Para mais, é preciso uma máquina paga (ADR 022).
- Fica para depois: Playwright em Node e Selenium.
