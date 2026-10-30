# Cofre de credenciais por empresa

## Escopo e limites

O portal tem um cofre independente do armazenamento de configuração dos apps
hospedados. Nesta versão, aceita tokens Bearer de uma linha (até 8192 bytes),
vinculados a **empresa + app + ambiente + cargo**. Desenvolvimento,
homologação e produção são vínculos distintos, nunca um fallback entre eles.
Administradores com `manage_roles` cadastram, substituem, revogam e consultam
metadados em **Segurança → Cofre de credenciais**.

Não existe API de leitura, revelação, cópia ou exportação de valor, nem mesmo
para administradores. A listagem não contém o destino nem o envelope.
O valor só entra pela requisição de gravação e é cifrado antes do SQL;
não é incluído em manifestos, fontes, logs, auditoria ou `agent_jobs.payload`.
Não coloque o token em campos de texto, nome de app, destino ou código.

O intermediário disponível faz uma requisição **HEAD** ao destino HTTPS fixo,
com o token no cabeçalho Authorization. Só devolve o código HTTP. Exige sessão
da empresa, visibilidade do app, cargo autorizado, ambiente exato e a política
de execução/aprovação vigente. Um administrador não ignora o vínculo de cargo
para usar a credencial. Um HTTP 401/403 do destino é um resultado do teste,
não uma confirmação de token válido.

**Por que não encaminhar a resposta?** Um provedor pode repetir o token no corpo,
nos cabeçalhos ou em mensagens de erro. Nenhum desses dados é retornado.
O destino não pode conter usuário/senha, consulta, fragmento, IP literal ou
porta diferente de 443. O intermediário não segue redirecionamentos, ignora
proxies do ambiente e verifica/pina o IP público resolvido a cada conexão.
Isso protege este intermediário; não é um firewall para scripts Windows.

**Não há injeção em variáveis/argumentos dos scripts**, tokens de delegação para
agentes, conectores de banco, OAuth ou encaminhamento genérico de requisições.
Não contorne isso colocando valores em entradas de execução. Integrações que
precisem executar operações de negócio devem ganhar adaptadores específicos
no servidor e definir saídas que não possam repetir a credencial. O cofre não
torna código externo confiável: execução hospedada e marketplace permanecem
desabilitados; não é uma certificação de compliance.

## Chave mestra e envelopes

Configure exclusivamente nos **Secrets do workspace**:

```text
CREDENTIAL_VAULT_KEYRING
```

Formato do valor (os placeholders não são chaves utilizáveis):

```json
{
  "active": "key-2026-b",
  "keys": {
    "key-2026-a": "<base64 de 32 bytes aleatórios independentes>",
    "key-2026-b": "<base64 de outros 32 bytes aleatórios>"
  }
}
```

Use um gerador criptográfico e um gerenciador seguro para produzir/guardar
as chaves. Não as escreva no chat, terminal com coleta de logs, `.env`,
manifesto, repositório ou banco de dados. Não reutilize `SESSION_SECRET` nem
`HUB_SECRET_KEY`. A recuperação segura das chaves é responsabilidade operacional:
sem a chave correspondente o ciphertext e seus backups são irrecuperáveis.

Sem esse Secret, o servidor continua funcionando com o cofre **bloqueado**:
metadados e revogação permanecem disponíveis. Uma configuração presente mas
inválida impede a inicialização, sem fallback automático. Alterações no Secret
exigem reiniciar o servidor.

Cada gravação gera uma chave de dados aleatória AES-256-GCM. Ela é envelopada
por AES-256-GCM com a chave mestra ativa. O envelope inclui versão de formato
e identificador da chave mestra, além de nonces independentes.
A empresa, o ID da credencial, app, ambiente, cargo, destino e revisão são
autenticados: copiar ciphertext entre empresas/objetos ou modificar os vínculos
não produz uma credencial utilizável.

### Rotação da chave mestra

1. Acrescente a nova chave independente ao Secret, conservando as antigas e
   selecionando a nova em `active`. Reinicie o servidor.
2. Em **cada empresa**, use **Reproteger com chave ativa**.
   A operação é transacional: um envelope inválido aborta todo o lote daquela
   empresa. Os tokens não mudam e não são enviados a agentes.
