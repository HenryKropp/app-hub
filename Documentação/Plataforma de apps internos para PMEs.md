# Plataforma de apps internos para PMEs

Sep 29, 2026 · @Henry

## Resumo

PMEs brasileiras já criam apps com IA, mas esses apps ficam presos no computador de quem criou. Queremos ser a casa onde os apps da empresa são criados, rodam e são mantidos: acessíveis de qualquer lugar, usáveis por qualquer colega e vivos mesmo quando quem criou sai.

**Posicionamento:** uma plataforma onde PMEs sem TI transformam os apps que criam com IA em ferramentas internas que qualquer pessoa usa, hospedadas, organizadas e cobradas em reais.

O desenvolvimento acontece na IA que a pessoa já usa (Claude, Cursor, ChatGPT), conectada à plataforma por um servidor MCP. O uso e a gestão acontecem numa interface web simples.

## O problema

Criar software pequeno com IA ficou fácil; rodar, compartilhar e manter esse software continua difícil. O app roda na máquina de quem criou, e tudo depende dessa pessoa.

**Dores principais (resolvemos primeiro)**

1. **Acesso de qualquer lugar:** o app só funciona naquele computador; não dá pra usar de casa ou em viagem.
2. **Continuidade:** se quem criou sai, adoece ou tira férias, ninguém sabe rodar e o app morre.
3. **Compartilhamento:** colegas não sabem instalar, configurar ou rodar os apps uns dos outros.
4. **Manutenção:** apps feitos com IA quebram (uma API muda, uma biblioteca atualiza) e só quem criou conserta.

**Dores secundárias (com clientes maiores)**

5. **Segurança de chaves e dados:** senhas e chaves de API espalhadas em pastas e computadores.
6. **Retrabalho:** duas pessoas criam a mesma solução porque ninguém sabe o que já existe.
7. **Falta de padrão:** cada app é feito de um jeito, o que dificulta manter e evoluir.

**Argumento de venda:** as alternativas (AWS, Vercel, Replit) são feitas pra programadores, cobram em dólar e têm conta variável.

**Evidência: a Alis (4 pessoas).** A equipe já tem vários apps criados com Claude (extração de documentos, notícias, entre outros), cada um numa pasta. Ninguém sabe rodar os apps dos outros, e um deles, que roda local, não pode ser usado de casa.

## Para quem

PMEs brasileiras de até 50 pessoas, sem TI próprio, que já criam apps com IA. O segmento é definido por comportamento, não por setor.

| Persona | Quem é | O que quer |
| --- | --- | --- |
| Criador | Quem faz os apps com IA | Publicar rápido e parar de ser chamado pra rodar pros outros |
| Usuário | O colega que só usa | Abrir no navegador, clicar e ter o resultado, sem saber o que é Python |
| Sócio/dono | Quem paga | Saber o que existe e ter certeza de que nada morre se alguém sair |

Na faixa de 1 a 10 pessoas dominam acesso remoto e continuidade; de 10 a 50 ganham peso catálogo, controle de acesso e padronização. Quem decide a compra é o sócio ou o gerente de operações, às vezes com um TI terceirizado opinando.

## Mercado e concorrência

O problema já foi apontado lá fora e o Brasil é o país onde ele mais aparece, mas não encontramos nenhuma startup brasileira focada nele.

**Sinais de mercado**

