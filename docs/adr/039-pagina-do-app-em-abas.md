# 039 - Página do app em abas

**Status:** Accepted (substitui a ADR 029 na organização da página)

## Context
Mesmo com as faces Usar · Cuidar (ADR 029), a página de cada app ainda
confundia quem cuida dele (Henry, 2026-11-11). No app publicado, "Abrir" e
"No meu computador" apareciam lado a lado sem dizer a diferença. Quem cuidava
caía em Pendências e não via as execuções, e os agendamentos só existiam na face
Usar. "O que pede", "o que entrega" e as execuções apareciam nas duas faces, e a
documentação estava espalhada em três lugares (topo, Ficha e Código). Foram
mostradas cinco opções (abas, painel de cartões, documento com coluna, rodar
primeiro, perguntas do responsável), e Henry escolheu as abas.

## Decision
- Uma página só, sem a troca Usar · Cuidar. Abas: **Início, Execuções,
  Documentação, Configurações, Servidor, Histórico**. Quem só usa o app vê
  Início, Execuções e Documentação (sem a parte técnica).
- **Topo:** nome, resumo e um selo de estado ao lado do nome (Suspenso, Esperando
  aprovação, "N coisas para resolver", ou para o app no servidor: No ar,
  Rascunho, Desligado, Parou com erro). Clicar no selo leva à aba certa. Há um
  botão principal coral: "Abrir" quando o app tem endereço ou tela, "Rodar" para
  script, e "Agendar" ao lado do script.
- "Testar no meu computador" sai do topo e vai para o menu **Mais**, com a
  explicação "roda a cópia da sua pasta, para testar antes de publicar". Ele
  também aparece na aba Servidor. Só aparece para quem cuida do app.
- **Início:** a barra de suspenso ou esperando aprovação, quando houver; "Para
  resolver", cada item com o botão que resolve; as 3 últimas execuções com "Ver
  todas"; os agendamentos; o que pede; o que entrega; quem cuida.
- **Documentação:** tudo numa aba, em ordem: resumo, o que pede, o que entrega,
  quem cuida e, para quem vê a parte técnica, pontos de atenção, outros apps,
  como roda, onde está, código-fonte, bibliotecas e chaves no código. Cada parte
  tem o seu Editar, e no alto aparece o que falta preencher.
- **Configurações:** chaves e senhas, outros ajustes, aprovar ou suspender o
  app, quem pode usar (Acesso) e Arquivar.
- A linha de etapas (registrado → aprovado → publicado → com reserva) sai: o
  selo e o "Para resolver" dizem o mesmo com menos espaço.
- O link `…/apps/N#chaves` continua abrindo direto em Configurações.

## Rationale
Quem cuida de um app quer rodar, ver se deu certo, documentar e configurar. Com
abas, cada uma dessas coisas tem um lugar só, a um clique, e o Início responde
"está tudo bem?" sem clicar. Abas são o padrão que todo mundo já conhece.

## Trade-offs
**Pros:**
- Nada aparece em dois lugares; os agendamentos voltam para quem cuida.
- O estado do app fica à vista no topo.
- Um só botão principal, com o mesmo nome conforme o tipo.

**Cons:**
- O Início repete em leitura o que pede e o que entrega (para editar, é na
  Documentação).
- Seis abas é o máximo; no celular elas rolam para o lado.

## Alternatives Considered
- Painel de cartões: tudo custava um clique a mais e pesava para quem só usa.
- Documento com coluna ao lado: bom para documentar, mas deixava Configurações e
  Servidor como links pequenos.
- Rodar primeiro: escondia a documentação numa gaveta.
- Perguntas do responsável: página longa e ações do dia a dia escondidas.
