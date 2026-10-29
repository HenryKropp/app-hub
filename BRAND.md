---
name: "Kova"
tagline: "Os apps da empresa, num lugar seguro."
specVersion: "0.3.0"
version: 1
language: pt-BR
type: master
---

# Kova

Identidade da marca: o que continua valendo mesmo se o visual for todo redesenhado. Como essa identidade aparece na tela do produto está no [`DESIGN.md`](DESIGN.md); o produto, no [`PRODUCT.md`](PRODUCT.md); a troca de nome, na [ADR 013](docs/adr/013-nome-kova-com-convivencia.md).

## Strategy

### Overview

**O que é.** O Kova é o lugar onde os apps que uma empresa cria com IA passam a morar: cada app ganha dono, ficha, botão Rodar e histórico, e qualquer colega consegue usá-lo sem saber programar.

**Origem.** Nasceu dentro da Alis Investimentos, onde dezenas de scripts e apps feitos com IA viviam espalhados em pastas do OneDrive, cada um dependendo de quem o criou. A Alis é o cliente zero; o Kova é produto próprio do Henry, feito para outras pequenas empresas sem TI. Antes se chamava App Hub (troca em 29/10/2026).

**O que ele realmente faz.** Transforma "um script na pasta de alguém" em "uma ferramenta da empresa". Não cria apps nem substitui a IA que os cria: recebe o que já existe, entende como roda e dá a ele uma casa com regras.

**O problema estrutural.** A IA tornou fácil criar apps e, com isso, criou bagunça. Cada app depende de uma pessoa, de uma pasta e de instruções que só ela sabe. Quando essa pessoa sai de férias, o app para de existir para a empresa. Ninguém sabe o que existe, qual versão rodou, nem onde está o risco (senhas no código, apps sem dono).

**A transformação.**
- **Antes:** "Me manda aquele script?", "Qual pasta?", "Rodou com a versão certa?". O app pertence a quem fez.
- **Depois:** o colega acha o app, clica em Rodar e pega os arquivos sozinho. O app pertence à empresa, com dono, reserva e histórico.

**Ambição.** Ser o porto seguro padrão dos apps que pequenas empresas criam com IA.

### Audience

**Público principal: quem usa.** Analistas e gestores de uma pequena empresa sem TI (hoje, a equipe da Alis), no computador do escritório com Windows, com pressa, entre outras tarefas. Não programam. Querem achar o app certo, rodar e pegar o arquivo gerado, sem pedir ajuda.
- **Acreditam que:** o app "do fulano" funciona, mas só se o fulano estiver por perto.
- **Desconfiam de:** sistemas novos que exigem treinamento, telas cheias de termos técnicos e ferramentas que prometem automatizar tudo.

**Público secundário: quem cria.** A pessoa da empresa que faz apps com IA (no piloto, o Henry). Quer tirar o app da pasta, deixá-lo usável por todos e saber quando quebra. Aceita detalhe técnico, mas não quer virar suporte.

**Público secundário: quem responde pela empresa.** Sócio ou administrador. Quer ver o que existe, o que é usado e onde está o risco. Fala de governança e de economia, não de código.

**Não é para:**
- empresas com time de TI e plataforma interna própria;
- desenvolvedores procurando onde construir apps do zero;
- quem quer uma loja de apps prontos de terceiros.

### Positioning

**Categoria.** A casa dos apps internos feitos com IA em empresas pequenas: registro, uso e governança num só lugar.

**O que o Kova não é.**
- Não é um construtor de apps nem um low-code. Os apps vêm de fora, feitos com IA ou à mão.
- Não é uma loja de apps.
- Não é uma ferramenta de desenvolvedor. Quem usa nunca vê código nem manifesto.
- Não é uma pasta compartilhada com outro nome.

**Paisagem competitiva.**
- **O que existe hoje de fato:** pastas no OneDrive, mensagens com instruções e a memória de quem criou. É o principal "concorrente".
- **Construtores de ferramentas internas** (Retool, Appsmith, Power Apps): pedem que o app seja refeito dentro deles e pressupõem alguém técnico.
- **Bancos de dados e automação sem código** (Airtable, Zapier, Power Automate): organizam dados e fluxos, não os apps que a empresa já tem.
- **Governança de TI para empresas grandes:** feita para times de TI, cara e pesada para quem não tem nenhum.
- **O Kova fica no meio:** aceita o app como ele é e dá a ele dono, tela de uso e histórico, sem exigir TI.

