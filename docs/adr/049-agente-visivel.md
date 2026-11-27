# 049 - Agente visível: ícone perto do relógio, desinstalar e reconectar

**Status:** Accepted

## Context
O Agente se instalava, ligava com o Windows e ficava invisível. A pessoa não
sabia se ele estava ligado nem tinha como desinstalá-lo. Quando o computador
era desconectado no Ziv, o Agente se fechava em silêncio e tirava a si mesmo
do início do Windows. O Henry aprovou o desenho em 2026-11-27
(`docs/superpowers/specs/2026-11-27-agente-icone-design.md`).

## Decision
- **Ícone na área de notificação** (`fyne.io/systray`, Go puro no Windows,
  porque o Agente é compilado com `CGO_ENABLED=0`).
  - Verde quando conectado, cinza nos outros estados.
  - Menu: estado, Abrir o Ziv, Conectar de novo (só quando desconectado),
    Sair e Desinstalar...
  - Confirmações com `MessageBoxW`.
- **Estado e reconexão em `internal/agent/session.go`.** `Session` faz
  parear → hello → loop e publica o estado num `Status`. `Agent.Loop` avisa
  quando o Ziv para de responder ou volta (`OnReachable`).
  - Quando o Ziv recusa a chave, o Agente grava a marca `desconectado` na
    pasta e espera o clique em Conectar de novo. Ele não abre o navegador
    sozinho a cada boot e continua no início do Windows, para o ícone
    aparecer.
- **Aplicativos instalados.** O Agente grava, a cada início,
  `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\ZivAgente`
  (empresa principal) ou `ZivAgente-<empresa>` (as outras), sem precisar de
  administrador, com nome "Ziv Agente (endereço do Ziv)" e
  `UninstallString = "<pasta>\AppHubAgente.exe" --desinstalar`. Cada empresa
  aparece e sai sozinha, como no início do Windows.
- **Desinstalar** (pelo menu ou pelo Windows), passos que rodam mesmo se um
  falhar:
  1. fecha o Agente;
  2. `POST /api/agent/goodbye` (o computador se desconecta; sem internet,
     segue assim mesmo);
  3. tira do início do Windows;
  4. tira a entrada de Aplicativos instalados;
  5. apaga a pasta por um `cmd` que espera o Agente sair. A pasta da empresa
     principal guarda as outras em `workspaces\`: dela saem só os arquivos
     dela, e a pasta só some se ficar vazia.
  - Gravado na auditoria como `computer.goodbye`.
- **Baixar e abrir o Agente de novo** apaga a marca `desconectado`: é um
  pedido claro de conectar, e o navegador abre para aprovar.
- **Segunda abertura** do `.exe` instalado com o Agente rodando: caixa de
  mensagem "já está rodando". O `fyne.io/systray` não mostra balão.
- Agente 0.5.0.
- **Desenho do ícone** (escolhido pelo Henry em 2026-11-27, Agente 0.5.1): o
  ícone oficial do Ziv (quadrado petróleo, haste do i cor de areia) com uma
  bolinha de estado no canto. Conectado: pingo coral (o farol aceso) e
  bolinha verde. Desconectado: pingo e bolinha cinza. Gerado por
  `backend/cmd/agent/icons/gerar.go`.

## Rationale
- Ícone com menu resolve o "não sei se está ligado" sem janela própria. O
  Henry descartou a "telinha".
- A marca de desconectado evita abrir o navegador sozinho em todo boot de um
  PC removido.

## Consequences
- O Agente fica maior (biblioteca do ícone).
- Um PC desconectado continua com o Agente no início do Windows, cinza, até
  alguém reconectar ou desinstalar.
- Testes do ícone em si (desenho e cliques) só no Windows, à mão. A lógica
  (estados, rótulos, passos de desinstalação) tem testes automáticos.
