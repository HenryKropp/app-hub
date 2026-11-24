# Fase 0 de segurança: o que o Henry faz fora do código

O código está no branch `seguranca-fase0` (ADR 048). Estes passos são seus,
nesta ordem. Os passos 1 e 2 você faz sozinho, no seu PC. Os passos 3 a 5
mexem no Google Cloud e no servidor. Peça ao Claude para fazer junto e dê o
"pode" quando for a hora.

Todos os comandos rodam no **WSL** (Ubuntu), onde já estão `mise`, a chave
SSH do servidor e a pasta `~/.ziv-producao`.

## 1. Chave que assina o Agente (5 min)

Sem ela, o deploy para. Ela fica só no seu PC: quem tiver essa chave consegue
mandar programa para todos os PCs pareados.

```sh
cd <pasta do repositório>/backend
mise x -- go run ./cmd/agentkey gerar ~/.ziv-producao/agente-assinatura.key
```

O comando mostra uma linha (a chave pública). Não precisa guardar essa linha.

**Faça uma segunda cópia** do arquivo `~/.ziv-producao/agente-assinatura.key`
num lugar seu: o gerenciador de senhas (como anexo) ou um pendrive guardado.
Se ela se perder, todos os PCs precisam reinstalar o Agente uma vez.

## 2. Chave das cópias do banco (5 min)

O servidor cifra a cópia com a parte **pública**. Só a parte privada, que fica
com você, abre a cópia.

```sh
sudo apt install age
age-keygen -o ~/.ziv-producao/backup-chave.txt
age-keygen -y ~/.ziv-producao/backup-chave.txt > ~/.ziv-producao/backup-recipient.txt
```

Faça uma segunda cópia de `backup-chave.txt`, como no passo 1. Sem ela, as
cópias não abrem.

## 3. Bucket no Google Cloud (10 min, custo perto de zero)

No Console do Google Cloud, projeto do Ziv:

1. **Cloud Storage → Criar bucket.**
   - Nome: por exemplo `ziv-copias-<algo-seu>`.
   - Região: `us-east1`, onde até 5 GB é grátis.
   - Classe: Standard.
   - Acesso: uniforme. Bloqueie o acesso público.
2. No bucket criado, abra **Proteção → Política de retenção: 30 dias.** Não
   trave a política ("lock") por enquanto. Assim dá para ajustar depois.
3. **IAM → Contas de serviço → Criar:** nome `ziv-copias`, sem papel no
   projeto.
4. De volta ao bucket, em **Permissões → Conceder acesso:** a conta
   `ziv-copias@...` com o papel **Storage Object Creator**. Esse papel só
   grava: a conta não lê nem apaga.
5. Na conta de serviço, abra **Chaves → Adicionar chave → JSON.** Salve o
   arquivo como `~/.ziv-producao/backup-gcp.json`.

## 4. Publicar e ligar no servidor (com o Claude, depois do seu "pode")

O Claude faz, com o seu "pode" explícito:

1. Rodar `bash scripts/deploy-gcp.sh`. Ele assina o Agente com a chave do
   passo 1, envia o servidor e reinicia.
2. Na VM:
   - instalar o `age` (`sudo apt install age`);
   - copiar `backup-recipient.txt` e `backup-gcp.json` para `~/ziv/`;
   - criar `~/ziv/backup.env` a partir de `deploy/backup.env.example`, com o
     nome do bucket.
3. Ligar a cópia diária:
   - copiar `deploy/ziv-backup.service` e `deploy/ziv-backup.timer` para
     `~/.config/systemd/user/`;
   - rodar `systemctl --user daemon-reload` e depois
     `systemctl --user enable --now ziv-backup.timer`;
   - rodar uma cópia na hora (`systemctl --user start ziv-backup`) e
     conferir que ela chegou ao bucket.
4. Tirar o cron antigo do `pg_dumpall` (`crontab -e`). Depois que a cópia
   nova chegar ao bucket, apagar as cópias antigas em texto aberto de
   `~/backups`.
5. Conferir o isolamento de dentro de uma caixa de app (antes, anotar a
   versão do Podman e a configuração de rede):
   - o Ziv na 8080 não responde, nem por `127.0.0.1` nem pelo endereço da
     máquina. Se responder, parar e corrigir antes de seguir;
   - o Postgres na 5433 responde;
   - um `X-Real-IP` falso não muda o endereço registrado.
6. Fechar o Postgres para fora: hoje ele está publicado em `0.0.0.0:5433`.
   Ele passa a ser publicado só onde as caixas alcançam. Isso exige recriar o
   contêiner `ziv-db`; o volume `ziv-pg` com os dados fica. Antes, testar numa
   caixa de teste e tirar uma cópia.
7. Testar `--read-only` nas caixas com os apps que estão no ar. Se nenhum
   quebrar, ligar.

## 5. Depois do deploy (quando quiser)

- **Trocar a chave do cofre** (`HUB_SECRET_KEY`): não é obrigatório agora; o
  importante é que agora dá. Passo a passo em `docs/INFRASTRUCTURE.md`.
- **Checagem no GitHub:** a cada envio, a aba **Actions** do repositório
  mostra se os testes passaram e se há falha conhecida nas bibliotecas. Hoje
  sobra uma falha alta nas bibliotecas do site (`undici`). Ela vem de
  ferramentas de teste e do CLI do shadcn, e não vai para o navegador. Está
  anotada no `TASKS.md`.

## O que muda para quem usa

- **Código do Claude:** passa a valer 90 dias. A tela mostra até quando. O
  login pelo navegador continua se renovando sozinho.
- **Sessão no site:** cai depois de 7 dias sem uso.
- **Computador:** um PC que ficar 90 dias sem falar com o Ziv precisa ser
  pareado de novo.
- **Pessoas:** na Equipe, quem gerencia pessoas pode **Desativar** e
  **Reativar** alguém.
- **Empresa nova:** escolhe na criação entre começar com as aprovações
  ligadas ou livre para testar.
