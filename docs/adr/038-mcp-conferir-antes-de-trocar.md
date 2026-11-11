# 038 - App com tela: conferir a versão nova antes de trocar (F4a + F4c, parte)

**Status:** Accepted

## Context
Antes, uma versão nova de app com tela entrava no ar assim que respondia na porta. Uma página quebrada (erro tratado, página em branco, rota errada) ainda respondia, então tomava o lugar da versão boa. O adendo 3 do plano pede que cada app diga como se testa e que o Ziv ensaie antes de trocar.

## Decision
- A ficha do app com tela ganha `check_text`, um texto que a página deve mostrar. A página é o `health_path`, que já existia.
- Ao testar ou publicar, depois que o programa responde, o Ziv lê a página (sem JavaScript, até 1 MB) e só segue se o texto aparecer. Ele tenta até o limite de tempo de sempre. Se o texto não aparecer, a publicação falha com "a versão nova abriu, mas a página X não mostra o texto esperado …", e a versão anterior segue no ar.
- Ligar uma versão já conhecida (acordar, voltar para a anterior) não confere o texto.
- `publicar_app_com_tela` aceita `conferir_texto` e `conferir_caminho`; texto vazio deixa de conferir. O protocolo e a receita mandam combinar o texto com a pessoa na primeira publicação.
- O site guarda o campo ao salvar as outras configurações, mas ainda não mostra nem edita.

## Rationale
O Ziv já ligava a versão nova ao lado da que está no ar antes de trocar. Faltava só um critério melhor que "respondeu". Um texto esperado é simples de explicar à pessoa e não pede schema nem tela nova.

## Trade-offs
**Pros:**
- Uma versão que abre mas mostra a página errada não derruba a boa.
- Nada muda para apps sem o texto.

**Cons:**
- Não serve para Streamlit, cuja página só aparece com JavaScript. Para esses apps vale a checagem de sempre.
- Ainda não há "como testar" para scripts (entradas de exemplo + ensaio), nem campo na tela do site.

## Alternatives Considered
- Abrir a página num navegador de verdade para rodar o JavaScript: descartado por agora. O servidor tem 1 GB, e um navegador pesa.
- Uma tabela nova com receitas de teste: descartada. O campo cabe na ficha, junto do `health_path`.
