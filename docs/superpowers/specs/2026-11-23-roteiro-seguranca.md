# Roteiro de segurança e governança do Ziv

> Aprovado pelo Henry em 2026-11-23. É um roteiro, não um desenho: cada fase
> ganha o próprio desenho (brainstorming → spec → plano) antes de construir.

## Por quê

Segurança é o argumento mais importante para o cliente, então tem que ser a
parte mais robusta do produto (Henry, 2026-11-23). O público agora é empresa
de 50 a 300 pessoas, onde quem compra é TI, segurança ou operações. Um
prospect do setor financeiro já disse que a TI proíbe deploy externo
(Supabase, Vercel, upload externo) e que todo app interno passa pela fila da
TI; o piloto depende de a segurança estar mais robusta.

"Governança", para o Henry, é sobretudo o controle de **dados externos**:
o que vai para a IA, a que sites e serviços os apps se conectam e os dados
pessoais (LGPD) que passam pelos apps.

## Decisões do Henry

- Onde os dados moram: **o cliente escolhe** entre formas de instalação
  (nuvem do Ziv, híbrido com apps na nuvem do cliente, Ziv inteiro no
  cliente). A ordem de construção está na Fase 3.
- Roteiro primeiro, construção depois, uma fase por vez.
- As três frentes de dados externos (IA, acessos de fora, LGPD) são o
  centro do roteiro.

## Onde estamos (inventário de 2026-11-23, conferido no código)

**Já existe:** senha com bcrypt; sessão em cookie HttpOnly guardada só como
hash; MCP com OAuth + PKCE e tokens revogáveis; pareamento do Agente aprovado
por pessoa; um schema do Postgres por empresa; cargos, áreas e acessos por
app (ADR 009); aprovações com etapa de TI (ADR 046); cofre AES-256-GCM
(ADR 027/028) e cofre de credenciais com chaves versionadas; registro de
ações (`security_audit`, `governance_events`); contêiner por app com limite
de memória e CPU; proteção contra CSRF e cabeçalhos de segurança; links
externos com prazo e código por e-mail (ADR 045). O servidor do Ziv não
chama nenhuma IA: a IA só entra pelo Claude Code de cada pessoa, via MCP.

**Falta:** ver as fases abaixo. Revisões anteriores: `threat_model.md` e a
varredura de 2026-11-13 (`CLAUDE-SECURITY-20261113-184628/`).

## Fase 0: fechar furos já encontrados

Curta (dias). Itens conferidos no código:

1. **Limite de tentativas não vale por pessoa no servidor.**
   `security_middleware.go` usa `r.RemoteAddr`, que atrás do Caddy é sempre
   o próprio servidor; na prática o limite é global (cadastros, login,
   pareamento) e um atacante pode travar o login de todos. Usar `X-Real-IP`
   só vindo de loopback, como `external_page.go` já faz.
2. **Contêineres dos apps alcançam o servidor e a internet inteira.**
   Chegam a `host.containers.internal` (Ziv na 8080, Postgres na 5433) e a
   qualquer site. Bloquear o host e, na Fase 2, liberar só os sites do app.
   Faltam também `--cap-drop=ALL`, `--security-opt no-new-privileges`,
   `--pids-limit` e, onde der, `--read-only`. Conexão ao banco usa
   `sslmode=disable`.
3. **Cópias de segurança na mesma máquina e sem criptografia.** Criptografar
   e mandar uma cópia para fora da VM.
4. **Chave do cofre (`HUB_SECRET_KEY`) junto do banco**, sem rotação.
   Passar para um gerenciador de chaves (KMS) ou, no mínimo, separar e
   permitir rotacionar. Confirmar se `CREDENTIAL_VAULT_KEYRING` está ligado
   em produção.
5. **Tokens "código" do MCP nunca expiram.** Dar prazo e renovação.
6. **Atualização do Agente sem assinatura própria.** O hash vem do mesmo
   servidor que entrega o binário; quem tomar o servidor roda código em
   todos os PCs pareados. Assinar com chave fora do servidor (e,
   antes de vender, Authenticode, já previsto em "Ziv no Windows").
