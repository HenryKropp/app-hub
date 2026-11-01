# Roadmap de produto do Ziv

Atualizado em 31/10/2026.

## Visão geral

O produto cresce em camadas sobre o mesmo núcleo: o manifesto `ziv.yaml`, que descreve qualquer app, e o Agente, que o executa. Primeiro a base fica sólida; depois vêm segurança, controle de custo, novos tipos de item (skills e agentes), distribuição (marketplace) e, por último, infraestrutura (nuvem).

| Ordem | Frente | Depende de |
| --- | --- | --- |
| 1 | Base sólida (hackathon) | — |
| 2 | Segurança e controle | Base |
| 3 | Painel de consumo de IA | Cofre (as chaves de IA ficam nele) |
| 4 | Skills e agentes entre IAs | Manifesto genérico |
| 5 | Marketplace interno | Base; fica mais rico com skills e agentes |
| 6 | Marketplace entre empresas | Cofre, isolamento e revisão de apps |
| 7 | Nuvem gerenciada | Isolamento forte e painel de custo |

Regra de trabalho: uma frente por vez, com alguém usando de verdade antes de abrir a próxima.

## 1. Agora: base sólida para o hackathon

O fluxo principal precisa funcionar sem ajuda num Windows real: importar, ver a ficha, rodar, receber o resultado.

- [ ] Agente pareando com o endereço de produção (vibehub.replit.app)
- [ ] Importação de scripts validada de ponta a ponta no Windows: diagnóstico, formulário gerado, execução, coleta de arquivos
- [ ] Diagnóstico mais confiável nos 19 apps do piloto (linguagem, entradas, saídas, dependências, senhas no código)
- [ ] Mensagens de erro sempre em português simples e com o próximo passo
- [ ] Telas vazias e estados de carregamento revisados
- [ ] Histórico de execução claro: quem rodou, quando, com quais entradas, qual resultado
- [ ] Primeira versão do painel de consumo de IA, só para apps que rodam pelo Ziv (detalhado na seção 3)

**Pronto quando:** alguém que não programa roda um app sem ajuda e encontra o resultado no histórico.

## 2. Segurança e controle

Pré-requisito para atender outras empresas: senhas saem do código, cada execução fica contida e tudo é auditável.

- [ ] **Cofre de segredos:** chaves de API e senhas guardadas criptografadas e entregues ao app só na hora de rodar, sem aparecer no código nem na tela
- [ ] **Detecção de segredos na importação:** o diagnóstico aponta senhas no código e oferece movê-las para o cofre
- [ ] **Isolamento básico da execução local:** pasta e permissões restritas por execução, limites de tempo e de recursos
- [ ] **Permissões mais finas:** quem pode ver, rodar, editar e publicar cada app
- [ ] **Aprovação de versões:** nova versão de um app só entra em uso depois de aprovada pelo dono
- [ ] **Trilha de auditoria:** registro de quem mudou o quê, exportável para o sócio
- [ ] **Instalador do agente assinado digitalmente,** sem avisos do Windows

**Pronto quando:** nenhum app do piloto tem senha no código e toda execução fica registrada.

## 3. Painel de consumo de IA multi-fornecedor

O sócio vê, num lugar só, quanto a empresa gasta com IA, em quê e onde dá para economizar, somando todos os fornecedores.

**Etapa A: o que roda pelo Ziv**

- [ ] Chamadas de IA dos apps passam pelo Ziv, usando as chaves do cofre
- [ ] Tokens e custo registrados por execução, app, pessoa e fornecedor
- [ ] Painel com totais do mês, ranking de apps e pessoas, e evolução no tempo

**Etapa B: o uso fora do Ziv**

- [ ] Integração com os painéis de administração de cada fornecedor, conforme os dados que eles liberarem
- [ ] Cadastro das assinaturas pagas (quem tem qual plano e quanto custa)
- [ ] Sugestões de economia: assinaturas pouco usadas, planos maiores ou menores que o uso

**Etapa C: controle**

- [ ] Limites de gasto por pessoa, área ou app
- [ ] Alertas quando um app ou pessoa sai do padrão

**Pronto quando:** o painel aponta uma economia real em pelo menos uma empresa.

## 4. Skills e agentes entre IAs

Skills e agentes viram itens do catálogo como os apps, com ficha, dono, versão e permissão, e funcionam em mais de uma IA.

- [ ] Novos tipos no manifesto: skill e agente, além de app
- [ ] Importar skills do Claude (pasta com SKILL.md) com ficha gerada automaticamente
- [ ] Importar instruções de outras IAs (assistentes personalizados do ChatGPT, do Gemini e similares)
- [ ] Instalar com um clique: o Ziv entrega a skill no formato de cada IA, com o passo a passo para quem não sabe fazer
- [ ] Versões e aviso aos usuários quando uma skill muda
- [ ] Indicadores de uso por skill e agente, ligados ao painel de consumo

**Pronto quando:** uma pessoa que nunca usou skills instala uma pelo Ziv e passa a usá-la no dia a dia.

## 5 e 6. Marketplace

Primeiro, o catálogo interno vira uma vitrine fácil de explorar. Depois, empresas passam a publicar e usar ferramentas umas das outras.

**Interno**

- [ ] Busca, categorias e filtros por área
- [ ] Destaque dos itens mais usados e dos novos
- [ ] Pedidos de ferramenta: alguém descreve o que precisa, um criador assume
- [ ] Avaliações e comentários internos

**Entre empresas**

- [ ] Publicação com revisão: checagem automática de segredos e dados internos antes de sair da empresa
- [ ] Modelos de apps por setor (ex.: extratores de dados públicos usados por todas as empresas do ramo)
- [ ] Instalação de um item publicado, com configuração própria de cada empresa
- [ ] Atualizações do autor chegando a quem instalou, com aprovação
- [ ] Regras de licença e, mais adiante, cobrança por item

**Pronto quando:** um item criado numa empresa é instalado e usado em outra.

## 7. Nuvem gerenciada sob demanda

Apps que não dependem do escritório rodam na nuvem, ligando só quando alguém clica em Rodar, com o custo visível.

- [ ] Execução em contêiner isolado por execução, desligado ao terminar
- [ ] Escolha por app: rodar local, na nuvem ou nos dois
- [ ] Agendamento: apps que rodam sozinhos em horários definidos
- [ ] Custo de nuvem por execução e por app, no mesmo painel do consumo de IA
- [ ] Limites de gasto e desligamento automático de apps parados
- [ ] Arquivos de saída guardados por um prazo definido e depois apagados

**Pronto quando:** um app roda na nuvem para alguém fora do escritório, com o custo da execução aparecendo no painel.

## Transversais

Itens que atravessam as frentes e entram quando a frente que os usa estiver pronta.

- **App desktop com instalador único** (decisão de arquitetura 005): substitui a instalação separada do agente e simplifica o primeiro uso.
- **Servidor MCP do Ziv:** permite que Claude e outras IAs consultem o catálogo e rodem apps a pedido do usuário, com as mesmas permissões. Entra junto com a frente de skills e agentes.
- **IA de manutenção:** quando um app falha, o Ziv explica o erro em português simples e sugere a correção ao criador. Entra depois da base sólida, usando o histórico de execuções.
- **Indicadores de produto:** execuções por semana, pessoas ativas, apps sem uso e pedidos de ajuda, acompanhados desde a primeira frente.
