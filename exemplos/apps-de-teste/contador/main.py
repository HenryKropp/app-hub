"""Contador de palavras: lê um arquivo de texto e lista as palavras mais usadas."""
import os
import re
from collections import Counter
from pathlib import Path

arquivo = Path(os.environ.get("APP_HUB_INPUT_ARQUIVO", ""))
saida = Path(os.environ.get("APP_HUB_OUTPUT_DIR", "."))
if not arquivo.is_file() or arquivo.suffix.lower() not in {".txt", ".csv", ".md"}:
    raise SystemExit("Escolha um arquivo de texto (.txt, .csv ou .md). PDF, Word e Excel não servem.")

texto = arquivo.read_text(encoding="utf-8", errors="replace")
palavras = [p for p in re.findall(r"[^\W\d_]{3,}", texto.lower())]
mais = Counter(palavras).most_common(20)

linhas = [f"Arquivo: {arquivo.name}", f"Total de palavras: {len(palavras)}", f"Palavras diferentes: {len(set(palavras))}", "", "As 20 mais usadas:"]
linhas += [f"{i:>2}. {p} ({n})" for i, (p, n) in enumerate(mais, 1)]
(saida / "contagem.txt").write_text("\n".join(linhas) + "\n", encoding="utf-8")
print("\n".join(linhas))
