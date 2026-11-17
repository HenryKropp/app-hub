# Links externos de artifacts (Projeto 1) — desenho

Data: 2026-11-17. Decidido com o Henry nesta conversa.

## Por quê

Uma empresa cliente do Ziv quer mandar um artifact específico para um cliente *dela*, que não tem login no Ziv. Hoje um artifact só abre para quem entra no Ziv (ADR 018). Segurança é a proposta de valor do Ziv, então mandar algo para fora tem de ser controlado e registrado: o dono da empresa precisa poder dizer "eu sei tudo o que saiu daqui".

Este é o Projeto 1 de 2. O Projeto 2 (depois) faz os outputs dos apps chegarem ao Ziv sozinhos, com link para "um output específico" ou "sempre o output mais recente". O Projeto 1 deve deixar o link pronto para ser reaproveitado ali.

## Objetivos

- Qualquer artifact **de arquivo HTML** pode virar um link externo que abre sem login no Ziv.
- Quem cria o link escolhe o tipo:
  - **Código por e-mail** (padrão): informa o e-mail do destinatário. Ao abrir, a pessoa pede um código de 6 dígitos, recebe no e-mail e digita. Sem conta e sem senha. Só aquele e-mail abre.
  - **Link aberto**: qualquer pessoa com o link abre na hora.
- Quem cria escolhe o conteúdo:
  - **Esta versão** (congelada), ou
  - **Sempre a mais recente** (muda sozinho quando sai versão nova do artifact).
- Todo link tem **prazo** (escolhido ao criar) e pode ser **cancelado** a qualquer momento. Cancelado ou vencido, o link para de abrir na hora.
- **Registro de quem viu:** cada abertura fica guardada (quando; e-mail, no link com código).
- **Permissão de cargo nova: "Compartilhar fora da empresa".** Ligada só para administrador em empresa nova e nas existentes; o admin liga para outros cargos. Só quem tem a permissão **e** pode ver o artifact cria link para ele.
- **Lista no Admin:** todos os links da empresa, com quem criou, artifact, tipo, destinatário, prazo, quantas vezes abriu e a última abertura, e botão Cancelar.
- Na página do artifact: uma área "Links externos" para criar, ver e cancelar os links daquele artifact.

## O que o cliente final vê

Endereço próprio, separado do site da equipe: `https://link.<domínio>/<código>` (ex.: `link.zivapp.duckdns.org/k3J9…`).

```
┌──────────────────────────────────────────────┐
│ [logo Ziv]  ← leva à página inicial do Ziv   │
│                                              │
│ Empresa Piloto compartilhou com você     │
│ Relatório mensal de outubro                  │
├──────────────────────────────────────────────┤
│                                              │
│        (o artifact, isolado, tela larga)     │
│                                              │
└──────────────────────────────────────────────┘
```

- Logo do Ziv no topo, que leva à página inicial pública do Ziv (a landing). Pedido do Henry.
- No link com código, antes do conteúdo: "Este conteúdo foi enviado para h***@cliente.com. Enviar código" → campo de 6 dígitos → conteúdo. Depois de confirmar, o navegador fica liberado para aquele link por um tempo (proposta: 30 dias, ou até o link vencer/ser cancelado).
- Link vencido, cancelado ou inexistente: uma página única e neutra, "Este link não está mais disponível", sem dizer qual dos três aconteceu.

## Restrições