**Diferenciais estruturais.**
- Genérico: qualquer app entra pelo mesmo caminho (o manifesto), sem tela feita sob medida.
- O app não é refeito: o Kova lê o que já existe e guarda o original como versão 1.
- Quem usa só vê "Rodar" e os arquivos; a parte técnica fica com quem cuida.
- Confiança visível: qual versão rodou, onde os arquivos ficaram e o que mudou.
- Governança sem TI: dono e reserva obrigatórios, aviso de senhas no código e histórico.

**Território.** O porto seguro: o lugar protegido onde os apps da empresa ficam guardados, em ordem e prontos para usar.

### Personality

**Arquétipo:** O Guardião calmo, a pessoa organizada da empresa que deixa tudo no lugar e entrega o que você precisa sem sermão.

**Atributos:** Confiança, Ordem, Rapidez, Simplicidade, Cuidado.

**O Kova é:** Calmo. Direto. Confiável. Discreto. Prático.

**O Kova não é:** Não é técnico. Não é eufórico com IA. Não é burocrático. Não é decorativo. Não é alarmista.

### References & Anti-References

#### References

- **Spotify:** pegar o uso de uma única cor viva como sinal (o verde deles, o coral nosso) e o botão de tocar como gesto central. Não pegar o tom de entretenimento, as playlists coloridas nem o fundo escuro como padrão.
- **Brex:** pegar a seriedade de produto financeiro para empresas, que passa confiança sem ser careta. Não pegar o tom de banco nem a linguagem de startup americana.
- **Magie:** pegar o nome curto, que não descreve o produto e fica na memória, e a simplicidade para pequenas empresas brasileiras. Não pegar a ideia de produto que conversa por chat nem a personalidade de assistente.
- **Notion e Vercel:** pegar a calma de ferramenta de trabalho moderna: neutra, rápida, sem enfeite. Não pegar o vocabulário de desenvolvedor da Vercel nem a sensação de "monte do seu jeito" do Notion.

#### Anti-References

- **O visual antigo do App Hub (o fichário):** rejeitar papel creme, serifa, carimbo e uma cor por tipo de app. Parecia arquivo morto, não ferramenta.
- **SaaS genérico de IA:** rejeitar "desbloqueie o potencial", "revolucione", gradientes roxos, brilhos e promessas de automatizar tudo.
- **Painel de TI corporativo:** rejeitar telas cheias de status coloridos, siglas e alertas que assustam quem só queria rodar um app.

### Promise

**Promessa central.**
- Todo app tem dono.
- Qualquer colega roda sozinho.
- Você sempre sabe o que rodou.
- Nada se perde.

**Mensagem-base.** O Kova dá a cada app feito com IA uma casa na empresa, para que qualquer pessoa use com confiança e sem depender de quem criou.

**Frase-síntese.** Os apps da empresa, guardados, em ordem e prontos para rodar.

### Guardrails

**Tom:** calmo, direto, confiável e simples.

**O Kova não pode ser:**
- uma ferramenta de programador;
- mais uma empresa de "IA mágica";
- um sistema burocrático de controle;
- um produto da Alis (a Alis é cliente, não dona da marca);
- algo que precisa de treinamento para usar.

**Teste rápido:** se um analista com pressa precisar perguntar o que a frase quer dizer, ou se ela soar como TI dando bronca, está errado.

## Voice

### Identity

**Quem somos.**

Somos o Kova, o lugar onde os apps da sua empresa moram. A IA facilitou criar ferramentas, e de repente cada pessoa tem um script numa pasta que só ela sabe rodar. A gente recebe esses apps do jeito que eles são, entende como funcionam e dá a cada um dono, ficha e um botão Rodar.

Quem usa não precisa saber programar. Acha o app, clica em Rodar e pega os arquivos. Quem cria para de ser suporte. Quem responde pela empresa vê o que existe, quem cuida de cada coisa e onde está o risco.

Não fazemos barulho. Guardamos, organizamos e deixamos pronto para usar.

**Essência.** A empresa usa seus apps sem depender de quem os criou.

### Tagline & Slogans

**Slogan principal:** "Os apps da empresa, num lugar seguro." Para a apresentação da marca: site, proposta comercial, rodapé, perfil em redes.

**Alternativas:**
- "Do script na pasta à ferramenta da empresa."
- "Feito com IA. Cuidado pela empresa."
- "Achou, rodou, pronto."

