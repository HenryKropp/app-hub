---
name: Visão do marketplace
description: Direção de produto declarada pelo usuário para o compartilhamento de microapps.
---

O usuário descreveu a ideia como “um hub de micro apps entre a empresa (e eventualmente publico)” e confirmou que a abertura ao público significa “Outras empresas adotam os microapps”.
Para as empresas clientes, escolheu “Um portal multiempresa com ambientes separados (recomendado)”, e pediu atenção a segurança e compliance entre empresas e na distribuição externa, incluindo senhas e dados.
Ao atualizar o projeto a partir do GitHub, escolheu “Integrar preservando o multiempresa (recomendado)”, em vez de substituir esta versão pela versão do GitHub.

**Why:** direção e modelo de produto confirmados pelo usuário; priorizar a separação entre empresas e os controles de dados antes de abrir a distribuição a terceiros.

**How to apply:** desenvolver o portal multiempresa como base e considerar compartilhamento interno seguido de marketplace B2B. O público-alvo externo são outras empresas adotando microapps, não visitantes anônimos executando aplicativos. Não presumir monetização, publicação aberta de código nem conformidade jurídica apenas pela implementação técnica.

## Entrada no produto

Para a entrada nas empresas, escolheu “Convite ou pedido de acesso”, além do cadastro de uma nova empresa.

**Why:** o usuário quer que quem chega ao portal consiga cadastrar a própria empresa ou pedir entrada em uma existente, sem perder a separação entre empresas.

**How to apply:** manter ambos os caminhos de adesão; pedidos públicos exigem autorização da empresa e não devem conceder acesso automaticamente.

O usuário rejeitou a página inicial com os formulários de acesso e cadastro juntos: “de tudo, tente se inspirar em empresas de verdade. melhor q tenha um botão login/cadastre-se por exemplo e depois joga pra outra tela”.

**Why:** o usuário pediu uma apresentação de produto antes dos formulários, com referências reais de empresas.

**How to apply:** manter a página inicial sem formulários e encaminhar pelo botão de login/cadastro para telas próprias. Não voltar a juntar cadastro da empresa e acesso na apresentação.

O usuário confirmou que os microapps “precisam de arquivos locais”.

**Why:** requisito declarado pelo usuário ao discutir o propósito do agente Windows.

**How to apply:** preservar o uso de arquivos locais ao avaliar alternativas de execução. Ainda não foi esclarecido se basta selecionar e enviar arquivos pelo navegador ou se é necessário acessar pastas diretamente; não presumir essa distinção resolvida.