- A Y Combinator pede startups de "uma nuvem para software pequeno": compartilhar um app interno deveria ser tão fácil quanto um Google Doc ([YC RFS](https://www.ycombinator.com/rfs)).
- O Brasil lidera o vibe coding nas empresas: 21,6% dos executivos usam essas plataformas, contra 15,6% nos EUA. Segurança e compliance travam 42,1% das empresas, e 22,4% dos pilotos nunca chegam em produção ([Jitterbit, via Times Brasil/CNBC](https://timesbrasil.com.br/colunas/ai-451/brasil-lidera-automacao-ia-vibe-coding-tropecos-jitterbit-2026/)).

| Concorrente | O que faz | Por que não atende nossa PME |
| --- | --- | --- |
| [Vercel](https://vercel.com/pricing) | Hospeda sites e apps web | Feito pra programador; US$ 20/mês por dev + uso variável em dólar; proteção por senha à parte |
| Replit, Lovable, Bolt | Criam e hospedam apps com IA | Foco em criar; cada um só cuida do que foi feito nele |
| Microsoft (Power Apps, [Agent 365](https://www.helpnetsecurity.com/2026/05/14/copilot-studio-security-governance-updates/)) | Criação e governança de apps e agentes | Exige ecossistema Microsoft e TI pra configurar |
| [Island](https://www.globenewswire.com/news-release/2026/08/03/3337542/0/en/island-secures-vibe-coding-with-new-enterprise-vibe-publishing.html) | Governa apps criados por funcionários | Foco em grandes empresas e segurança |
| Claude, ChatGPT | Criam e compartilham artefatos | Não são o lugar onde a empresa roda e mantém tudo |

**Onde está o espaço:** empresa pequena, sem TI, que cria em várias IAs e só quer que os apps funcionem pra todo mundo, com preço previsível em reais.

## Arquitetura do produto

O produto tem duas portas de entrada sobre um mesmo núcleo: o criador trabalha pelo MCP, dentro da IA que já usa; usuários e sócios usam a web. Tudo passa pela camada de gestão, que roda sobre nuvem alugada.

&#91;embedded content: arquitetura · 3 portas de entrada, gestão no centro, nuvem por baixo\]

A gestão é onde está o diferencial; a nuvem é quase commodity e pode ser alugada pronta no começo.

## Camada de gestão: os módulos

A gestão é o produto de verdade e onde está o diferencial. O coração é a interface automática: transformar "um script na pasta do Henry" em "uma ferramenta da empresa".

| Módulo | O que faz | Dores | Versão |
| --- | --- | --- | --- |
| 1. Publicação | Sobe o app (pasta, GitHub ou via MCP); a IA detecta linguagem, dependências, entradas e chaves, e tira senhas do código | 1, 3, 5 | V1 |
| 2. Ficha do app | Página gerada pela IA: o que faz, entradas, saídas, exemplo, responsável, última execução | 2, 3 | V1 |
| 3. Execução com interface automática | Gera a tela (campos, botão Rodar, resultado ou arquivo); permite agendar e enviar por e-mail | 1, 3 | V1 |
| 4. Catálogo | Todos os apps num lugar, busca em linguagem natural, aviso de app parecido antes de criar | 6 | V2 |
| 5. Acesso e cofre | Login Google/Microsoft, permissão por app, chaves guardadas sem ninguém ver | 5 | V1 (login), V2 (cofre) |
| 6. Saúde e manutenção | Acompanha execuções, avisa erro, IA explica em português e propõe correção, histórico e rollback | 4 | V2 |
| 7. Continuidade | Responsável e reserva por app; transferência quando alguém sai; destaque pra apps sem dono | 2 | V2 |
| 8. Painel do sócio | Apps existentes, uso, quem usa, problemas, custo | 2, 6 | V3 |

Os números da coluna Dores seguem a lista da seção O problema.

## MCP: o ambiente de desenvolvimento

Não construímos a IA que programa nem um editor próprio. Construímos o ambiente da empresa e o expõemos por um servidor MCP (Model Context Protocol), padrão aberto já suportado por Claude, Cursor, ChatGPT e VS Code.

**Exemplo de fluxo.** O criador pede ao Claude: "cria um app que resume as notícias das empresas da carteira todo dia às 7h". Pelo MCP, o Claude:

1. consulta o catálogo pra ver se já existe algo parecido ou um componente reaproveitável;
2. puxa os padrões da empresa (estrutura, visual, uso do cofre);
3. cria o app dentro da plataforma, com histórico desde a primeira linha;
4. publica, dá acesso e agenda.

**Ferramentas que o MCP expõe**

| Ferramenta | Para quê |
| --- | --- |
| Catálogo | Buscar apps e componentes existentes |
| Padrões | Modelos e regras da empresa pra IA seguir |
| Projeto | Criar, editar e salvar versões com histórico automático |
| Cofre | Usar chaves de API sem expô-las |
| Publicação | Colocar no ar, dar acesso, agendar |
| Saúde | Ler erros e logs pra IA corrigir |

**Por que MCP:** é neutro entre IAs, não exige construir editor ou chat e encaixa no hábito atual de quem cria.

**Cuidados**

- O MCP é pro criador; o usuário continua precisando da interface web.
- Ações sensíveis (publicar pra empresa toda, mexer em dados) exigem permissão por pessoa e confirmação humana.
- Dependemos de como os clientes de IA tratam MCP; a importação manual fica como alternativa.

## Roadmap

Começamos por um piloto do módulo 1 (publicação) na Alis; se funcionar bem lá, validamos com outras PMEs antes de construir a V1 completa.

&#91;embedded content: roadmap · piloto, V1, V2, V3 e os portões entre eles\]

Começar pela V3 seria competir com o Lovable no primeiro dia; começar pela V1 resolve algo que ninguém resolve bem pra PME. O MVP resolve acesso remoto, compartilhamento e boa parte da continuidade.

## Modelo de negócio

Cobramos uma mensalidade fixa por empresa, em reais, e rodamos os apps em nuvem alugada. Não precisamos de data center.

**Infraestrutura.** No MVP, usamos plataformas prontas por trás (Fly.io, Railway ou Render, via API): mais caro por app, mas valida rápido. Com clientes pagando, migramos pra AWS ou similar, com região em São Paulo se o cliente quiser os dados no Brasil.

**De onde vem a margem**

- **Densidade:** dezenas de apps pequenos dividem o mesmo servidor, isolados entre si.
- **Eficiência invisível:** apps sem uso "dormem" e acordam quando alguém abre; tarefas agendadas só rodam no horário.
- **Valor da simplicidade:** o cliente paga por "funciona sem eu entender nada", não só pelo servidor.

**Preço.** Hipótese inicial: preço fixo por faixa (por exemplo, até 10 apps e 10 usuários por um valor mensal). Pra PME, previsibilidade pesa mais que ser o mais barato. Cobrar por usuário desestimula o uso; cobrar por app desestimula criar. Os valores ainda serão definidos nas entrevistas.

Gerenciar a nuvem que o cliente já tem (FinOps) fica fora do escopo: nosso público hoje quase não gasta com nuvem.

## O que buscamos num sócio técnico

O produto é técnico no núcleo: rodar código gerado por IA, de vários clientes, com segurança. O sócio técnico constrói e é dono dessa parte; o lado de negócio cuida de clientes, validação e vendas.

**Experiência desejada**

- Infraestrutura em nuvem e containers (AWS ou similar, Docker, orquestração).
- Isolamento e segurança multi-cliente (sandboxes, micro máquinas virtuais, gestão de segredos).
- Produtos de plataforma para desenvolvedores (deploy, CI, logs).
- Integração com LLMs e MCP.

**Perfil:** gosta de construir rápido e em pequenos passos, topa começar em cima de plataformas prontas e tem interesse em PMEs, não só em tecnologia.

## Riscos e perguntas em aberto

O maior risco é um gigante (Microsoft, Anthropic, OpenAI, Replit) resolver o mesmo problema dentro do próprio ecossistema antes de ganharmos tração.

| Risco | Mitigação |
| --- | --- |
| Labs de IA e Microsoft lançam hospedagem e gestão de apps | Ser neutro entre IAs, simples pra PME sem TI e cobrar em reais |
| Segurança: código de um cliente acessar dados de outro | Isolamento forte desde o dia 1; sócio técnico com experiência nisso |
| Apps das PMEs serem complexos demais pra interface automática | Começar por scripts de entrada → processamento → saída; validar tipos de app nas entrevistas |
| PME não sentir a dor forte o suficiente pra pagar | Validar disposição a pagar antes de construir |
| Dependência do padrão MCP e dos clientes de IA | Manter importação manual como alternativa |

**Perguntas em aberto**

- Que tipo de app as PMEs criam? Scripts simples ou sistemas complexos?
- Quem sente mais a dor, o criador ou o sócio? Pra quem vendemos?
- Quanto pagariam por mês, e em qual modelo (faixa, usuário, app)?
- Quantas PMEs brasileiras já criam apps com IA hoje?
- Como achar os primeiros clientes: comunidades, parceiros ou conteúdo?

## Plano de validação

Antes de construir, confirmar com 15 a 20 PMEs que a dor existe e que elas pagariam. A Alis é o cliente zero.

**Passos**

- [ ] Mapear os apps da Alis: o que fazem, quem fez, frequência de uso, o que acontece se pararem
- [ ] Perguntar aos sócios da Alis: "se alguém saísse amanhã, o que quebraria?" e "pagariam pra isso não acontecer?"
- [ ] Colocar o app local da Alis na nuvem, mesmo manualmente, e anotar tudo o que deu trabalho
- [ ] Listar 20 PMEs conhecidas (amigos, família, ex-colegas) onde alguém já cria apps com IA
- [ ] Fazer 15 entrevistas com o roteiro abaixo
- [ ] Consolidar os resultados e decidir: seguir, ajustar ou descartar

**Roteiro de entrevista** (perguntar sobre o que já aconteceu, não sobre a ideia)

1. Que apps ou ferramentas vocês já criaram com IA? Me mostra um.
2. Onde eles rodam hoje? Quem consegue usar além de quem criou?
3. Conta da última vez que alguém precisou usar um app e não conseguiu. O que fizeram?
4. Já aconteceu de um app parar de funcionar? Quem consertou e quanto tempo levou?
5. Se quem criou saísse amanhã, o que aconteceria com esses apps?
6. Vocês já tentaram resolver isso? Com qual ferramenta, e por que parou?
7. Quanto gastam hoje com ferramentas de IA e nuvem por mês?

**Critérios pra seguir em frente**

- Pelo menos 10 de 15 empresas têm 3 ou mais apps criados com IA em uso.
- Pelo menos 8 relatam um caso real de app inacessível, quebrado ou dependente de uma pessoa.
- Pelo menos 5 aceitam testar um piloto, e 3 topam pagar (ou pré-pagar) por ele.