3. Confirme sucesso em todas as empresas, incluindo as criadas recentemente.
   Para retirar a chave antiga, considere também os backups que dependem dela.
4. Remova a chave antiga do Secret quando a política de recuperação permitir;
   reinicie novamente. Não substitua uma chave mantendo o mesmo identificador.

**Reproteger não muda a credencial no provedor.** Se houve suspeita de exposição,
revogue/rotacione o token também no provedor e grave o novo valor em **Rotacionar**.
A substituição incrementa a revisão; app e ambiente são imutáveis.

### Revogação e auditoria

Revogar elimina o envelope atual e mantém um registro inativo. Não é possível
reativar esse registro com PUT; é preciso cadastrar uma nova credencial.
Revogar funciona mesmo com o cofre bloqueado. Uso, substituição e reproteção
disputam o bloqueio da mesma linha. A revogação aguarda uma chamada já em curso
(com prazo de rede de até 10 segundos); ao responder, novos usos daquela
credencial não começam. Ela não desfaz uma chamada já feita ao provedor.

Gravação, rotação e revogação têm auditoria transacional, apenas com identificador,
ator, revisão, ação e horário. Uma tentativa durável `use.started` é escrita
antes da decifragem/rede. A conclusão registra `use.completed` ou `use.failed`.
Uma interrupção pode deixar apenas a tentativa. Só identificadores conhecidos
e autorizados são copiados do banco para a auditoria do cofre; recusas sem esse
vínculo aparecem somente no registro administrativo com rota-modelo e status,
nunca com o ID não confiável recebido na URL. Uma falha de auditoria impede o
uso ou impede que ele seja apresentado como concluído. Falhas não incluem
erros brutos do banco/provedor. Esses registros não são imutáveis perante
um administrador do banco; entrega externa resistente a alterações não existe.

## Backup e restauração

O backup PostgreSQL contém apenas envelopes, nunca a chave mestra. A exportação
JSON de Segurança é só metadados e **não é backup do cofre**.
Não disponibilize backups por uma rota pública ou como arquivo de download.
Backups completos podem conter outros dados pessoais/históricos e precisam
de criptografia e controle de acesso próprios.

**Um backup antigo não contém as revogações posteriores.** Não coloque uma
restauração bruta no ar. Procedimento obrigatório:

1. Pare o portal e impeça tráfego para o banco restaurado.
2. Restaure o backup em um banco isolado; se necessário, aplique as migrações
   até incluir `credential_vault`, mantendo o portal parado.
3. Execute com `psql -v ON_ERROR_STOP=1` no banco restaurado:
   `scripts/quarantine-restored-credentials.sql`. Ele revoga e remove envelopes
   restaurados **em todas as empresas prontas**, inclusive os que constavam
   como ativos no backup. Se falhar, não inicie o servidor.
4. Revogue/rotacione no provedor os tokens afetados e cadastre novos valores.
   Reavalie cargos, permissões e chaves antes de liberar o tráfego.
5. Só então reinicie o portal. Documente o incidente sem guardar valores secretos.

O script de quarentena é destrutivo para os envelopes restaurados; isso é
intencional para não ressuscitar autorizações antigas. Ele não remove apps,
empresas, pessoas ou chaves dos Secrets. Quem controla o banco e as chaves pode
ignorar o procedimento e reativar snapshots antigos: não se promete proteção
automática contra rollback por um operador privilegiado.

## Verificação

Os testes usam chaves e tokens sintéticos, nunca credenciais reais. Cobrem:
separação de empresas com IDs sobrepostos; vínculo a app, ambiente e cargo;
usuários sem permissão de gestão; suspensão; gravação criptografada;
ausência de valor em respostas, fontes, manifestos, auditoria, logs e jobs;
rotação do token/chave, retirada da chave antiga, adulteração de envelope,
rollback de reproteção parcial; arquivo `pg_dump`/`pg_restore` e quarentena
após restauração; rejeição de destinos internos e conteúdo ecoado pelo provedor.

```sh
cd backend
GOSUMDB=sum.golang.org go test ./internal/vault ./internal/workspace -count=1
```

Os testes de portal criam um banco de teste separado e exigem `pg_dump` e
`pg_restore` compatíveis com o servidor PostgreSQL. Nunca executam a
quarentena ou restauração no banco de desenvolvimento/produção.
