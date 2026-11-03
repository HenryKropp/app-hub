"""Tarefa demorada: conta devagar, para testar o acompanhamento e o botão Parar."""
import os
import time
from pathlib import Path

segundos = int(float(os.environ.get("APP_HUB_INPUT_SEGUNDOS", "90") or 90))
saida = Path(os.environ.get("APP_HUB_OUTPUT_DIR", "."))
for s in range(1, segundos + 1):
    if s % 10 == 0 or s == 1:
        print(f"{s} de {segundos} segundos...", flush=True)
    time.sleep(1)
(saida / "terminou.txt").write_text(f"Contei {segundos} segundos até o fim.\n", encoding="utf-8")
print("Terminei.")
