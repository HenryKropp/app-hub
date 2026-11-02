# Revisão de UX do Ziv — 01/11/2026

Revisão feita no Ziv rodando local (`scripts/run-local.sh`), no código da `main`
mais as correções da branch `rodar-local-windows`, a 1366×768. Telas vistas:
landing (`/boas-vindas`), acesso (`/acesso`), cadastro (`/cadastre-se`), login
da empresa (`/w/<empresa>/entrar`), Catálogo vazio, Começar, Registrar,
Equipe, Segurança, Servidor, Configurações e Minhas execuções. Não vistas com
dados reais: ficha de app, formulário de Rodar, faixa de execução e página da
execução (o catálogo estava vazio).

Régua usada: `PRODUCT.md` (público que não programa, "rodar em poucos
cliques", "erro sempre com o próximo passo", "genérico antes de sob medida") e
`DESIGN.md` ("The Play Button in a Quiet Room").

## Impressão geral

Por dentro, o Ziv segue o próprio sistema de design: neutro, limpo, com o
Começar bem pensado. Por fora, a landing e as telas de entrada são **outro
produto**: serifa, papel creme, petróleo, palavras em coral itálico e rótulos
pequenos acima dos títulos, exatamente o "archive look" que o `DESIGN.md`
proíbe. E o caminho de quem chega é longo e cheio de atrito: cadastra, cai num
login que diz "Bom ter você de volta", digita tudo de novo, e encontra um
catálogo que manda "apontar a pasta", o que este portal não permite.

O maior ganho não é visual: é **encurtar o caminho do cadastro até o primeiro
Rodar**.

## 1. Cadastro e entrada (o ponto fraco)

| Achado | Gravidade | Recomendação |
|---|---|---|
| Depois de criar a empresa, a pessoa **não entra**: vai para uma tela de login, com título "Bom ter você de volta", e precisa digitar e-mail e senha que acabou de criar. Para quem acabou de se cadastrar, isso parece erro ("não foi"). | 🔴 Crítico | Entrar automaticamente após criar e abrir o Começar. Se a entrada automática for uma decisão de segurança ("Sem entrada automática após criar"), ao menos trazer o e-mail preenchido e trocar o título para "Empresa criada. Entre com a senha que você acabou de definir." |
| A 1366×768, a senha e o botão "Criar espaço da empresa" ficam **abaixo da dobra**; metade da tela é texto de marketing repetido da landing. | 🔴 Crítico | Formulário em uma coluna centrada, curto, com o botão visível sem rolar. O texto lateral sai (quem chegou aqui já leu a landing). |
| "Endereço da empresa" (`ziv / ____`) obriga a inventar um identificador técnico, com regras (minúsculas, hífens, 3 a 40). É o campo que mais gera erro. | 🟡 Moderado | Gerar o endereço a partir do nome da empresa ("Alis Investimentos" → `alis-investimentos`), mostrado como texto com "trocar". Só abre o campo se a pessoa quiser. |
| Senha sem "mostrar senha" e sem confirmação; com o envio barrado em silêncio pelo navegador, a pessoa não sabia o que errou (corrigido na branch: agora há mensagem fixa). | 🟡 Moderado | Botão de olho no campo de senha. Mensagem de erro **ao lado do campo** que falhou, não só no rodapé. |
| `/acesso` pede "identificador ou endereço" da empresa. Quem trabalha numa empresa raramente sabe isso. | 🟡 Moderado | Entrada por e-mail: a pessoa digita o e-mail, o Ziv mostra as empresas dela (sem revelar empresas de outros e-mails). Mantém o identificador como alternativa. |
| Três layouts diferentes para entrar: landing creme, acesso/cadastro creme com cartão, login da empresa com painel petróleo dividido. | 🟢 Menor | Um único layout de autenticação, no estilo do app (branco, Geist, cartão com borda fina). |

## 2. Landing page (`/boas-vindas`)

| Achado | Gravidade | Recomendação |
|---|---|---|
| O botão coral principal do topo é **Login**; "Cadastre-se" é um link secundário. Para quem chega pela landing, a ação principal é começar. O header ainda junta "Login / Cadastre-se" num botão só. | 🔴 Crítico | Coral em "Criar espaço da empresa" (ou "Começar grátis", se fizer sentido); "Entrar" como link discreto no header. |
| A promessa não fala do que o Ziv faz de único: **clicar em Rodar e receber os arquivos** de um script que antes só uma pessoa sabia rodar. "Microapps", "casa compartilhada" e "scripts do Windows" são abstratos. | 🔴 Crítico | Título concreto, no idioma do público: "O script que só o fulano sabe rodar vira um botão para a equipe toda." Subtítulo com os três passos: importar, rodar, baixar o resultado. |
| O mockup do hero ("Aurora Studio", "Bom dia, equipe", avatares, ⌘K) **não é o produto**: o Ziv real não tem essa tela, não usa serifa e o público é Windows (⌘ é do Mac). O aviso "exemplo ilustrativo" admite isso. `DESIGN.md`: "Don't ship placeholder, sample or fake content". | 🟡 Moderado | Usar uma captura real do catálogo com o botão Rodar e a faixa de execução (a "assinatura" do produto). Ou um GIF de 10 s: Rodar → arquivos prontos. |
| Visual contra o sistema de design: serifa nos títulos, palavras em coral itálico ("Finalmente juntas.", "grande diferença", "O acesso, não."), rótulos acima de cada título ("UM LUGAR COMPARTILHADO…", "01 — MENOS PROCURA…"), fundo creme e petróleo. Viola The One Voice Rule (coral só como preenchimento), The No Kicker Rule e o "Don't bring back the archive look". | 🟡 Moderado | Decidir: ou a landing segue o `DESIGN.md` (Geist, branco, coral só no botão), ou o `DESIGN.md` ganha uma seção "marketing" que autoriza outro tom. Hoje o documento diz uma coisa e a tela faz outra. |
| A seção "Uma casa para as pequenas ferramentas" tem uma faixa "MICROAPPS + SCRIPTS LOCAIS + UMA EQUIPE +" que parece clicável e não faz nada. | 🟢 Menor | Remover ou transformar em três exemplos reais de uso. |
| Sem prova: nenhum caso real, nenhum número. `PRODUCT.md` proíbe inventar, mas o piloto existe. | 🟢 Menor | Um caso real da Alis, com permissão: "19 scripts que viviam numa pasta viraram botões para o time de research." |

## 3. Primeiro uso, já dentro da empresa

| Achado | Gravidade | Recomendação |
|---|---|---|
| Catálogo vazio diz "**Aponte a pasta** onde ficam os apps", mas neste portal a leitura de pastas é bloqueada; a própria tela Registrar termina com "Análise de pastas do computador indisponível". | 🔴 Crítico | Trocar o vazio por "Envie o primeiro script: escolha a pasta dele no seu computador e o Ziv prepara a ficha." Tirar o bloco "Análise indisponível" quando o caminho não existe neste portal. |
| As áreas de uma empresa nova são as da **Alis** (Compliance, Investimentos, Research, Relações com Investidores…), criadas pela migração `007_access.sql`. Fere "Genérico antes de sob medida" e aparece para qualquer cliente. | 🔴 Crítico | Empresa nova começa sem áreas (ou com "Geral"); a Alis cria as dela. Mudança de banco: migração nova só para empresas novas, sem apagar as áreas da Alis. |
| Registrar um script exige escolher linguagem, arquivo de entrada, arquivo de dependências e criar as entradas à mão. O diagnóstico que já existe (`scanner`) não roda na importação. | 🟡 Moderado | Pedaço B do roadmap: ao escolher a pasta, o Ziv preenche linguagem, entrada, dependências e campos; a pessoa só revisa. |
| Instruções do Agente com termos técnicos: "apphub-url.txt", "não mova o EXE sozinho", "ZivAgente.exe" (o arquivo local se chama `AppHubAgente.exe`). Aparecem duas vezes na mesma tela. | 🟡 Moderado | Um instalador único (ADR 005) resolve de vez. Até lá: um passo a passo com 3 linhas, sem nomes de arquivo, e o nome do EXE igual ao do ZIP. |
| Menu lateral para uma empresa recém-criada: Segurança, Servidor (que só mostra "desativada neste portal"), e "Em breve: Painel, Cofre". | 🟡 Moderado | Esconder Servidor quando desativado; tirar os "Em breve" do menu (vão para o roadmap). Menos itens = o Começar e o Catálogo ficam evidentes. |
| Segurança é escrita para engenheiro: "keyring", "CREDENTIAL_VAULT_KEYRING", "Endpoint HTTPS", "Token bearer", "HEAD", "jobs", "payloads", "sandbox". O sócio é o público dessa tela. | 🟡 Moderado | Duas camadas: em cima, 3 frases para o sócio (o que está protegido, o que não está, o que fazer); embaixo, recolhido, "Detalhes técnicos". |
| Equipe pede "Senha inicial" e diz "passe para a pessoa pessoalmente", enquanto existe convite por link (`/convite`). | 🟢 Menor | Convite por e-mail/link como caminho principal; senha inicial só como alternativa. |

## 4. Consistência

| Elemento | Problema | Recomendação |
|---|---|---|
| Tipografia | Landing e entrada em serifa; app em Geist. | Geist em tudo (ou exceção documentada no `DESIGN.md`). |
| Cor | Coral em texto e itálico na landing/cadastro; petróleo como fundo de painel (o `DESIGN.md` reserva o petróleo para o ícone). | Coral só como preenchimento de ação; petróleo só no logo. |
| Rótulos acima do título | Em toda a landing, no cadastro, no acesso e no login. | Remover (No Kicker Rule). |
| Nome | "Hub", "App Hub" e "apphub" ainda aparecem em textos visíveis ("o Hub não pode acessar pastas", "apphub-url.txt"). | Trocar por "Ziv" no texto que a pessoa lê. |
| Validação de formulário | Corrigida no cadastro; a mesma regra de endereço inválida para o Chrome (`pattern="[a-z0-9-]+"`) ainda existe em Segurança, sem efeito prático. | Mesmo padrão do cadastro: a página valida e explica. |

## 5. Acessibilidade (o que deu para verificar)

- Texto auxiliar muito pequeno e cinza claro nos cartões de entrada (dicas de campo, "Ex.: horizonte ou…", rodapés): provável contraste abaixo de 4.5:1 sobre o creme. Medir com o `scripts/contrast-check.mjs`.
- O link "Pular para o conteúdo" existe nas telas internas. Bom.
- O balão nativo do navegador some antes de ser lido; a correção do cadastro (mensagem fixa com `role="alert"`) também ajuda leitor de tela. Levar o mesmo padrão ao login e ao convite.

## O que funciona bem

- **Começar** (5 etapas, "o progresso só muda quando o Ziv confirma") é o melhor fluxo do produto: claro, honesto e no ritmo da pessoa.
- Interior fiel ao `DESIGN.md`: neutro, uma cor só, títulos diretos ("Apps da empresa", "O que você rodou").
- Estados vazios com próximo passo em Minhas execuções ("No catálogo, clique em Rodar…").
- Honestidade sobre limites (Segurança diz o que não está protegido). Falta só falar a língua do sócio.

## Prioridades

1. **Cadastro → dentro do Ziv sem redigitar nada.** Formulário curto e visível sem rolar, endereço gerado do nome, entrada automática (ou login preenchido com o título certo). É o primeiro contato e hoje parece quebrado.
2. **Catálogo vazio e áreas genéricas.** Texto do vazio que leva ao envio do script; empresa nova sem as áreas da Alis. Sem isso, todo cliente novo bate em um beco e vê dados de outra empresa.
3. **Landing com uma promessa concreta e a ação certa em coral.** "Criar espaço" como botão principal; mostrar o produto real (Rodar → arquivos); alinhar o visual ao `DESIGN.md` ou documentar a exceção.
4. Depois: diagnóstico automático na importação (pedaço B), Segurança em duas camadas, menu enxuto.
