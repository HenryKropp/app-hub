import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, expect, test, vi } from "vitest"
import { api, type VersionInfo } from "@/lib/api"
import { ana, henry } from "@/test/fixtures"
import { DescribeDialog } from "./DescribeDialog"

afterEach(() => vi.restoreAllMocks())

const version: VersionInfo = {
  number: 2, origin: "folder", restored_from: null, author_name: "Ana", author_probable: false, description: "",
  summary: { added: 0, removed: 0, modified: 1, binaries: [] }, files_changed_at: null, created_at: "2026-10-01T12:05:00Z", current: true,
}

test("preselects the author and sends author_id null for Não sei", async () => {
  vi.spyOn(api, "people").mockResolvedValue([henry, ana])
  const save = vi.spyOn(api, "describeVersion").mockResolvedValue(version)
  const user = userEvent.setup()
  render(<DescribeDialog appId={3} version={version} open onOpenChange={() => {}} onSaved={() => {}} />)
  const select = await screen.findByLabelText("Quem fez a mudança?")
  await vi.waitFor(() => expect(select).toHaveValue("2"))
  await user.selectOptions(select, "Não sei")
  await user.click(screen.getByRole("button", { name: "Salvar" }))
  expect(save).toHaveBeenCalledWith(3, 2, { description: "", author_id: null })
})
