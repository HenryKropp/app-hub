# Kova

Antes chamado App Hub.

Plataforma para empresas registrarem, compartilharem e executarem microapps.
Cada app tem responsável, ficha, permissões, entradas e histórico.

## Estado atual — 26/10/2026

- Interface web em React/Vite, servidor em Go e banco PostgreSQL.
- Ambiente de desenvolvimento no Replit; execução local no Windows pelo Agente
  do Kova.
- Importação de scripts Python e JavaScript pelo navegador, com formulário de
  entradas, envio de arquivos, preparação de dependências e coleta de resultados.
- Catálogo, cargos, áreas, controle de acesso e histórico de versões.
- A execução completa dos scripts em Windows **ainda não foi validada**. No teste
  guiado, o agente encontrou uma barreira de acesso ao endereço privado de
  desenvolvimento do Replit antes de conseguir conectar.

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

O comando prepara o frontend, compila servidor e agente e inicia o Kova. No
Replit, use o workflow **Start application**. Não inclua credenciais no código.

## Mapa da documentação

| Documento | Finalidade |
|---|---|
| [PRODUCT.md](PRODUCT.md) | Público, posicionamento e princípios de produto |
| [Especificação v3](Documentação/Especificação%20do%20produto%20v3.md) | Visão ampla, incluindo a futura nuvem gerenciada |
| [PRD](docs/PRD.md) | Requisitos do produto e contexto do piloto original |
| [Scripts](docs/SCRIPTS.md) | Importação, entradas, dependências, resultados e segurança |
| [Infraestrutura](docs/INFRASTRUCTURE.md) | Ambiente atual e diferenças em relação ao piloto local |
| [Replit](replit.md) | Instruções de execução neste ambiente |
| [Decisões de arquitetura](docs/adr/) | Decisões e justificativas; algumas descrevem fases anteriores |
| [Protocolo de registro](docs/PROTOCOLO-REGISTRO.md) | Registro e análise dos apps |
| [Roadmap histórico](docs/TASKS.md) | Etapas do piloto; não é uma lista atualizada das tarefas do projeto |
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
- Não remova o login do Kova para contornar restrições de acesso do ambiente.

Enviar o código ao GitHub não publica a aplicação nem conclui a validação Windows.
