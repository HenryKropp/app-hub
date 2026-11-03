# ADR 016 — Agente em um EXE só, com o endereço da empresa dentro

Data: 2026-11-03. Status: aceita.

## Contexto
Cada empresa do portal (`/w/<empresa>`) baixava o Agente num ZIP com o EXE e um `apphub-url.txt` ao lado. A pessoa precisava extrair os dois para a mesma pasta. Se abrisse o EXE de dentro do ZIP, ele se conectava à empresa errada. O Henry pediu que baixar o Agente fosse "seamless", logo depois do cadastro, e que houvesse um aviso quando o computador não estivesse conectado.

## Decisão
1. **Um EXE só.** Para as outras empresas, o servidor anexa ao fim do EXE um bloco com o endereço da empresa, a marca `ZIVHUB01` e o tamanho. O arquivo baixado se chama `ZivAgente-<empresa>.exe`. O Windows ignora bytes depois do programa. O Agente lê esse bloco no próprio arquivo quando não há `apphub-url.txt` ao lado. `APPHUB_URL` e o arquivo ao lado continuam valendo. A empresa principal recebe o EXE sem bloco.
2. **Depois do cadastro, a tela "Conecte seu computador"** (`/instalar-agente`) mostra o download, três passos e o estado ao vivo. A pessoa pode pular.
3. **Aviso nos dois lugares.**
   - Uma faixa no topo das páginas: "Seu computador não está conectado ao Ziv. Sem o Agente, não será possível rodar os apps do catálogo."
   - Um diálogo ao clicar em Rodar sem computador conectado.
4. **Começar sem a etapa do nome**, que o cadastro já resolve. Ficam 4 etapas.

## Trade-offs
**Prós:** nada para extrair. Abrir o arquivo direto do Downloads já conecta à empresa certa.
**Contras:**
- Anexar dados ao EXE invalida uma assinatura digital, caso o EXE venha a ser assinado. Se isso acontecer, o endereço terá de ir por outro caminho, como o nome do arquivo ou um campo assinado.
- O novo formato só vale com um Agente compilado a partir desta versão. `run-replit.sh` recompila.

## Alternativas consideradas
- **ZIP com arquivo ao lado:** é o problema que este ADR resolve.
- **Endereço no nome do arquivo:** o navegador renomeia arquivos repetidos (`(1)`), e isso é frágil.
- **Pareamento por código digitado:** é um passo a mais para a pessoa.