**Slogans por contexto:**
- Para quem usa: "Clique em Rodar. O resto é com o Kova."
- Para quem cria: "Seu app sai da sua pasta e entra na empresa."
- Para o sócio: "Saiba o que existe, quem cuida e onde está o risco."
- Para as férias: "O app continua quando quem fez sai de férias."
- Para a confiança: "Você sempre sabe qual versão rodou."

### Message Pillars

**Porto seguro**
- Todo app tem um lugar, um dono e uma reserva.
- O original é guardado como versão 1, e dá para voltar.

**Sozinho e rápido**
- Qualquer colega acha, roda e baixa o resultado sem pedir ajuda.
- Poucos cliques; detalhe só para quem quiser.

**Confiança visível**
- Fica claro qual versão rodou, onde os arquivos foram salvos e o que mudou.

**Governança sem TI**
- Saber o que existe, o que é usado e onde está o risco, sem precisar de um time técnico.
- Senha no código vira aviso, não surpresa.

**Do jeito que o app já é**
- O Kova não pede para refazer nada: lê o app como ele é e cuida dele.

### Phrases

- "Todo app tem casa no Kova."
- "Da pasta de alguém para a empresa inteira."
- "O app fica. Quem fez pode sair de férias."
- "Rodou, está no Kova: versão, arquivos e quem rodou."
- "Feito com IA, guardado no Kova."
- "Nenhum app órfão."
- "Na enseada, nenhum app se perde."

### Vocabulary

| Conceito | Chamamos de | Não | Por quê |
|---|---|---|---|
| O produto | o Kova (masculino) | a Kova, Kova Hub, App Hub | Decidido pelo Henry; App Hub é o nome antigo |
| Programa, script ou executável registrado | app | ferramenta, solução, sistema, aplicação | Uma palavra só para tudo, curta |
| Executar um app | Rodar | executar, disparar, processar | O gesto central da marca |
| A página do app | ficha | perfil, detalhe, página de configuração | Diz o que é: o documento do app |
| Uma vez que o app rodou | execução | job, tarefa, processo | Português simples |
| O programa no computador de cada pessoa | Agente do Kova | cliente, daemon, serviço | Nome fixo no produto |
| O arquivo que descreve o app | manifesto | config, YAML | Só aparece para quem cria, nunca para quem só usa |
| Quem cuida do app | responsável e reserva | owner, mantenedor, admin do app | Português e papéis claros |

**Escrita:** "Kova" sempre com K maiúsculo nos textos, sem negrito ou aspas. Fala-se "kôva".

### Tonal Rules

1. Escreva em português do Brasil simples; frases curtas.
2. Fale com "você"; o Kova fala de si na terceira pessoa ("o Kova guarda") na interface e na primeira do plural ("a gente") em texto de marca.
3. Nenhum jargão para quem só usa. Se um termo técnico for inevitável, explique-o na mesma frase.
4. Comece pelo que a pessoa quer fazer, não pelo que o sistema é.
5. Erro sempre vem com o próximo passo.
6. Não exagere: nada de "revolucionário", "mágico", "inteligente" nem pontos de exclamação.
7. Mostre fatos verificáveis (versão, data, pasta, quem rodou) em vez de adjetivos.
8. Fale da IA como origem dos apps, nunca como o herói do Kova.
9. Governança é cuidado, não fiscalização: diga o que falta, sem culpar ninguém.
10. Não invente números, clientes ou depoimentos.

**O que não somos.**
- Não somos uma plataforma para programadores.
- Não somos uma empresa de IA.
- Não somos um sistema de controle que vigia as pessoas.
- Não somos uma loja de apps.
- Não somos a pasta compartilhada com outro nome.

| Dizemos | Nunca dizemos |
|---|---|
| "Rodar" | "Executar o job" |
| "O app terminou: 3 arquivos novos" | "Processamento concluído com sucesso!" |
| "Este app ainda não tem reserva. Escolha alguém." | "Violação de política de governança" |
| "Os apps da empresa, num lugar seguro" | "Desbloqueie o potencial da IA na sua empresa" |
| "Achamos uma senha escrita no código" | "Vulnerabilidade crítica detectada" |
| "Salvo no Kova" | "Persistido no repositório" |

## Visual

### Logo & Marks

- **Símbolo:** quadrado petróleo de cantos arredondados com um arco cor de areia aberto à direita (a enseada) e um ponto coral no centro (o app protegido). Geometria oficial:

  ```svg
  <svg viewBox="0 0 48 48">
    <rect width="48" height="48" rx="12" fill="#0E3B43"/>
    <path d="M33 13.5A14 14 0 1 0 33 34.5" fill="none" stroke="#F4EFE6" stroke-width="5" stroke-linecap="round"/>
    <circle cx="24" cy="24" r="5" fill="#FF6B4A"/>
  </svg>
  ```

