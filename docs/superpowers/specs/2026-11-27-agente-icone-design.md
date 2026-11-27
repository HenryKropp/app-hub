# Agente visível: ícone perto do relógio e desinstalar (desenho curto)

Aprovado pelo Henry em 2026-11-27.

## Por quê

Hoje o Agente se instala, liga com o Windows e fica invisível. A pessoa não
sabe se ele está ligado e não tem como desinstalá-lo. Isso parece estranho e
é ruim para a confiança, que é a proposta de valor do Ziv.

## O que entra

1. **Ícone na área de notificação** (canto inferior direito, perto do relógio).
   - Verde: conectado. Cinza: sem conexão.
   - Menu:
     - Linha de estado (não clicável): `Ziv Agente 0.5.0 — Conectado (NOME-DO-PC)`.
     - **Abrir o Ziv**: abre o site da empresa no navegador.
     - **Sair**: pede confirmação, avisando que apps agendados neste PC não
       rodam enquanto ele estiver fechado. Fecha até o próximo login no Windows.
     - **Desinstalar...**
2. **Estados de "sem conexão"**, cada um com o que fazer:
   - Sem internet ou Ziv fora do ar: "Sem conexão — tentando de novo sozinho".
     O Agente já tenta de novo, e a pessoa não precisa fazer nada.
   - PC desconectado no Ziv (tirado na tela Computadores ou parado há 90
     dias): hoje o Agente se fecha em silêncio. Passa a ficar no ícone com
     "Desconectado do Ziv" e o item **Conectar de novo**, que abre a mesma
     tela de parear da primeira instalação. Não precisa baixar nada de novo.
3. **"Ziv Agente" em Configurações → Aplicativos instalados**, com o botão
   Desinstalar do próprio Windows.
4. **Desinstalar** (pelo menu ou pelo Windows):
   - pede confirmação;
   - para o Agente e tira do início do Windows;
   - tira a entrada de Aplicativos instalados;
   - apaga a pasta do Agente;
   - avisa o Ziv, que desconecta o computador, e ele some da lista de
     Computadores. Se estiver sem internet, desinstala do mesmo jeito: o
     computador fica na lista até alguém tirar ou até os 90 dias.

## Fora

- Janela própria do Agente: o Henry descartou ("esquece a telinha").
- Ver apps rodando ou histórico pelo ícone: isso fica no site.

## Abordagens

- **Ícone com menu, em Go puro (escolhida).** Usa uma biblioteca de ícone de
  bandeja sem cgo (por exemplo `fyne.io/systray`), porque o Agente é
  compilado com `CGO_ENABLED=0`. As confirmações usam a caixa de mensagem
  nativa do Windows (`MessageBoxW`). É pequeno e não depende de nada
  instalado no PC.
- Janela própria (WebView2 ou nativa): o Henry descartou, e ela aumentaria
  muito o Agente.
- Página local no navegador servida pelo Agente: não resolve o "não sei se
  está ligado", porque a pessoa teria que lembrar de abrir.

## Decidido pelo Claude (o construtor pode ajustar)

- **Entrada de desinstalação:** em `HKCU\...\CurrentVersion\Uninstall\ZivAgente`,
  sem precisar de administrador. `UninstallString` aponta para o executável
  com `--desinstalar`.
- **Apagar a própria pasta:** um programa não apaga a si mesmo enquanto roda.
  A pasta é apagada por um processo auxiliar (`cmd`) que espera o Agente
  terminar.
- **Rota nova no servidor:** `POST /api/agent/goodbye`, autenticada com a
  chave do computador. Revoga o próprio computador, como o
  `DELETE /api/computers/{id}`, e registra a ação na auditoria.
- **Versão do Agente:** 0.5.0. Os PCs recebem pela atualização assinada.
- **Ícone:** o "Z" do Ziv com uma bolinha de estado, em `.ico` embutido.
- **Instância única:** o Agente já usa a porta `127.0.0.1:47631` como trava.
  Um segundo clique no `.exe` só pede ao Agente que já está rodando para
  mostrar o balão "O Ziv Agente já está rodando", sem abrir uma segunda cópia.
