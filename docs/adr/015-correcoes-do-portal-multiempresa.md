# ADR 015 — Correções do portal multiempresa

Data: 2026-11-03. Status: aceita.

## Contexto
Entre 29/10 e 02/11, o Ziv virou um portal com várias empresas (`/w/<empresa>`), cadastro público, convites, cofre de credenciais e a tela Segurança. Parte disso foi feita pelo Agente do Replit. Na revisão de 03/11 apareceram regressões e abusos possíveis:
- os apps compartilhados não ligavam mais em nenhuma empresa;
- o limite de login por endereço de rede travava a empresa inteira atrás de um proxy;
- o cadastro aberto podia esgotar as 20 vagas e tomar o endereço da Alis;
- a aprovação obrigatória bloqueava os apps já em uso;
- a publicação no Replit não montava;
- empresas novas davam 404 em outra cópia do servidor;
- testes de credencial seguravam conexões;
- chamadas anônimas enchiam a auditoria.

## Decisão
1. **Apps compartilhados só na empresa principal.** Eles voltam a ligar com `workspace.WithPrincipal`. As outras empresas seguem com a resposta "desativada para esta empresa" até os containers, bancos e endereços passarem a ser por empresa.
   - **Substituída pela ADR 025** (2026-11-09): apps com tela em todas as empresas, ligados por empresa.
2. **Limite de login por conta.** 10 senhas erradas por e-mail. O endereço de rede ganha um teto alto (300 por 15 min). Login certo não conta, e só POST conta. Com a tabela cheia, sai a janela mais antiga.
3. **Cadastro com reservas e cota.**
   - Endereços reservados: os fixos no código mais `RESERVED_COMPANY_SLUGS` (`alis` e `alis-investimentos` no `.replit`).
   - `COMPANY_LIMIT` (padrão 20) e `COMPANY_SIGNUPS_PER_DAY` (padrão 10).
   - Só a empresa principal cria empresas por dentro.
4. **Migração 015.** Aprova uma vez a versão atual dos apps que ninguém revisou. Decisões já tomadas ficam como estão, e apps e versões novos continuam pedindo aprovação.
5. **Replit.**
   - O deploy monta com `post-merge.sh` e roda `run-replit.sh`, com `BASE_URL`/`HUB_URL` de produção.
   - Uma empresa desconhecida é procurada no registro antes do 404.
6. **Um teste de credencial por vez em cada empresa**, com pool de 8 conexões. Chamadas anônimas fora das rotas de entrada não gravam auditoria.

## Rationale
Cada correção é a menor que tira o risco sem desfazer o desenho do portal. Apps compartilhados por empresa exigem renomear containers, redes e endereços: é trabalho de uma frente própria, não de uma correção.

## Trade-offs
**Prós:**
- A Alis volta a ter os compartilhados (destrava o News Scrapper).
- Login e cadastro resistem a abuso simples.
- Os apps em uso não param.

**Contras:**
- Sem confirmação de e-mail, alguém ainda pode gastar a cota diária de cadastros (o administrador sobe `COMPANY_SIGNUPS_PER_DAY` ou `COMPANY_LIMIT`).
- O limite de login segue por cópia do servidor.
- O `.replit` de deploy só se confirma na primeira publicação.

## Alternativas consideradas
- **Fechar o cadastro público:** descartado, porque o cadastro curto é o caminho do primeiro uso (revisão de UX de 01/11).
- **Confirmação por e-mail:** exige um serviço de envio. Fica para quando houver SMTP.
- **VM reservada no Replit (uma cópia só):** custa mais. Carregar a empresa do registro resolve sem custo.
