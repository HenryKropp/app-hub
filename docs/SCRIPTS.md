# Scripts e configuração inicial

## Configuração inicial

Entre no Hub e abra **Começar** (`/comecar`). O assistente consulta dados reais:

1. Nome da empresa — salvo por quem tem permissão de administrar cargos.
2. Computador conectado — conexão e disponibilidade do agente.
3. Primeiro app — app registrado no catálogo.
4. Primeira execução bem-sucedida — execução sua com resultado.
5. Equipe — mais de uma pessoa cadastrada.

As etapas não são marcadas manualmente. Use **Atualizar progresso** depois de
concluir uma ação em outra tela. O assistente não bloqueia usuários existentes.

## Importar e executar um script

Em **Registrar apps**, use a importação de script e envie os arquivos do app,
incluindo o arquivo principal. Escolha uma área, descreva o app e configure as
entradas. O responsável inicial é a pessoa que importou. Só quem pode registrar
apps pode importar código.

São aceitos Python (`.py`) e JavaScript (`.js`, `.mjs`, `.cjs`). Os arquivos do
pacote somam no máximo 10 MB, com até 100 arquivos. Arquivos de credenciais como
`.env`, `.pem` e `.key` são recusados. Não inclua senhas no código.

Antes de salvar qualquer pacote, o servidor reutiliza a detecção do registro por
pasta em todos os arquivos enviados. Se encontrar possíveis senhas ou chaves,
interrompe a importação sem gravar arquivos ou cadastrar o app. O aviso mostra
somente a quantidade de ocorrências, nunca os valores ou trechos de código.
Revise e remova as credenciais nos arquivos originais, depois selecione-os
novamente. Como a detecção é heurística, pode haver falsos positivos e segredos
não reconhecidos; referências a variáveis de ambiente e exemplos reconhecidos
como placeholders não geram alerta por atribuição.

Para aceitar um falso positivo ou assumir o risco, marque a confirmação
explícita e clique novamente em **Registrar script**. Alterar a seleção ou
cancelar descarta essa confirmação. O servidor reanalisa o pacote em cada envio.
O código confirmado é salvo e entregue ao agente **sem alterações**: não há
mascaramento do código executável nem cofre de credenciais. A confirmação não
libera arquivos de credenciais bloqueados, caminhos inválidos ou outros limites.

O código importado fica no PostgreSQL e é entregue ao agente autenticado na
execução. Não depende de uma pasta Windows acessível pelo servidor Replit.
Uma cópia temporária no servidor permite aproveitar o histórico de versões.

Instale o agente atualizado em **Configurações > Meu computador**. Python 3 ou
Node.js precisam estar instalados no Windows e disponíveis para o agente; após
instalá-los, reinicie o agente. Agentes antigos são impedidos de executar scripts.

Ao clicar em **Rodar**, o Hub abre o formulário mesmo quando a pasta de saída já
foi lembrada. Preencha os campos, envie os arquivos e escolha onde salvar.
Os arquivos enviados no formulário também podem somar até 10 MB.

## Contrato das entradas

O manifesto continua sendo a fonte de verdade:

- `argument`: argumento de linha de comando. O padrão é `--id=valor`; o campo
  opcional `flag` permite definir, por exemplo, `--arquivo`. Várias escolhas
  repetem a opção. O processo é iniciado diretamente, sem shell.
- `prompt`: uma resposta por linha na entrada padrão, na ordem do manifesto.
- `fixed_file`: arquivo enviado é gravado no caminho relativo `path` da cópia de
  trabalho. Não aceita destinos de código nem o arquivo principal.
- `dialog`: valor disponível nas variáveis de ambiente abaixo. Um script que só
  lê uma janela de seleção própria precisa ser adaptado para consumir o valor.
- `window`: o próprio app pede a informação. Não aparece no formulário do Hub.

Toda entrada preenchida também fica disponível em:

- `APP_HUB_INPUT_<ID>`: valor simples; hífens no ID viram sublinhados e letras
  viram maiúsculas.
- `APP_HUB_INPUT_<ID>_JSON`: lista JSON, preservando múltiplas escolhas.
- Arquivos enviados são representados pelo caminho da cópia local preparada.
- `APP_HUB_OUTPUT_DIR`: pasta de trabalho da execução, para gerar os resultados.

IDs devem começar por uma letra e conter apenas letras, números, hífen e
sublinhado, com até 64 caracteres. Número, data, opções, obrigatoriedade, campos
desconhecidos, tamanho e caminhos são validados novamente no servidor e no agente.

## Dependências e resultados

Para Python, selecione um arquivo de dependências, como `requirements.txt`. O
agente usa o pip em uma pasta temporária de bibliotecas, sem alterar a instalação
global do Python. Recomenda-se fixar versões. A preparação tem limite de 5 minutos.

Para JavaScript, envie `package.json` e, preferencialmente, `package-lock.json`.
O agente prepara as dependências com npm, sem scripts de instalação de pacotes.
Bibliotecas que dependem desses scripts podem não funcionar e devem ser adaptadas
pelo criador. Sem dependências externas, basta enviar o próprio script.

Cada execução tem uma cópia de trabalho temporária, removida no final. Novos
arquivos e arquivos alterados são copiados para `Execucao-<id>` dentro da pasta
de saída escolhida, sem substituir resultados anteriores. A saída do console é
entregue como `.apphub-console.txt`, limitada a 64 KB.

Uma saída diferente de zero é falha mesmo quando há arquivos produzidos.
O processo principal tem limite de 15 minutos. Os arquivos continuam acessíveis
pelo fluxo de downloads do agente. Formulários e pacotes de código são removidos
do pedido ao agente quando ele é concluído; o pacote fonte original permanece
associado ao app para execuções futuras.

**A pasta temporária não é uma sandbox de segurança.** Scripts e bibliotecas
executam com as permissões da pessoa no Windows e podem acessar outros arquivos,
rede e processos. Importe apenas código revisado e de confiança. A instalação de
dependências Python também pode executar código dos pacotes.

## Limites da verificação

Há testes automatizados de validação, importação, autorização, progresso,
transporte de arquivos, execução real de Python e JavaScript em Linux e retorno
de falha. O agente é compilado para Windows, mas a execução em um computador
Windows real e os instaladores de dependências com rede exigem validação nesse
ambiente. Não há bypass de login para mostrar telas protegidas.
