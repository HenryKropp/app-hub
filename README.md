# Ziv

Antes chamado App Hub.

Plataforma para empresas registrarem, compartilharem e executarem microapps.
Cada app tem responsável, ficha, permissões, entradas e histórico.

## Estado atual — 23/11/2026

- No ar 24h em servidor gratuito no Google Cloud (ADR 022); desenvolvimento
  local. Interface em React/Vite, servidor em Go, banco PostgreSQL.
- Portal multiempresa com isolamento por empresa; catálogo, página do app em
  abas, cargos, áreas, apps abertos ou restritos.
- Agente no Windows validado de verdade: roda scripts e programas, atualiza
  sozinho, vários apps ao mesmo tempo, Parar.
- Apps com tela no servidor (compartilhados pela empresa), com navegador
  automático; agendamento; cofre de senhas por app.
- Versões com histórico e voltar atrás; rascunho; teste de script no PC antes de
  publicar; governança com aprovação do líder e, opcionalmente, da TI.
- MCP: o Claude cria, registra, testa, roda e publica apps; artifacts com link
  externo.
- Ainda não: login com Google/Microsoft, rodar pelo celular sem o Agente,
  instalação na nuvem do cliente, e-mail em produção (falta domínio próprio),
  instalador único e assinatura digital.

A lista de tarefas, com o que está pendente, fica em [docs/TASKS.md](docs/TASKS.md).

Ter código implementado não significa que todos os cenários foram verificados.
Consulte os [limites de verificação](docs/SCRIPTS.md#limites-da-verificação).

## Como funciona

O servidor central mantém catálogo, contas, permissões, código importado e
histórico. O agente instalado no Windows recebe os pedidos e executa os apps com
as permissões do usuário. O navegador é a interface atual.

A evolução para um aplicativo Windows com janela própria e as mesmas telas do
site já está prevista na [decisão de arquitetura 005](docs/adr/005-app-hub-no-computador-com-hub-central.md).
Não equivale a instalar um servidor e banco separados em cada computador.

A execução gerenciada na nuvem, combinada com execução local para apps que
dependem do escritório, faz parte da visão de produto. **Não é uma capacidade
implementada pelo fluxo atual de scripts.**

## Executar em desenvolvimento

Consulte [replit.md](replit.md) para configuração e inicialização no Replit e
[docs/INFRASTRUCTURE.md](docs/INFRASTRUCTURE.md) para os componentes e variáveis.

Com as dependências e variáveis configuradas:

```sh
bash scripts/run-replit.sh
```

O comando prepara o frontend, compila servidor e agente e inicia o Ziv. No
Replit, use o workflow **Start application**. Não inclua credenciais no código.

No próprio computador (Windows com Git Bash, Mac ou Linux), use
`bash scripts/run-local.sh`; o passo a passo está em
[Rodar no próprio computador](docs/INFRASTRUCTURE.md#rodar-no-próprio-computador-windows-mac-ou-linux--01112026).

## Mapa da documentação

| Documento | Finalidade |
|---|---|
| [PRODUCT.md](PRODUCT.md) | Público, posicionamento e princípios de produto |
| [Roadmap](ROADMAP.md) | Frentes de produto em ordem, do hackathon à nuvem gerenciada |
| [Especificação v3](docs/especificacao/Especificação%20do%20produto%20v3.md) | Visão ampla, incluindo a futura nuvem gerenciada |
| [PRD](docs/PRD.md) | Requisitos do produto e contexto do piloto original |
| [Scripts](docs/SCRIPTS.md) | Importação, entradas, dependências, resultados e segurança |
| [Infraestrutura](docs/INFRASTRUCTURE.md) | Ambiente atual e diferenças em relação ao piloto local |
| [Replit](replit.md) | Instruções de execução neste ambiente |
| [Decisões de arquitetura](docs/adr/) | Decisões e justificativas; algumas descrevem fases anteriores |
| [Protocolo de registro](docs/PROTOCOLO-REGISTRO.md) | Registro e análise dos apps |
| [Tarefas](docs/TASKS.md) | Tarefas por projeto, com o que está feito e o que falta |
| [Planos e especificações](docs/superpowers/) | Desenhos e planos de implementação por etapa |

As especificações v1/v2 e os planos antigos são histórico, não prova de
funcionalidades entregues. Quando houver divergência operacional com o piloto
WSL, use as instruções atuais de infraestrutura e de execução no Replit.

## Segurança e pendências

- Scripts locais executam com as permissões da pessoa. **A pasta temporária não
  é uma sandbox de segurança.** Use apenas código e dependências revisados.
- A detecção de credenciais na importação é heurística, não um cofre de segredos.
- Isolamento forte de microapps externos, gestão de credenciais por empresa e
  publicação revisada entre empresas continuam pendentes.
- Instalador único, janela desktop e assinatura digital fazem parte da evolução
  prevista; não devem ser apresentados como concluídos.
- Não remova o login do Ziv para contornar restrições de acesso do ambiente.

Enviar o código ao GitHub não publica a aplicação nem conclui a validação Windows.
