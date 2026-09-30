# PRD — App Hub (Módulo 1: Gestão e orquestração de apps)

**Status:** Em construção (Fundação e Registro prontos) · **Atualizado:** 2026-09-29
**Fonte:** `Documentação/Especificação do produto.md` (visão completa, Módulos 1 e 2)

---

## 1. Visão geral

O App Hub é uma plataforma onde pequenas empresas sem TI registram, organizam, rodam e mantêm os apps que criam com IA. Ele transforma "um script na pasta de alguém" em "uma ferramenta da empresa", com dono, documentação, tela de uso e histórico.

**Objetivo do Módulo 1:** qualquer pessoa da empresa encontra, entende e usa qualquer app, sem saber programar e sem depender de quem criou.

**Princípio central: o Hub é genérico.** Ele não é feito sob medida para os apps da Alis. Qualquer app, de qualquer tipo, entra pelo mesmo caminho: é registrado, ganha um **manifesto** (a "carteira de identidade" do app, um arquivo que descreve como ele roda, o que recebe e o que devolve) e, a partir dele, ganha ficha, tela de uso e histórico. Os apps da Alis são o primeiro caso real e servem para testar essa generalidade. Novos apps, criados à mão ou pela IA via MCP (V2), entram pelo mesmo manifesto.

### Piloto

- **Onde roda:** no computador do Henry (Windows), localmente, sem nuvem.
- **Apps de teste:** a pasta `C:\Users\HenryKroppAlisInvest\OneDrive - Alis\ALIS\5. Operacional\8. Programação`.
- **IA (Claude API):** fica para uma fase seguinte. O piloto começa com detecção por regras e confirmação humana, e as funções de IA entram quando a chave for configurada.

### Variedade real encontrada na pasta de teste

A pasta confirma que o Hub precisa ser genérico. Hoje ela tem:

| Padrão | Exemplos | Como o app recebe a entrada |
|---|---|---|
| Script com arquivo fixo na pasta | Rentabilidade fundos (`cnpjs.csv`), Carteira detalhada (`Input fundos.xlsx`) | Lê um arquivo com nome fixo ao lado do código |
| Script com janela de seleção | Extrair Seções do FRE, PDF to MD | Abre uma janela do Windows pedindo o arquivo ou a pasta |
| App com tela própria (web) | Sistema Alis (Streamlit), Research Platform, News Scrapper (servidor) | Abre no navegador |
| App de desktop | Análise FIAs, Transcritor de calls | Abre uma janela própria |
| Tarefa recorrente | News Scrapper (agendado no Windows) | Roda sozinho no horário |
| Executável pronto | CVM Downloader, Ferramenta CVM (`.exe`) | Duplo clique |
| Sistema complexo | Sistema-Alis (vários serviços) | Só registro e ficha |
| Documento, não app | Claude Project _ Edu, Expandir Projeção (`.docx`) | Não roda, só fica catalogado |

---

## 2. Usuários

| Perfil | Quem é | O que quer |
|---|---|---|
| **Criador** | Quem faz apps com IA (no piloto, o Henry) | Tirar o app da pasta, deixá-lo usável por todos e saber quando quebra |
| **Usuário** | Colega que não programa | Achar o app certo, entender e clicar em Rodar |
| **Sócio / Administrador** | Quem responde pela empresa | Ver o que existe, o que é usado e onde está o risco |

---

## 3. Funcionalidades (escopo do piloto)

A numeração segue a especificação do produto.

### 3.1 Registro e publicação (1.1)
- **Registro em lote:** o criador aponta uma pasta, o Hub varre as subpastas e sugere um app por subpasta.
- **Registro individual:** o criador aponta uma pasta de um app só.
- **Diagnóstico automático:** para cada app, o Hub identifica linguagem, tipo de app, ponto de entrada (o arquivo que inicia o app), bibliotecas, arquivos de entrada e saída prováveis e chaves/senhas no código.
- **Confirmação do incerto:** o Hub mostra o diagnóstico em português e pergunta só o que ficou em dúvida ("este arquivo é uma entrada?").
- **Manifesto salvo ao lado do app:** o arquivo de configuração fica dentro da pasta do app, legível mesmo fora do Hub.
- **Detecção de segredos:** se achar senha ou chave escrita no código, avisa e oferece mover para o cofre (3.7).
- **Nada é perdido:** o estado original do app é guardado como versão 1 (3.6).
- **Aviso de duplicado:** ao registrar, avisa se já existe app parecido (por nome e descrição), no catálogo ou no mesmo lote.
- **Pastas de arquivo:** pastas com nome de material antigo (ex.: "Old", "backup") vêm marcadas para pular.
- Só publica com responsável definido (3.9).

