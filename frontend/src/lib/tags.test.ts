import { expect, test } from "vitest"
import { parseTags } from "./tags"

test("parseTags trims, lowercases, drops empties and dedupes", () => {
  expect(parseTags(" Vendas, semanal ,,vendas")).toEqual(["vendas", "semanal"])
  expect(parseTags("")).toEqual([])
})
