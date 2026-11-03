"""Registra os apps de teste no Ziv local (servidor com DEV_MODE=1).

Uso: python3 importar.py [endereço] [e-mail]
Padrão: http://localhost:5173 e o e-mail de Henry. Apps que já existem são pulados.
Empresas criadas pelo cadastro (/w/<empresa>) não aceitam o login de desenvolvimento:
passe uma sessão válida em ZIV_SESSAO (valor do cookie hub_session).
"""
import os
import base64
import http.cookiejar
import json
import sys
import urllib.error
import urllib.request
from pathlib import Path

BASE = sys.argv[1] if len(sys.argv) > 1 else "http://localhost:5173"
EMAIL = sys.argv[2] if len(sys.argv) > 2 else "henry.kropp@alisinvestimentos.com"
AQUI = Path(__file__).parent


def entrada(id_, label, type_="text", required=True, options=None):
    return {"id": id_, "label": label, "type": type_, "source": "argument", "required": required, "options": options or [], "multiple": False}


APPS = [
    ("ola-ziv", "Olá Ziv", "Administrativo", "O teste mais simples: diz oi e salva um arquivo com o seu nome.",
     [entrada("nome", "Seu nome")], [{"label": "Arquivo de boas-vindas", "pattern": "ola.txt"}]),
    ("juros", "Simulador de juros compostos", "Investimentos", "Mostra mês a mês quanto um valor rende com juros compostos e salva a tabela para o Excel.",
     [entrada("valor", "Valor inicial (R$)", "number"), entrada("taxa", "Taxa ao mês (%)", "number"), entrada("meses", "Quantos meses", "number")],
     [{"label": "Tabela mês a mês", "pattern": "simulacao.csv"}]),
    ("contador", "Contador de palavras", "Research", "Lê um arquivo de texto e lista as palavras mais usadas.",
     [entrada("arquivo", "Arquivo de texto (.txt, .csv, .md)", "file")], [{"label": "Contagem", "pattern": "contagem.txt"}]),
    ("sorteio", "Sorteio da equipe", "Administrativo", "Divide uma lista de nomes em grupos aleatórios.",
     [entrada("nomes", "Nomes, separados por vírgula"), entrada("grupos", "Quantos grupos", "choice", options=["2", "3", "4"])],
     [{"label": "Grupos sorteados", "pattern": "grupos.txt"}]),
    ("vencimentos", "Calendário de vencimentos", "Operações", "Gera as datas das parcelas mês a mês, pulando sábados e domingos.",
     [entrada("inicio", "Primeiro vencimento", "date"), entrada("parcelas", "Quantas parcelas", "number")],
     [{"label": "Datas das parcelas", "pattern": "vencimentos.csv"}]),
    ("erro", "Teste de erro", "Compliance", "Falha de propósito, para ver como o Ziv mostra um erro.", [], []),
    ("demorada", "Tarefa demorada", "Operações", "Conta devagar por alguns segundos, para testar o acompanhamento e o botão Parar.",
     [entrada("segundos", "Quantos segundos", "number", required=False)], [{"label": "Aviso de fim", "pattern": "terminou.txt"}]),
]

jar = http.cookiejar.CookieJar()
web = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))


def chamar(method, path, body=None):
    headers = {"Content-Type": "application/json", "Origin": BASE.split("/w/")[0]}
    if os.environ.get("ZIV_SESSAO"):
        headers["Cookie"] = "hub_session=" + os.environ["ZIV_SESSAO"]
    req = urllib.request.Request(BASE + path, method=method, data=json.dumps(body).encode() if body is not None else None, headers=headers)
    with web.open(req) as r:
        return json.loads(r.read() or b"null")


if not os.environ.get("ZIV_SESSAO"):
    chamar("POST", "/api/dev/login", {"email": EMAIL})
existentes = {a["manifest"]["name"] if "manifest" in a else a.get("name") for a in chamar("GET", "/api/apps")}
for pasta, nome, area, resumo, inputs, outputs in APPS:
    if nome in existentes:
        print(f"já existe: {nome}")
        continue
    codigo = (AQUI / pasta / "main.py").read_bytes()
    manifest = {"manifest_version": 1, "id": "", "name": nome, "summary": resumo, "kind": "script", "language": "python", "entrypoint": "main.py",
                "run": {"command": "", "url": ""}, "runtime": {"language": "python", "dependencies_file": "", "dependencies": None},
                "inputs": inputs, "outputs": outputs, "secrets": None, "schedule": "", "area": area, "owner": None, "backup": None,
                "tags": ["teste"], "notes": ["App de teste criado a partir de exemplos/apps-de-teste."]}
    try:
        criado = chamar("POST", "/api/scripts/import", {"manifest": manifest, "files": {"main.py": {"name": "main.py", "content_base64": base64.b64encode(codigo).decode()}}})
    except urllib.error.HTTPError as e:
        print(f"ERRO em {nome}: {e.code} {e.read().decode()}")
        continue
    print(f"registrado: {nome} (id {criado['id']})")