- Mesmo isolamento do ADR 018 para o HTML: servido com `Content-Security-Policy: sandbox …` sem `allow-same-origin`, dentro de iframe com sandbox.
- O endereço `link.` é um subdomínio próprio: nenhum cookie da equipe chega lá, e um HTML malicioso não consegue imitar a tela de login do Ziv no endereço do Ziv. O Caddy já emite certificados para subdomínios sob demanda.
- `link` vira nome reservado (empresa e app não podem usá-lo).
- O código do link é longo e aleatório (não dá para adivinhar nem listar).
- O Ziv hoje não manda e-mail. Entra um serviço de envio: **Resend** (grátis até 3 mil/mês). Chave em variável de ambiente, fora do git.
- **Sem domínio próprio por ora (decisão do Henry, 2026-11-17).** O Resend só entrega para fora com um domínio verificado, e o DuckDNS não aceita os registros de verificação. Então o link com código é **construído inteiro, mas fica desligado no ar** até haver domínio e chave do Resend: sem a chave, a opção "Código por e-mail" aparece desabilitada com o aviso "Disponível em breve", e o padrão ao criar passa a ser "Link aberto". Com `DEV_MODE`, o código aparece no log para testar localmente. Ligar depois é só configurar a chave e o remetente.
- Limites de tentativa: pedidos de código por link e por endereço de rede; tentativas erradas por código. O código vence em poucos minutos.
- Isolamento por empresa como no resto do Ziv: o link pertence a uma empresa e só mostra artifact dela.

## Não objetivos

- Outputs de apps (Projeto 2).
- Artifacts do tipo **link** (`https://` para outro site): não ganham link externo. A pessoa já pode mandar o próprio endereço.
- Domínio próprio da empresa cliente, cores ou logo da empresa na página.
- Aviso para quem criou quando o cliente abrir (fica como ideia).
- Comentários ou respostas do cliente final.

## Anti-objetivos (falharia mesmo funcionando)

- Um link que continua abrindo depois de cancelado ou vencido.
- Conteúdo que sai da empresa sem ficar registrado quem criou e quem abriu.
- O cliente final precisar criar conta ou senha.
- Um artifact conseguir ler a sessão de alguém ou se passar pela tela do Ziv.

## Abordagens consideradas

1. **Endereço próprio para links (`link.<domínio>`)** — escolhida. Separa totalmente quem é de fora do site onde a equipe faz login. Custo extra pequeno porque o servidor já atende subdomínios.
2. **Dentro do site do Ziv (`/l/<código>`)** — um pouco mais rápida, mas expõe a gente de fora o risco já anotado no ADR 018 (HTML imitando a tela do Ziv). Descartada.
3. **Google Drive ou compartilhamento do claude.ai** — o Ziv perde o controle, o cancelamento e o registro de quem viu. Descartada.

## Decisões do Henry

- Os dois tipos de link, com código por e-mail como padrão.
- Ordem: artifacts primeiro, outputs de apps no Projeto 2.
- Permissão de cargo "Compartilhar fora da empresa" + lista no Admin.
- Endereço próprio para links, com o logo do Ziv levando à página inicial.

## Propostas minhas (podem ser corrigidas)

- Prazo ao criar: 7 dias, 30 dias (padrão), 90 dias ou 1 ano. **Sem opção "sem prazo"**: um link esquecido que nunca fecha vai contra a proposta de segurança; quem precisar renova.
- **Quem criou perde o acesso** (sai da empresa, perde a permissão "Compartilhar fora" ou deixa de poder ver o artifact): o link para de abrir. Conferido a cada abertura, não em lote.
- **Quem vê e cancela:** na página do artifact, quem tem "Compartilhar fora" e pode ver o artifact vê os links dele e pode cancelar qualquer um. A lista no Admin é para quem tem a aba Admin.
- **O que conta como abertura:** cada vez que o conteúdo carrega. Guarda-se a data e hora e, no link com código, o e-mail. Não se guardam IP nem navegador (menos dado pessoal, LGPD).
- "Sempre a mais recente" como padrão do conteúdo, porque é o que se espera de um relatório que muda; "esta versão" para quem quer congelar.
- Liberação do navegador por 30 dias após o código.
- Link com código: um e-mail por link (para mandar a duas pessoas, criam-se dois links, e cada um tem seu registro).
- Arquivar ou apagar o artifact cancela os links dele.

## Fica para quem constrói

- Formato das tabelas, rotas e telas, seguindo o padrão do Ziv.
- Texto exato do e-mail e das telas (português, tom do Ziv).
- Desenho do link para o Projeto 2 reaproveitar (o alvo do link não deve ser preso só a artifact).
