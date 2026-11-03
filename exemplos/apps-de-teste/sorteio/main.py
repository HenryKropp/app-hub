"""Sorteio da equipe: divide uma lista de nomes em grupos aleatórios."""
import os
import random
from pathlib import Path

nomes = [n.strip() for n in os.environ.get("APP_HUB_INPUT_NOMES", "").replace("\n", ",").split(",") if n.strip()]
grupos = int(os.environ.get("APP_HUB_INPUT_GRUPOS", "2") or 2)
saida = Path(os.environ.get("APP_HUB_OUTPUT_DIR", "."))
if len(nomes) < grupos:
    raise SystemExit(f"Preciso de pelo menos {grupos} nomes para formar {grupos} grupos.")

random.shuffle(nomes)
resultado = []
for i in range(grupos):
    resultado.append(f"Grupo {i + 1}: " + ", ".join(nomes[i::grupos]))
(saida / "grupos.txt").write_text("\n".join(resultado) + "\n", encoding="utf-8")
print("\n".join(resultado))
