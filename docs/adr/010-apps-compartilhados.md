# 010 - Apps compartilhados

**Status:** Accepted

## Context
Até aqui o Hub só abria apps no computador de quem pedia (pelo Agente). Alguns apps, como o Sistema Alis, precisam ficar no ar para muita gente usar ao mesmo tempo, com um banco só e os mesmos dados para todos. Desenho completo em `docs/superpowers/specs/2026-10-06-apps-compartilhados-design.md` e plano em `docs/superpowers/plans/2026-10-06-apps-compartilhados.md`.

## Decision
- **Podman, com motor trocável.** Cada app compartilhado roda numa caixa (contêiner) criada pelo Hub, atrás de uma interface de motor (`CONTAINER_ENGINE`, padrão `podman`). Toda caixa e imagem leva o rótulo `app-hub.app=<id do app>`, nome `apphub-<slug>-<implantação>`, imagem `localhost/apphub/<slug>:<implantação>`, na rede `app-hub-apps`.
- **Um banco por app, dentro do `app-hub-db`.** Cada app que pede banco ganha o banco `app_<id>` e um usuário só dele. O app recebe `DATABASE_URL`, `PORT`, `BASE_URL` e `APPHUB_DATA_DIR=/data`; os valores de senha nunca voltam pela API.
- **Porteiro no próprio Hub, por subdomínio.** Cada app responde em `<slug>.localhost:8080` (a caixa de teste em `<slug>--teste.localhost:8080`). O Hub confere quem é a pessoa e se ela pode abrir o app, e só então repassa o pedido. O acesso é reconferido a cada 5 minutos.
- **Entrada pelo Hub, sem login por app.** A pessoa entra no Hub, que gera um código de uso único válido por 60 segundos; o endereço do app o troca por um cookie de 12 horas.
- **Cabeçalhos `X-AppHub-*`.** O app recebe `X-AppHub-User-Id`, `-Email`, `-Name` e `-Role`. O nome e o papel vão codificados em URL (para aguentar acentos), então o app precisa decodificar. Qualquer `X-AppHub-*` vindo do navegador é apagado.
- **Publicar lado a lado, com volta automática.** A versão nova sobe ao lado da atual; só quando responde bem na porta ela assume o lugar. Se falhar, a anterior continua no ar. Só uma publicação por app de cada vez (a segunda recebe 409). "Voltar para a anterior" volta só a versão, ou a versão e os dados.
- **Descansar quando parado.** O app dorme após 30 minutos sem acesso e acorda no próximo pedido (página "acordando"). O vigia confere a saúde a cada 30 s e religa um app que caiu, até 3 vezes em 10 minutos.
- **Cópia diária do banco**, às 3h, guardada 14 dias. Cópias antes de publicar e antes de restaurar ficam 30 dias; também há cópia manual.
- **Custo estimado por memória × tempo.** O uso é medido a cada minuto e guardado 90 dias; com o valor mensal do servidor informado pelo administrador, o Hub reparte o gasto entre os apps.
- **Permissão `manage_server` ("Cuidar do servidor")**, dada ao administrador. Quem a tem age sobre qualquer app compartilhado (parar, ligar, mudar limites) e vê a página "Servidor e gastos". Publicar, voltar, configurar e restaurar um app é de quem pode restaurá-lo; ver mensagens e uso, de quem vê a parte técnica.
- **Exceção de segurança do Podman (aprovada em 2026-10-07).** O Hub pode rodar `podman exec` dentro do contêiner compartilhado `app-hub-db`, só para `pg_dump` e `pg_restore` (cópias). Parar, remover, remover imagem e ler mensagens (`Stop`, `Remove`, `RemoveImage`, `Logs`) continuam restritos a caixas com o rótulo `app-hub.app`. O `app-hub-db` e o `sistema-alis-db` nunca são parados nem removidos pelo Hub.
- **O Hub tranca o próprio banco.** A cada início, o Hub tira de `PUBLIC` a permissão de conectar (`CONNECT`) no banco dele (`LockDown` do `appdb`). Assim, os usuários dos apps não conseguem abrir o banco do Hub, mesmo estando no mesmo servidor de banco.
- **Checagem antes de publicar** com receitas automáticas (Streamlit, Python web, Node) e um "texto para o Claude" quando falta algo.

## Rationale
Podman roda sem exigir um servidor central e já existe nos computadores do time. Um banco por app dá isolamento e cópia simples sem pagar um servidor de banco por app. O porteiro dentro do Hub reaproveita o login e as permissões que já existem (ADR 009) e evita que cada app tenha sua própria tela de entrada. Publicar lado a lado e voltar sozinho protege quem está usando durante uma atualização.

## Trade-offs
**Pros:**
- Um app vira compartilhado sem reescrever o login nem o banco
- Um login e uma regra de acesso só, a mesma do catálogo
- Atualização sem derrubar o app; cópia diária e volta de versão

**Cons:**
- Precisa do Podman ligado; sem ele, só os apps compartilhados ficam indisponíveis (o resto do Hub funciona)
- A geração de PDF do Sistema Alis não funciona dentro da caixa; a checagem avisa
- O Hub passa a ter uma exceção de `exec` no contêiner do banco, mais um ponto a vigiar
- Todos os apps e bancos dividem o mesmo servidor (memória, disco e processador)

## Alternatives Considered
- **Kubernetes:** pesado demais para um servidor e uma equipe pequena
- **Um banco para todos os apps:** um app poderia ler ou estragar os dados de outro; cópia e volta por app ficariam difíceis
- **Login próprio por app:** cada app teria de implementar e manter a entrada, e as permissões do Hub não valeriam
