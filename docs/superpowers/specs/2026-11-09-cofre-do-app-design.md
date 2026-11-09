# Cofre do app: chaves de scripts guardadas no Ziv

**Data:** 2026-11-09 · **Pedido do Henry** depois do teste da Etapa 2 do MCP

## Por quê
O Henry tentou subir pelo Claude um script de verdade, um baixador do BTG que usa e-mail e senha. O Ziv recusou três vezes porque achou senhas no código. Não havia para onde mandá-las:
- o "Cofre de credenciais" está trancado no servidor e nunca entrega um valor a um script;
- só os apps com tela guardam chaves.

Nas palavras dele: "Essa parte de senhas tá muito ruim. Cadê nosso cofre?".

## O que o Henry decidiu
1. **Cada app tem o seu cofre** ("um cofre dentro de cada projeto").
2. **Só o administrador e o dono do app mexem no cofre.**
3. Quem só usa o app roda o script normalmente: o script recebe as chaves, mas a pessoa nunca as vê.
4. **O administrador e o dono podem ver o valor** com um botão "Mostrar". Cada visualização fica registrada no histórico de segurança.
5. O fluxo pelo Claude:
   - ele troca a senha no código por `os.environ["NOME"]` e declara o nome na ficha;
   - o Ziv devolve o link "preencha as chaves aqui";
   - a pessoa digita o valor no site, nunca na conversa.
6. Quando o Agente roda o script, pega as chaves só para aquela execução e entrega só ao script, sem gravar em disco nem em log. Isso exige uma versão nova do Agente.

## Como funciona (escolhas técnicas do cofundador)
- **Nomes na ficha, valores no cofre.**
  - A ficha (manifesto) ganha `keys: [{name, label, required}]`, só para scripts. Os nomes seguem o formato `[A-Z_][A-Z0-9_]*`, no máximo 20 por app, e nunca começam com `_`.
  - Os valores vão para a tabela `app_config`, que já existe e é a mesma dos apps com tela. Ficam com `is_secret = true`, cifrados com a chave do servidor (`HUB_SECRET_KEY`, AES-256-GCM, pacote `vault`). Sem essa chave, o cofre fica fechado e o Ziv diz isso.
- **Quem pode:** gravar, apagar e mostrar exige ser o dono do app (`owner_id`) ou ter a permissão `manage_roles` (administrador). Quem pode ver o app vê os nomes e se cada chave está preenchida, nunca o valor.
- **Rotas, todas auditadas sem o valor:**
  - `GET /api/apps/{id}/keys`: lista nome, rótulo, se é obrigatória, se está preenchida, quem preencheu e quando, e se a pessoa pode mexer;
  - `PUT /api/apps/{id}/keys/{name}`: grava um valor com `{value}`, de no máximo 4096 caracteres, sem quebra de linha nem NUL;
  - `DELETE /api/apps/{id}/keys/{name}`: apaga;
  - `POST /api/apps/{id}/keys/{name}/reveal`: devolve `{value}` e grava `app_key.revealed` em `security_audit`.
- **Entrega ao Agente:** a chave nunca vai em `agent_jobs.payload`.
  - O pedido de execução marca só `keys: true` no payload.
  - Ao pegar o trabalho, o Agente chama `GET /api/agent/jobs/{id}/keys` com o token do computador. A rota responde só para:
    - um trabalho de execução,
    - daquele computador,
    - já pego e ainda não terminado,
    - e uma vez só.
  - Ela devolve `{NOME: valor}` e grava `app_key.delivered`.
  - O Agente põe as chaves no ambiente do processo do script depois de instalar as dependências: o pip e o npm não as recebem. Elas não entram no console salvo, que é só o que o script imprime.
  - Vale também para execuções agendadas, que usam o mesmo caminho.
- **Chave obrigatória vazia:** a execução é recusada antes de ir ao PC, com "Falta preencher a chave X no cofre do app" e o link.
- **Agente 0.3.4:** a versão sobe porque o servidor exige versão exata. O Agente se atualiza sozinho como hoje.
- **Pelo Claude (MCP):**
  - `registrar_app` aceita `keys` na ficha, também numa versão nova;
  - a resposta lista as chaves que faltam preencher e dá o link `.../apps/N#chaves`;
  - a recusa por senhas no código manda trocar o valor por `os.environ["NOME"]` e declarar o nome em `keys`;
  - `ver_app` mostra as chaves (nome e se está preenchida).
- **Site:** a página do script ganha a seção **"Cofre"**, com cada chave, o estado, Preencher/Trocar, Mostrar e Apagar para quem pode. A âncora `#chaves` abre direto nela.

## Fora do escopo
- O "Cofre de credenciais" antigo (testes HEAD), que continua como está.
- Mostrar valores das configurações dos apps com tela.
- Chaves por ambiente.

## Riscos aceitos
- Um script pode imprimir a própria chave. Isso iria só para o console da pasta de saída de quem rodou, e o código é o aprovado.
- O botão "Mostrar" expõe o valor a um login de dono ou administrador; fica no histórico.
