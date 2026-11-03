"""Calendário de vencimentos: gera as datas das parcelas, pulando fins de semana."""
import csv
import os
from datetime import date, datetime, timedelta
from pathlib import Path


def ler_data(texto):
    for formato in ("%Y-%m-%d", "%d/%m/%Y"):
        try:
            return datetime.strptime(texto.strip(), formato).date()
        except ValueError:
            pass
    return date.today()


def mais_meses(d, n):
    mes = d.month - 1 + n
    ano, mes = d.year + mes // 12, mes % 12 + 1
    for dia in (d.day, 30, 29, 28):
        try:
            return date(ano, mes, dia)
        except ValueError:
            continue


inicio = ler_data(os.environ.get("APP_HUB_INPUT_INICIO", ""))
parcelas = int(float(os.environ.get("APP_HUB_INPUT_PARCELAS", "12") or 12))
saida = Path(os.environ.get("APP_HUB_OUTPUT_DIR", "."))
dias = ["segunda", "terça", "quarta", "quinta", "sexta", "sábado", "domingo"]

with open(saida / "vencimentos.csv", "w", newline="", encoding="utf-8-sig") as f:
    escrita = csv.writer(f, delimiter=";")
    escrita.writerow(["Parcela", "Vencimento", "Dia da semana"])
    for i in range(parcelas):
        d = mais_meses(inicio, i)
        while d.weekday() >= 5:
            d += timedelta(days=1)
        escrita.writerow([i + 1, d.strftime("%d/%m/%Y"), dias[d.weekday()]])
        print(f"Parcela {i + 1}: {d.strftime('%d/%m/%Y')} ({dias[d.weekday()]})")
print("Datas salvas em vencimentos.csv.")
