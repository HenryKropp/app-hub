"""Teste de erro: falha de propósito, para ver como o Ziv mostra um erro."""
import os
import sys
from pathlib import Path

saida = Path(os.environ.get("APP_HUB_OUTPUT_DIR", "."))
(saida / "antes-do-erro.txt").write_text("Este arquivo foi criado antes do erro.\n", encoding="utf-8")
print("Comecei normalmente e criei antes-do-erro.txt.")
print("Agora vou falhar de propósito.", file=sys.stderr)
sys.exit(1)
