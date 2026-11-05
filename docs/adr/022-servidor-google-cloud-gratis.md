# 022 - Servidor 24h grátis no Google Cloud

**Status:** Accepted

## Context
O Henry quer o Ziv no ar 24 horas, para outras pessoas usarem, sem depender do computador dele e sem custo. Opções que ficaram de fora:
- **Vercel:** não roda o motor (long-poll do Agente, apps compartilhados, rotinas em segundo plano).
- **Replit:** o plano grátis dorme.
- **Locaweb (padrão do cofundador):** é paga.
- **Oracle Cloud Always Free (primeira escolha):** o cadastro falhou no fim com o erro genérico do antifraude.

## Decision
- **Máquina:** Google Cloud Compute Engine **e2-micro**, no nível "Always Free".
  - Região `us-east1`, porque o grátis só vale em `us-east1`, `us-central1` e `us-west1`. São Paulo é paga.
  - Ubuntu 24.04 Minimal x86, disco padrão de 30 GB, 1 GB de RAM e mais 2 GB de swap.
  - Nome `ziv`, IP 34.24.21.104, usuário `ziv`.
- **Endereço:** `https://zivapp.duckdns.org`, grátis. Os apps compartilhados ficam em `https://<app>.zivapp.duckdns.org`; o DuckDNS responde a qualquer subdomínio. Um cron de 5 em 5 minutos (`~/duckdns/duck.sh`) avisa o DuckDNS se o IP mudar.
- **HTTPS:** Caddy, com Let's Encrypt.
  - O domínio principal tem certificado normal.
  - Os subdomínios usam `on_demand_tls`, com a pergunta `ask` em `localhost:8080/healthz`.
- **Ziv:** roda como serviço systemd do usuário `ziv` (`~/.config/systemd/user/ziv.service`, com linger ligado), na porta 8080.
  - Fica em `~/ziv`, com o binário `server`, o `frontend/dist` e o `agent/AppHubAgente.exe`.
  - As variáveis ficam em `~/ziv/ziv.env` (permissão 600, fora do git).
  - Roda como usuário para que o Podman sem root dos apps compartilhados seja o mesmo do servidor.
- **Banco:** contêiner Podman `ziv-db` com `postgres:17-alpine`, e não a imagem do Supabase, que é pesada demais para 1 GB.
  - Porta 5433 e volume `ziv-pg`, o mesmo nome e porta do ambiente local, para que os apps compartilhados alcancem o banco por `host.containers.internal:5433`.
  - `shared_buffers=64MB`. Volta sozinho no boot pelo `podman-restart.service`.
  - O PostgreSQL do apt foi instalado e desligado.
- **Compilar aqui, não na máquina:** com 1 GB de RAM, o build de Node e Go apertaria. Por isso, `scripts/deploy-gcp.sh` compila o site, o servidor Linux e o Agente (apontado para o endereço novo), envia por rsync (preservando `ziv.env`) e reinicia o serviço.
- **Cópias:** `pg_dumpall` diário às 06:30 UTC em `~/backups`, guardando as últimas 7.
- **Segredos:** `HUB_SECRET_KEY` nova, só deste servidor, com cópia em `~/.ziv-producao/HUB_SECRET_KEY` no WSL do Henry. A senha do banco fica em `~/ziv/.dbpw` na máquina.

## Consequences
- O custo é zero, dentro dos limites do nível grátis: 1 instância e2-micro, 30 GB de disco padrão e 1 GB de saída por mês para fora da América do Norte. Convém olhar a fatura nos primeiros dias.
- 1 GB de RAM é pouco: poucos apps compartilhados ao mesmo tempo. Se apertar, o caminho é uma máquina paga (cerca de R$25/mês).
- As cópias ficam no mesmo disco: protegem contra erro, não contra perder a máquina. Falta uma cópia fora dela.
- O `ask` do TLS sob demanda aceita qualquer subdomínio: o `/healthz` cai na página do site e responde 200. Alguém poderia gastar o limite do Let's Encrypt pedindo nomes aleatórios. Falta um endpoint que só aceite apps que existem.
- O endereço fica gravado no Agente e no conector do Claude. Trocar para um domínio próprio depois exige recompilar o Agente e baixar de novo.
- Banco novo e vazio: os dados locais e os do Replit não foram trazidos.
