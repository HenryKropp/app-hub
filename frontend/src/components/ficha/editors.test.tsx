import { useState } from "react"
import { describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import type { Input } from "@/lib/api"
import { DependsEditor, InputsEditor, NotesEditor, choiceProblems } from "./editors"
import { appSummary } from "@/test/fixtures"

function Harness({ start }: { start: Input[] }) {
  const [v, setV] = useState(start)
  return (
    <>
      <InputsEditor value={v} onChange={setV} />
      <pre data-testid="out">{JSON.stringify(v)}</pre>
    </>
  )
}

const out = () => JSON.parse(screen.getByTestId("out").textContent!)

describe("InputsEditor", () => {
  it("asks for options when the input is a list", async () => {
    render(<Harness start={[{ id: "docs", label: "Documentos", type: "file", source: "window", required: true }]} />)
    await userEvent.selectOptions(screen.getByLabelText("Tipo de Documentos"), "choice")
    await userEvent.type(screen.getByLabelText("Opções de Documentos"), "FRE{enter}DFP")
    await userEvent.click(screen.getByLabelText("Pode marcar mais de uma"))
    expect(out()[0]).toMatchObject({ type: "choice", options: ["FRE", "DFP"], multiple: true })
  })

  it("drops options when the input stops being a list", async () => {
    render(<Harness start={[{ id: "d", label: "D", type: "choice", source: "window", required: true, options: ["A"], multiple: true }]} />)
    await userEvent.selectOptions(screen.getByLabelText("Tipo de D"), "text")
    expect(out()[0].options).toBeUndefined()
    expect(out()[0].multiple).toBeUndefined()
  })
})

describe("choiceProblems", () => {
  it("names each list without options", () => {
    expect(choiceProblems([{ id: "d", label: "Docs", type: "choice", source: "window", required: true }])).toEqual(['as opções de "Docs"'])
  })
  it("ignores blank option lines", () => {
    expect(choiceProblems([{ id: "d", label: "Docs", type: "choice", source: "window", required: true, options: ["", " "] }])).toEqual(['as opções de "Docs"'])
  })
})

describe("NotesEditor", () => {
  it("keeps one note per line and drops empty lines", async () => {
    let got: string[] = []
    render(<NotesEditor value={[]} onChange={(v) => (got = v)} />)
    await userEvent.type(screen.getByLabelText("Observações"), "Precisa da VPN.{enter}{enter}Roda às 9h.")
    expect(got).toEqual(["Precisa da VPN.", "Roda às 9h."])
  })
})

describe("DependsEditor", () => {
  it("lists other apps, not itself, and keeps unknown ids", async () => {
    let got: string[] = []
    const apps = [appSummary({ id: 1, slug: "files-prep", name: "Files Prep" }), appSummary({ id: 2, slug: "eu", name: "Eu" })]
    render(<DependsEditor value={["sumiu"]} onChange={(v) => (got = v)} apps={apps} selfSlug="eu" />)
    expect(screen.queryByLabelText("Eu")).toBeNull()
    expect(screen.getByText(/sumiu/)).toBeInTheDocument()
    await userEvent.click(screen.getByLabelText("Files Prep"))
    expect(got).toEqual(["sumiu", "files-prep"])
  })
})