- **Variante para fundo escuro:** o quadrado passa a `#256A75`; arco e ponto não mudam.
- **Nome escrito:** "Kova" na fonte da marca (Geist). Um nome escrito próprio fica para uma segunda rodada.
- **Área livre:** pelo menos o raio do ponto coral em volta do símbolo.
- **Proibido:** mudar as cores do símbolo, girar ou fechar o arco, tirar ou mover o ponto, esticar, aplicar sombra, brilho ou gradiente, e colocar sobre foto.
- **Fonte oficial:** o componente `Logo` em `frontend/src/components/bits.tsx` e o desenho da identidade em `docs/superpowers/specs/2026-10-28-kova-identidade-design.md`.

### Core Colors

- **Coral** `#FF6B4A` (obrigatória). O sinal: o que dá para começar e o que está rodando agora. Vale porque é a única cor viva; aparece sempre como preenchimento, nunca como cor de texto.
- **Petróleo** `#0E3B43` (obrigatória). A enseada, a proteção. Só no símbolo.
- **Areia** `#F4EFE6` (obrigatória). O arco do símbolo, o abrigo. Só no símbolo.
- **Tinta** `#11161A` (obrigatória). O texto e a base neutra da marca: quase preta, nunca preto puro.

**Evitar:** violeta (a cor do App Hub antigo), qualquer arco-íris de cores para tipos, áreas ou pessoas, e gradientes.

### Typefaces

- **Geist**, a voz da marca, em todos os textos. Fallback: `ui-sans-serif, system-ui, sans-serif`. Licença: SIL Open Font License 1.1 (The Geist Project Authors). Verificada em 29/10/2026 no pacote `@fontsource-variable/geist`.
- **Geist Mono**, só para dados com forma de máquina: caminhos de pasta, versões, datas e durações. Fallback: `ui-monospace, monospace`. Licença: SIL Open Font License 1.1. Verificada em 29/10/2026 no pacote `@fontsource-variable/geist-mono`.

### Art Direction

**Palavras-chave:** Silêncio útil, Um só sinal, Ordem, Neutro, Plano, Familiar.

**Direção:** a identidade comunica abrigo e prontidão, não decoração. Tudo é neutro para que o único ponto de cor diga "comece aqui" ou "está rodando".

**Território visual:** o botão de tocar numa sala silenciosa. Uma sala de trabalho branca, organizada e conhecida, como as ferramentas que a pessoa já usa, onde só uma coisa tem cor: o que dá para começar agora.

## Governance

### Naming

- O nome do produto mora numa constante só (`frontend/src/lib/brand.ts` e `backend/internal/brand`), para que uma troca custe uma linha.
- Partes do produto ganham nomes descritivos em português, com "do Kova" quando precisam de dono: "Agente do Kova", "Minhas execuções", "Apps da empresa". Sem nomes inventados para funções.
- "Kova" com K maiúsculo; sem plural nem variações ("Kovas", "Kova App").
- Antes de usar publicamente: a busca no INPI ainda está pendente (por conta do Henry). Domínios candidatos: kova.app e kova.com.br, ainda não registrados.

### Claims

**Aprovadas:** nenhuma ainda. Não há clientes externos, números de uso, preços nem depoimentos.

**Fatos que podem ser ditos** (vêm do produto, não são promessas de resultado):
- O Kova guarda o estado original de cada app como versão 1.
- Cada app publicado tem um responsável.
- O Kova avisa quando encontra senha ou chave escrita no código.

**Rascunhos, precisam de aprovação do Henry e de evidência:**
- "Economiza horas da equipe por semana." (sem medição)
- "Funciona com qualquer app." (o fluxo completo no Windows ainda não foi validado para todos os tipos)

**Proibidas:** números de clientes, economia ou uso sem fonte; "seguro" no sentido de certificação de segurança; qualquer menção à Alis como cliente sem autorização dela.

### Accessibility Commitments

- WCAG 2.1 AA nos dois temas, claro e escuro: texto a 4.5:1 (3:1 a partir de 24px), bordas de controle, foco e ícones a 3:1.
- O estado nunca é dito só por cor: sempre ícone e palavra.
- O tema escuro tem o mesmo nível do claro, não é de segunda categoria.