### 3.2 Ficha do app (1.2)
- Página por app com: nome e resumo, como usar, entradas, saídas, exemplo (última execução bem-sucedida), responsável e reserva, dependências, status e uso, versão.
- Campos editáveis à mão. No piloto sem IA, o texto nasce de um modelo preenchido com o diagnóstico; com IA, é escrito em português simples.
- **Exportação:** a ficha também é salva como arquivo de texto na pasta do app.

### 3.3 Execução com interface automática (1.3), a feature central
- Todo app que pode rodar ganha um botão **Rodar**.
- **Scripts:** o Hub gera um formulário a partir das entradas do manifesto (upload de arquivo, data, número, texto, lista de opções). O usuário preenche e roda.
- **Apps com tela própria (web):** o Hub sobe o app e abre a tela dele.
- **Apps de desktop e executáveis:** o Hub abre o app.
- **Ambiente preparado sozinho:** na primeira vez, o Hub instala a versão certa da linguagem e das bibliotecas. Nas seguintes, reaproveita o que já instalou. O usuário não vê terminal.
- **Progresso e resultado:** mostra o andamento e, ao final, o texto impresso, as tabelas e os arquivos gerados para baixar.
- **Registro de cada execução:** quem rodou, quando, com quais entradas, duração, resultado e versão usada.
- **Repetir execução:** rodar de novo com as mesmas entradas.
- Apps que não rodam pelo Hub (documentos, planilhas com macro, sistemas complexos) aparecem só com ficha.

### 3.4 Catálogo e descoberta (1.4)
- Tela inicial com os apps em cartões: nome, resumo, responsável, status e último uso.
- Busca simples por nome, resumo e etiquetas.
- Filtro por área (ex.: Investimentos, Operações, RI) e por status.
- Favoritos por pessoa.

### 3.5 Padrões (1.5)
- Formato padrão de manifesto e de estrutura de pasta, documentado e aplicado a todos os apps.
- Na ficha, o Hub aponta o que foge do padrão (ex.: "a chave está escrita no código", "a entrada depende de uma janela de seleção").

### 3.6 Versões e histórico (1.6)
- Toda alteração publicada gera uma versão, com quem mudou, quando e um resumo.
- O Hub detecta quando o código da pasta mudou e oferece registrar a nova versão.
- Botão para voltar a uma versão anterior. A restauração também vira uma versão.
- Histórico de execuções ligado à versão usada.

### 3.7 Acesso, permissões e cofre (1.7)
- Cadastro de pessoas da empresa. Cada app tem quem pode usar e quem pode editar.
- **Cofre local:** chaves e senhas guardadas criptografadas. O app recebe a chave na hora de rodar e o usuário nunca a vê.
- Registro de qual app usou qual chave e quando.

### 3.8 Saúde (1.8)
- Toda execução com erro fica registrada com a mensagem de erro.
- Status de cada app: funcionando, com erro ou sem uso.
- **Com IA (fase seguinte):** explicação do erro em português.

### 3.9 Continuidade e responsáveis (1.9)
- Todo app tem responsável e reserva.
- Lista de **apps em risco**: sem reserva, com dono único, sem uso há muito tempo ou com erro recente.

### 3.10 Painel de governança (1.10)
- Uma tela com inventário (apps por área e tipo), uso (mais usados, execuções por semana) e apps em risco.

### 3.11 Registro pelo Claude via MCP (1.11, antecipado)
Pedido do Henry em 2026-09-30: registrar apps de dentro do Claude, sem preencher nada à mão.
- O criador conecta o Claude que já usa ao Hub, com um **código de acesso pessoal** gerado na tela Pessoas.
- O criador pede, por exemplo, "registra todos os apps da pasta de programação". O Claude:
  1. lista as pastas e o que já está no catálogo;
  2. lê o código de cada app, com **chaves e senhas mascaradas** pelo Hub (o Claude nunca vê os valores);
  3. parte do diagnóstico por regras e completa com o que entendeu do código: resumo em português, entradas, saídas, arquivo de início, área e pontos de atenção;
  4. registra o app com responsável e reserva, usando o formato padrão do manifesto.