7. Menores: token do Agente em texto no `agente.json` (usar DPAPI no
   Windows); sessões sem expirar por inatividade e que não caem ao trocar o
   cargo; registro dinâmico do OAuth aceita qualquer endereço https; empresa
   nova nasce com aprovações desligadas; registro de ações sem app, chave,
   IP nem leituras; sem varredura de dependências (govulncheck, npm audit)
   nem CSP.

## Fase 1: quem entra

- Login com Google e Microsoft (OIDC; SAML se um cliente pedir).
- Segundo fator (app autenticador) para quem não usa o SSO; obrigatório para
  administradores.
- Trocar e recuperar a senha pela própria pessoa.
- Sessão que expira por inatividade; sessões caem ao trocar o cargo ou
  remover a pessoa; lista de sessões ativas.
- Revisão periódica de acessos (o administrador confirma quem ainda deve
  ter acesso).

## Fase 2: dados que saem (o centro)

**Apps acessando fora**
- Cada app declara os sites que usa (`external_domains` já existe, só
  informativo); a TI aprova e o Ziv **bloqueia o resto** no servidor e
  registra cada acesso. Nos PCs, ao menos registrar o que o app acessou.
- O que o app acessa entra no pedido de aprovação (ADR 046).

**Dados indo para a IA**
- Registro do que cada pessoa mandou para a IA pelo Ziv (MCP) e por quais
  apps.
- Regras que a TI liga: por exemplo, não mandar CPF, dados de cliente ou
  senhas.
- Painel de uso de IA para o gestor (prioridade 2 do Henry): quem usa, para
  quê, quanto custa, quais dados foram.
- Responder com clareza: plano comercial do provedor, sem treino, retenção
  (ZDR), e a opção de o cliente usar a própria conta de IA (inclusive dentro
  da nuvem dele, via Bedrock ou Vertex).

**Dados pessoais (LGPD)**
- Apagar uma pessoa e uma empresa inteira (com prazo e prova).
- Apagar dados antigos sozinho (hoje a limpeza de execuções é manual).
- Exportar os dados de uma pessoa a pedido dela.
- Registro das operações de tratamento (LGPD art. 37).

## Fase 3: onde os dados moram

Na ordem:
1. **Servidor de verdade no Brasil** (região São Paulo), cópias fora da
   máquina, banco com criptografia e monitoramento. Hoje: GCP e2-micro em
   us-east1.
2. **Híbrido**: o portal fica com o Ziv; apps, arquivos e senhas rodam numa
   máquina na nuvem do cliente com um Agente de servidor ("Agente na nuvem do
   cliente", TASKS). Atende quem proíbe deploy externo.
3. **Ziv inteiro no cliente** (ADR 005 já prevê), para a TI mais rígida.

## Fase 4: prova para a TI (em paralelo, quase sem código)

1. Página e documento de segurança (5–10 páginas): as formas de instalação,
   o caminho dos dados, criptografia, cofre, o que vai para a IA.
2. Contrato de tratamento de dados (DPA) em português com as cláusulas-padrão
   da ANPD (Res. 19/2024) para a transferência a provedores no exterior.
3. Anexo para o setor financeiro com as cláusulas do art. 17 da
   Res. CMN 4.893/2021 (países dos dados, acesso do BACEN, aviso de
   subcontratação, entrega e exclusão no fim do contrato).
4. Lista pública de suboperadores (Google Cloud, Resend e, quando houver,
   provedores de IA) com país e finalidade.
5. Questionário já respondido (CAIQ-Lite ou SIG Lite).
6. Plano de resposta a incidentes com aviso ao cliente em até 48 h
   (o controlador tem 3 dias úteis para avisar a ANPD, Res. 15/2024) e
   contato de privacidade.
7. Políticas básicas: segurança, acessos, desenvolvimento seguro, retenção.
8. Teste de invasão por empresa de fora, com resumo e correções.
9. Depois: plataforma de automação de conformidade (Vanta, Drata,
   Secureframe ou similar) e ISO 27001 (selo mais reconhecido no Brasil,
   sobretudo no financeiro; ~6–12 meses). SOC 2 se aparecer cliente de
   cultura americana.

Custos a decidir pelo Henry: servidor pago, teste de invasão, ferramenta de
conformidade e certificação.

## Próximo passo

Desenhar a Fase 0 (furos), que é curta e não depende de decisão de custo.
