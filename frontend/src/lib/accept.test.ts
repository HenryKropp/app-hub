import { expect, test } from "vitest"
import { acceptText, acceptsFile, parseAccept } from "./accept"

test("parseAccept normalizes what people type", () => {
  expect(parseAccept("txt, .CSV; *.md  .txt")).toEqual([".txt", ".csv", ".md"])
  expect(parseAccept("  ")).toEqual([])
  expect(parseAccept("pdf, não-é-tipo")).toEqual([".pdf"])
})

test("acceptText and acceptsFile", () => {
  expect(acceptText([".txt"])).toBe(".txt")
  expect(acceptText([".txt", ".csv", ".md"])).toBe(".txt, .csv ou .md")
  expect(acceptsFile([".txt"], "a.TXT")).toBe(true)
  expect(acceptsFile([".txt"], "a.pdf")).toBe(false)
  expect(acceptsFile([".txt"], "txt")).toBe(false)
  expect(acceptsFile([], "a.pdf")).toBe(true)
})
