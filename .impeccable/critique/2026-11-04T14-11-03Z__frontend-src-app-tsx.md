---
target: todas as telas do Ziv
total_score: 22
max_score: 40
na_heuristics: 
p0_count: 1
p1_count: 4
target_identity: "file:/home/henrykroppalisinvest/app-hub/frontend/src/App.tsx"
target_fingerprint: "sha256:f7ee0b10775acfbcab3446c9caca57234cb0bda5d70aae9d23541aea7aaa0fd6"
target_path: /home/henrykroppalisinvest/app-hub/frontend/src/App.tsx
timestamp: 2026-11-04T14-11-03Z
slug: frontend-src-app-tsx
---
# Crítica de design — todas as telas do Ziv (2026-11-04)
Method: dual-agent (A: design review · B: detector + browser)

Nota Nielsen: 22/40 (Aceitável). 1 Status 3 · 2 Mundo real 2 · 3 Controle 3 · 4 Consistência 1 · 5 Prevenção de erro 2 · 6 Reconhecer 3 · 7 Eficiência 2 · 8 Minimalismo 2 · 9 Recuperar de erro 2 · 10 Ajuda 2.

Detector: código-fonte 0 achados; navegador /entrar 6, / 5, /apps 26, /apps/6 15, /configuracoes 8 (maioria contraste baixo de texto cinza sobre creme, texto pequeno; banner "paleta creme").

P0 O visual em produção (index.css, commit 942c2d9 de 30/10: creme #F6F4EE, petróleo, sidebar sálvia, coral #E87557, pêssego no menu ativo, serifa e gradiente nas páginas públicas) contradiz o DESIGN.md (neutro preto e branco, coral #FF6B4A só em Rodar). Decidir qual é o verdadeiro.
P1 Rodar parece disponível com o Agente desconectado (CatalogPage, AppPage).
P1 Coral em ações que não rodam nada: + Novo artifact, Criar artifact, + no celular, menu ativo, links.
P1 Jargão/inglês: "Artifacts", "sandbox", "Choose Files", "Hub" no /registrar, "Identificador" no /acesso.
P1 /conectar sem código: beco sem saída; mensagem genérica "Não foi possível completar a ação." (api.ts:326).
P2 Excesso de faixas: aviso do Agente + "Faltam alguns passos" + cartão de execução de 7 dias atrás cobrindo botões.
P2 Home serve ao admin, não ao analista: sem Rodar nos favoritos/recentes; avisos duplicados; Meu time longo.
P2 Nomes e ênfase inconsistentes: "O que você rodou" x "Minhas execuções"; "Aprovado" x "Liberado"; "App" x "Janela"; Rodar de novo coral x contorno.
P3 /admin: parede de tabelas vazias; "Pausar todos"/"Apagar" no fim e com cara de inofensivos.
P3 Voltar versão é o botão principal em /apps/:id/versoes/:n, sem confirmação.

Fortes: página do resultado (RunPage); status sempre ícone + palavra; ficha do app (O que ele pede / entrega / Quem cuida).
