# 001 - O Hub roda no WSL e lê as pastas do Windows

**Status:** Accepted

## Context
O piloto roda no computador do Henry (Windows). Os apps de teste ficam no OneDrive (`C:\Users\...\8. Programação`) e muitos dependem do Windows: `.bat`, `pythonw`, janelas `tkinter`, `schtasks`, `.exe`. O ambiente de desenvolvimento do Hub (Go + Postgres em container) roda no WSL.

## Decision
O servidor do Hub roda no WSL. As pessoas digitam caminhos do Windows (`C:\...` ou `\\wsl$\...`) e o Hub traduz para o caminho que o WSL enxerga (`/mnt/c/...`), mostrando sempre o formato do Windows na tela (`internal/hostpath`).

A **execução** dos apps (Fase 4) acontecerá do lado Windows, chamada a partir do WSL (interop do Windows: `cmd.exe`/`powershell.exe`), porque os apps dependem do Windows. Essa parte será detalhada numa ADR própria quando o executor for construído.

## Rationale
- Mantém a stack padrão (Go, Postgres em container, deploy em container no Módulo 2).
- Ler e escrever nas pastas do Windows pelo `/mnt/c` funciona sem instalar nada extra.
- Separar "onde o Hub roda" de "onde o app roda" é o que o Módulo 2 (nuvem) também vai precisar.

## Trade-offs
**Pros:**
- Mesmo código no piloto e na nuvem.
- Nenhuma mudança nos apps existentes.

**Cons:**
- Ler arquivos pelo `/mnt/c` é mais lento que no Linux (a varredura da pasta de teste leva alguns segundos).
- A execução vai atravessar a fronteira WSL → Windows.

## Alternatives Considered
- Servidor Go nativo no Windows: descartado por ora, porque foge da stack e do Postgres em container. Pode ser reavaliado se o interop for um problema.
- Rodar os apps no Linux: descartado, porque os apps usam `.bat`, `pythonw`, janelas do Windows e caminhos `C:\`.
