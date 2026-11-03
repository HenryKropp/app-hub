"""Olá Ziv: o teste mais simples. Diz oi e salva um arquivo com o seu nome."""
import os
from datetime import datetime
from pathlib import Path

nome = os.environ.get("APP_HUB_INPUT_NOME", "").strip() or "pessoa sem nome"
saida = Path(os.environ.get("APP_HUB_OUTPUT_DIR", "."))
agora = datetime.now().strftime("%d/%m/%Y às %H:%M")

print(f"Olá, {nome}! O Ziv rodou este app em {agora}.")
(saida / "ola.txt").write_text(f"Olá, {nome}!\nEste arquivo foi criado pelo Ziv em {agora}.\n", encoding="utf-8")
print("Arquivo ola.txt criado.")