- **Confirmação humana:** cada registro passa pela confirmação do próprio Claude ("permitir registrar?"), e o app fica marcado no Hub como **registrado via IA**.
- A IA age com as permissões de quem gerou o código de acesso, nunca mais que isso.
- **Não precisa de chave da Claude API:** quem lê e escreve é o Claude do criador.

---

## 4. Fluxos principais

**A. Registrar os apps existentes (criador)**
1. Abre o Hub e clica em "Registrar apps".
2. Escolhe a pasta de programação.
3. O Hub mostra a lista de apps encontrados, cada um com o diagnóstico.
4. Para cada app, confirma o que ficou incerto, define área, responsável e reserva.
5. Publica. Os apps aparecem no catálogo.

**B. Usar o app de um colega (usuário)**
1. Abre o Hub, busca "rentabilidade" e encontra o app.
2. Lê a ficha e clica em Rodar.
3. Sobe o arquivo de CNPJs no formulário e clica em Rodar.
4. Acompanha o progresso e baixa a planilha gerada.

**C. Atualizar um app (criador)**
1. Altera o código na pasta, com a IA que já usa.
2. O Hub percebe a mudança e pede para registrar a nova versão, com um resumo.
3. Se algo der errado, volta para a versão anterior com um clique.

**E. Registrar pelo Claude (criador)**
1. Na tela Pessoas, gera o código de acesso e conecta o Claude (uma vez só).
2. No Claude, pede: "registra os apps da pasta de programação".
3. O Claude lê cada app, mostra o que entendeu e registra, pedindo confirmação a cada um.
4. Os apps aparecem no catálogo com a marca "via IA", e o criador pode revisar no Hub.

**D. Ver a saúde da empresa (sócio)**
1. Abre o painel.
2. Vê quantos apps existem, quais são usados, quais estão com erro e quais estão em risco.

---

## 5. Requisitos não funcionais

- **Local:** tudo roda no computador do Henry, sem depender de internet (exceto quando o próprio app precisa).
- **Não destrutivo:** o Hub nunca apaga nem altera o código de um app sem confirmação. Ele só adiciona o manifesto e a ficha à pasta.
- **Execução isolada da pasta original:** rodar um app não pode sujar nem sobrescrever a pasta original. Arquivos gerados ficam guardados no histórico da execução.
- **Português simples** em toda a interface, sem termos técnicos sem explicação.
- **Manifesto aberto e legível:** o app continua compreensível fora do Hub.
- **Segredos nunca visíveis** na interface, nos registros ou no manifesto.
- **Preparado para o Módulo 2:** a forma de rodar os apps fica separada do resto, para depois levar a execução para a nuvem sem refazer o Hub.
- **Operável pela IA (MCP):** tudo o que a interface faz (registrar, ler ficha, rodar, versionar) fica disponível como operação que a IA também pode usar (3.11).

---

## 6. Fora do escopo do piloto

- **Módulo 2 inteiro** (nuvem, acesso remoto, cobrança).
- **MCP além do registro:** criar apps do zero, rodar, ler saúde e usar o cofre pelo MCP ficam para depois das Fases 4–7. O registro pelo MCP (3.11) entra já.
- **Funções de IA com chave própria do Hub:** diagnóstico por IA dentro do Hub, ficha escrita por IA dentro do Hub, explicação de erros, "pergunte ao app", busca em linguagem natural e resumo das mudanças. Ficam para quando a chave da API estiver configurada.
- **Correção automática de apps (1.8):** V2.
- Login com Google ou Microsoft, grupos de acesso, chaves pessoais por usuário.
- Encadeamento de apps, presets salvos e agendamento pelo Hub.
- Desenvolvimento compartilhado com resolução de conflitos.
- Trilha de auditoria exportável, resumo semanal por e-mail, tempo economizado.
- Execução de planilhas com macro e de sistemas complexos (só registro e ficha).
