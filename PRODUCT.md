# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users
- **Usuário (principal no dia a dia):** analistas e gestores da Alis, no computador do escritório (Windows), com pressa, entre outras tarefas. Não programam. O trabalho: achar o app certo, clicar em Rodar e pegar os arquivos que ele gerou.
- **Criador:** quem faz apps com IA (no piloto, o Henry). Tira o app da pasta, deixa usável por todos e quer saber quando quebra. Registra apps, cuida da ficha, das versões e do histórico.
- **Sócio / Administrador:** responde pela empresa. Quer ver o que existe, o que é usado e onde está o risco.

## Product Purpose
O App Hub transforma "um script na pasta de alguém" em "uma ferramenta da empresa", com dono, ficha, tela de uso e histórico. Qualquer pessoa da empresa encontra, entende e usa qualquer app, sem saber programar e sem depender de quem criou.

Sucesso, com o mesmo peso: o analista **resolve rápido e sozinho** (achou, rodou, pegou os arquivos sem pedir ajuda) e **confia no resultado** (sabe que rodou a versão certa e que os arquivos são os certos).

## Positioning
Produto próprio do Henry, não da Alis. A Alis é o cliente zero; depois, vender a outras pequenas empresas sem TI. A marca e o visual são do App Hub, não precisam combinar com a Alis. O Hub é genérico: qualquer app entra pelo mesmo caminho, o **manifesto** (`app-hub.yaml`, salvo ao lado do app). A partir dele o app ganha ficha, botão Rodar e histórico. Os apps da Alis são o primeiro caso real e servem para testar essa generalidade, não para moldar o Hub.

## Operating Context
- Piloto local no computador do Henry (Windows, Hub em WSL). Apps reais ficam numa pasta do OneDrive.
- Apps com janela abrem no Windows de quem clicou, pelo **Agente do Hub**, um programa pequeno instalado em cada computador e conectado em **Meu computador**. Rodar = abrir o app, esperar fechar, mostrar os arquivos novos ou alterados para baixar.
- Tipos de app encontrados: script com arquivo fixo, script com janela de seleção, app web próprio, app de desktop, tarefa agendada, executável pronto, sistema complexo (só ficha) e documento (só catálogo).
- Rodar em poucos cliques: Rodar no cartão do catálogo, pasta lembrada por pessoa e app, faixa de andamento embaixo da tela, Minhas execuções.

## Capabilities and Constraints
- Interface toda em português do Brasil, linguagem simples, sem jargão técnico para o usuário.
- Perfis: usuário, criador (responsável/reserva de um app) e administrador.
- Tudo que o Hub sabe de um app vem do manifesto e do diagnóstico; a ficha é editável à mão.
- Versões e histórico: o estado original vira versão 1; mudanças na pasta viram versões novas; dá para voltar o código.
- Em aberto: formulário automático para scripts, apps web, tarefas agendadas, cofre de segredos, painel de uso, funções de IA (sem chave configurada no piloto).

## Brand Commitments
- Nome: **App Hub**.
- Voz: português simples e direto, frases curtas, sem jargão; quando algo dá errado, dizer o próximo passo.
- Visual (escolha do Henry, 2026-10-02): limpo e neutro como Notion e Vercel, com uma só cor forte de destaque (violeta vivo), usada como o Spotify usa o verde. Sensação ágil e direta. Claro por padrão, com modo escuro.

## Evidence on Hand
- Um app real rodando de ponta a ponta: **Eq. Research Files Prep** (pasta "2026 06 23 - Henry Files Prep").
- Pasta de apps de teste da Alis (ver `docs/PRD.md`, seção "Variedade real").
- Não há clientes externos, depoimentos, preços ou números de uso: não inventar.

## Product Principles
1. Rodar em poucos cliques; detalhes só para quem quiser aprofundar.
2. Quem só usa nunca precisa entender código, pastas técnicas ou o manifesto.
3. Confiança visível: sempre deixar claro qual versão rodou, onde os arquivos foram salvos e o que mudou.
4. Genérico antes de sob medida: nenhuma tela feita para um app específico.
5. Erro sempre vem com o próximo passo.
