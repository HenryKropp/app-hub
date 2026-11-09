# 033 - Execuções visíveis: o que está rodando, há quanto tempo, com o quê e o que saiu

**Status:** Accepted

## Context
Henry teve dificuldade para achar as próprias execuções enquanto elas rodavam:
o que está rodando, há quanto tempo, o que foi preenchido e o que saiu. Duas
causas: quem cuida do app abre a página já na face Cuidar (ADR 029), e a lista
de execuções só existia na face Usar; e os valores preenchidos no formulário de
Rodar iam para o computador (no pedido ao Agente) mas nunca apareciam. Pior: o
Ziv apaga o formulário do pedido quando a execução termina, então nem dava para
recuperar depois.

## Decision
- **Cuidar ganha "Execuções"** no grupo App do menu, para quem pode ver
  execuções do app (a mesma regra da face Usar: quem vê as de todos vê todas,
  os outros só as próprias). Em cima, "Rodando agora", com o tempo correndo
  segundo a segundo; embaixo, as recentes: situação, quando começou, duração,
  quem (só para quem vê as de todos) e quantos arquivos. Cada linha leva à
  página da execução. A face Usar continua com a sua lista.
- **Minhas execuções** abre com o bloco "Rodando agora": cada execução aberta
  da pessoa, com o nome do app, há quanto tempo roda (contando), o computador e
  o botão Parar quando o Agente sabe parar (o mesmo fluxo de confirmação da
  página da execução).
- **Página da execução**:
  - "O que foi preenchido": cada valor com o nome da entrada na ficha, na
    ordem da ficha. Arquivo aparece só pelo nome. Entrada com cara de senha
    (tipo password/secret na ficha, ou nome como senha, token, segredo,
    "chave de API", "chave de acesso") aparece como "escondido". Sem nada:
    "Nenhum valor preenchido". Execução de antes deste registro diz isso.
    Execução de agendamento sem registro (a perdida, por exemplo) mostra o
    formulário do agendamento.
  - Enquanto roda, "rodando há 3 min 12 s", contando.
  - Quando termina e o script deixou o registro do que escreveu na tela
    (`.apphub-console.txt`), "Registro do app" mostra o texto num bloco de
    letra fixa, com rolagem, só o final quando passa de 20 mil caracteres, e o
    botão "Baixar o registro". Acima de 1 MB só baixa. O texto vem do
    computador onde rodou, como os outros arquivos.
- **Início**: enquanto algo roda, uma linha por execução, "Rodando agora:
  Rentabilidade há 3 min 12 s", que leva à execução.
- **Dados**: a tabela `runs` ganha a coluna `inputs` (migração 026). Guarda só
  os valores digitados e os nomes dos arquivos; nunca o conteúdo dos arquivos;
  entrada com cara de senha fica só marcada como escondida, sem o valor. A
  resposta de `GET /api/runs/{id}` ganha `inputs`, `inputs_recorded` e
  `console`; a de `GET /api/runs/active` ganha `started_at` e `computer_name`.

## Rationale
Quem cuida do app é justamente quem mais precisa ver as execuções, e estava
sem elas na face onde chega. O formulário é a metade que faltava para entender
um resultado ("rodou com outubro ou novembro?"). Guardar na própria execução,
e não ler do pedido ao Agente, é o que funciona depois que ela termina, e some
junto com ela na limpeza de registros antigos.

## Trade-offs
**Pros:**
- Em qualquer lugar (Início, Minhas execuções, página do app, página da
  execução) dá para ver o que está rodando e há quanto tempo.
- Cada execução diz com o quê rodou, sem guardar arquivos nem senhas.

**Cons:**
- Antes, o Ziv apagava o formulário quando a execução acabava. Agora os
  valores digitados ficam guardados enquanto a execução existir (até a
  limpeza de registros antigos da empresa). Quem pode ver a execução vê os
  valores, como já via os arquivos que ela criou.
- Esconder por nome é um palpite: "Token" é escondido mesmo que não seja
  segredo, e uma senha com nome estranho passaria. Hoje a ficha não tem tipo
  "senha"; senhas de verdade vão para o Cofre (ADR 027/028).
- O registro do app só aparece com o computador ligado, porque vem dele.
- "Rodando agora" do Início e de Minhas execuções mostra só as execuções da
  própria pessoa (as que ela pode parar); as dos outros aparecem na página do
  app.

## Alternatives Considered
- Ler o formulário do pedido ao Agente: some quando a execução termina, e o
  pedido também carrega o conteúdo dos arquivos.
- Mostrar o registro do app sempre inteiro: um script que escreve muito
  travaria a página; o final é o que costuma explicar um erro.
- Um painel separado de "monitor" de execuções: mais um lugar para procurar;
  preferimos pôr o que está rodando onde a pessoa já olha.
