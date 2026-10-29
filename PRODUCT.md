# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users
- **Usuário (principal no dia a dia):** analistas e gestores da Alis, no computador do escritório (Windows), com pressa, entre outras tarefas. Não programam. O trabalho: achar o app certo, clicar em Rodar e pegar os arquivos que ele gerou.
- **Criador:** quem faz apps com IA (no piloto, o Henry). Tira o app da pasta, deixa usável por todos e quer saber quando quebra. Registra apps, cuida da ficha, das versões e do histórico.
- **Sócio / Administrador:** responde pela empresa. Quer ver o que existe, o que é usado e onde está o risco.

## Product Purpose
O Ziv transforma "um script na pasta de alguém" em "uma ferramenta da empresa", com dono, ficha, tela de uso e histórico. Qualquer pessoa da empresa encontra, entende e usa qualquer app, sem saber programar e sem depender de quem criou.

Sucesso, com o mesmo peso: o analista **resolve rápido e sozinho** (achou, rodou, pegou os arquivos sem pedir ajuda) e **confia no resultado** (sabe que rodou a versão certa e que os arquivos são os certos).

## Positioning
Produto próprio do Henry, não da Alis. A Alis é o cliente zero; depois, vender a outras pequenas empresas sem TI. A marca e o visual são do Ziv, não precisam combinar com a Alis. O Ziv é genérico: qualquer app entra pelo mesmo caminho, o **manifesto** (`ziv.yaml`, salvo ao lado do app; nos apps antigos, `kova.yaml` ou `app-hub.yaml`). A partir dele o app ganha ficha, botão Rodar e histórico. Os apps da Alis são o primeiro caso real e servem para testar essa generalidade, não para moldar o Ziv.

## Operating Context
- Ambiente atual (26/10/2026): Ziv web no Replit, com PostgreSQL e execução local pelo Agente do Ziv no Windows. O piloto original usava WSL e apps numa pasta do OneDrive; esse contexto não descreve o servidor atual.
- Apps com janela abrem no Windows de quem clicou, pelo **Agente do Ziv**, um programa pequeno instalado em cada computador e conectado em **Configurações › Meu computador**. Rodar = abrir o app, esperar fechar, mostrar os arquivos novos ou alterados para baixar.
- Tipos de app encontrados: script com arquivo fixo, script com janela de seleção, app web próprio, app de desktop, tarefa agendada, executável pronto, sistema complexo (só ficha) e documento (só catálogo).
- Rodar em poucos cliques: Rodar no cartão do catálogo, pasta lembrada por pessoa e app, faixa de andamento embaixo da tela, Minhas execuções.

## Capabilities and Constraints
- Interface toda em português do Brasil, linguagem simples, sem jargão técnico para o usuário.
- Perfis: usuário, criador (responsável/reserva de um app) e administrador.
- Tudo que o Ziv sabe de um app vem do manifesto e do diagnóstico; a ficha é editável à mão.
- Versões e histórico: o estado original vira versão 1; mudanças na pasta viram versões novas; dá para voltar o código.
- Implementado no fluxo de scripts: importação Python/JavaScript pelo navegador, formulário automático, upload de entrada, preparação de dependências e coleta de resultados. O fluxo completo em Windows permanece sem validação.
- Em aberto: execução gerenciada na nuvem, evolução desktop com janela e instalador único, cofre de segredos, painel de uso e funções de IA. A visão de produto não é uma declaração de funcionalidades entregues.

## Brand Commitments
- Nome: **Ziv** (antes App Hub e, por um dia, Kova; ver [ADR 013](docs/adr/013-nome-kova-com-convivencia.md) e [ADR 014](docs/adr/014-nome-ziv.md)). Em hebraico, "brilho". Curto e fácil de falar em português e inglês. Kova saiu porque, dito em voz alta, soa como "cova". Nos textos, "o Ziv".
- Voz: português simples e direto, frases curtas, sem jargão; quando algo dá errado, dizer o próximo passo.
- Visual (escolha do Henry, 2026-10-28): app neutro em preto e branco, como Notion e Vercel, com uma só cor viva, o coral `#FF6B4A`, usada só como preenchimento (Rodar e o que está rodando agora). O símbolo é um quadrado petróleo com a enseada e o ponto coral. Detalhes em `DESIGN.md`.

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
