"""Simulador de juros compostos: mostra mês a mês quanto um valor rende."""
import csv
import os
from pathlib import Path


def numero(id_, padrao):
    texto = os.environ.get(f"APP_HUB_INPUT_{id_}", "").strip().replace(",", ".")
    return float(texto) if texto else padrao


def reais(v):
    return "R$ " + f"{v:,.2f}".replace(",", "_").replace(".", ",").replace("_", ".")


valor = numero("VALOR", 1000.0)
taxa = numero("TAXA", 1.0) / 100
meses = int(numero("MESES", 12))
saida = Path(os.environ.get("APP_HUB_OUTPUT_DIR", "."))

with open(saida / "simulacao.csv", "w", newline="", encoding="utf-8-sig") as f:
    escrita = csv.writer(f, delimiter=";")
    escrita.writerow(["Mês", "Saldo (R$)", "Rendimento no mês (R$)"])
    saldo = valor
    for mes in range(1, meses + 1):
        rendimento = saldo * taxa
        saldo += rendimento
        escrita.writerow([mes, f"{saldo:.2f}".replace(".", ","), f"{rendimento:.2f}".replace(".", ",")])

print(f"{reais(valor)} a {str(round(taxa * 100, 2)).replace('.', ',')}% ao mês por {meses} meses viram {reais(saldo)}.")
print("Tabela salva em simulacao.csv (abre no Excel).")
