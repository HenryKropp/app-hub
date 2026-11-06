import { expect, test } from "vitest"
import { describeFrequency, upcoming, frequencyProblem } from "./schedules"

// 2026-11-06 10:02 in Brasília (UTC-3).
const at = (s: string) => new Date(`${s}-03:00`)

test("describes frequencies like the server", () => {
  expect(describeFrequency({ kind: "minutes", every: 5 })).toBe("A cada 5 minutos")
  expect(describeFrequency({ kind: "hours", every: 1 })).toBe("A cada hora")
  expect(describeFrequency({ kind: "daily", at: "07:00" })).toBe("Todo dia às 7h")
  expect(describeFrequency({ kind: "weekdays", at: "07:30", days: [5, 1, 3] })).toBe("Seg, qua e sex às 7h30")
  expect(describeFrequency({ kind: "monthly", at: "09:00", day: 5 })).toBe("Todo mês no dia 5 às 9h")
})

test("lists the next run times in Brasília time", () => {
  expect(upcoming({ kind: "minutes", every: 5 }, at("2026-11-06T10:02:00"), 2).map((d) => d.toISOString()))
    .toEqual([at("2026-11-06T10:05:00").toISOString(), at("2026-11-06T10:10:00").toISOString()])
  expect(upcoming({ kind: "weekdays", at: "07:30", days: [1, 3, 5] }, at("2026-11-06T08:00:00"), 1)[0].toISOString())
    .toBe(at("2026-11-09T07:30:00").toISOString())
  expect(upcoming({ kind: "monthly", at: "09:00", day: 28 }, at("2027-01-28T10:00:00"), 1)[0].toISOString())
    .toBe(at("2027-02-28T09:00:00").toISOString())
})

test("explains invalid frequencies", () => {
  expect(frequencyProblem({ kind: "minutes", every: 4 })).toBe("No mínimo a cada 5 minutos.")
  expect(frequencyProblem({ kind: "weekdays", at: "07:00", days: [] })).toBe("Escolha pelo menos um dia da semana.")
  expect(frequencyProblem({ kind: "daily", at: "07:00" })).toBe("")
})
