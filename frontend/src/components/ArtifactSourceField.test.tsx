import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { useState } from "react"
import { expect, test } from "vitest"
import { ArtifactSourceField, isClaudeLink, type ArtifactSource } from "./ArtifactSourceField"

test("isClaudeLink recognises Claude hosts only", () => {
  expect(isClaudeLink("https://claude.ai/public/artifacts/abc")).toBe(true)
  expect(isClaudeLink(" https://claude.site/artifacts/abc ")).toBe(true)
  expect(isClaudeLink("https://www.claude.ai/x")).toBe(true)
  expect(isClaudeLink("https://notclaude.ai/x")).toBe(false)
  expect(isClaudeLink("https://claude.ai.evil.com/x")).toBe(false)
  expect(isClaudeLink("não é link")).toBe(false)
})

function Harness() {
  const [v, setV] = useState<ArtifactSource>({ kind: "link", url: "" })
  return <ArtifactSourceField value={v} onChange={setV} />
}

test("pasting a Claude link suggests uploading the HTML instead", async () => {
  const user = userEvent.setup()
  render(<Harness />)
  await user.type(screen.getByLabelText("Link"), "https://exemplo.com/painel")
  expect(screen.queryByText(/Links do Claude/)).toBeNull()
  await user.clear(screen.getByLabelText("Link"))
  await user.type(screen.getByLabelText("Link"), "https://claude.ai/public/artifacts/abc")
  expect(screen.getByText(/Links do Claude não aparecem dentro do Ziv/)).toBeInTheDocument()
})
