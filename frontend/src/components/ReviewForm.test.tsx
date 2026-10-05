import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import type { Area, AppSummary } from "@/lib/api"
import { ana, henry, manifest, scanItem, row, ALL_CAN } from "@/test/fixtures"
import { missingForPublish, ReviewForm } from "./ReviewForm"

afterEach(() => vi.restoreAllMocks())

const RESEARCH: Area = { id: 1, name: "Research", app_count: 0, leaders: [], members: [] }
const AREAS = [RESEARCH, { ...RESEARCH, id: 2, name: "Investimentos" }]

type FormProps = Partial<React.ComponentProps<typeof ReviewForm>> & { areas?: Area[]; apps?: AppSummary[] }
function renderForm(p: FormProps = {}) {
  return render(
    <ReviewForm item={scanItem()} people={[row(henry), row(ana)]} areas={[RESEARCH]} apps={[]} me={henry} onPublished={() => {}} onSkip={() => {}} {...p} />,
  )
}

describe("missingForPublish", () => {
  test.each([
    ["complete", manifest(), 1, 0, []],
    ["no owner", manifest(), 0, 0, ["o responsável"]],
    ["same backup", manifest(), 1, 1, ["um reserva diferente do responsável"]],
    ["no entrypoint for script", manifest({ entrypoint: "" }), 1, 0, ["o arquivo que inicia o app"]],
    ["area outside the list", manifest({ area: "Marketing" }), 1, 0, ["a área"]],
    ["list without options", manifest({ inputs: [{ id: "d", label: "Docs", type: "choice", source: "window", required: true }] }), 1, 0, ['as opções de "Docs"']],
    ["documents need no entrypoint", manifest({ kind: "reference", entrypoint: "" }), 1, 0, []],
  ])("%s", (_name, m, owner, backup, want) => {
    expect(missingForPublish(m, owner, backup, AREAS)).toEqual(want)
  })
})

describe("ReviewForm", () => {
  test("shows questions, secrets and defaults the owner to me", () => {
    renderForm({ item: scanItem({ secrets: [{ name: "API_KEY", file: "config.json", line: 2, hint: "Chave" }] }) })
    expect(screen.getByText(/Confirme este ponto/)).toBeInTheDocument()
    expect(screen.getByText("config.json:2 · Chave")).toBeInTheDocument()
    expect(screen.getByLabelText("Responsável")).toHaveValue("1")
    expect(screen.getByRole("button", { name: "Publicar no catálogo" })).toBeEnabled()
  })

  test("removing an input and publishing sends the edited manifest", async () => {
    const user = userEvent.setup()
    const save = vi.spyOn(api, "saveApp").mockResolvedValue({ id: 9, folder_path: "", folder_display: "", owner_id: 1, backup_id: 2, manifest: manifest(), updated_at: "", can_run: false, version: null, folder_readable: true, remembered_folder: "", area_id: 1, restricted: false, can: ALL_CAN })
    const onPublished = vi.fn()
    renderForm({ item: scanItem({ manifest: manifest({ area: "" }) }), areas: AREAS, onPublished })

    await user.click(screen.getByRole("button", { name: "Remover Arquivo cnpjs.csv" }))
    expect(screen.queryAllByTestId("input-row")).toHaveLength(0)
    await user.selectOptions(screen.getByLabelText("Reserva"), "2")
    await user.selectOptions(screen.getByLabelText("Área"), "Investimentos")
    await user.click(screen.getByRole("button", { name: "Publicar no catálogo" }))

    expect(save).toHaveBeenCalledWith(
      expect.objectContaining({ owner_id: 1, backup_id: 2, manifest: expect.objectContaining({ inputs: [], area: "Investimentos" }) }),
    )
    expect(onPublished).toHaveBeenCalled()
  })

  test("blocks publishing without an owner and explains why", async () => {
    const user = userEvent.setup()
    renderForm({ item: scanItem(), people: [row(henry), row(ana)], onPublished: () => {} })
    await user.selectOptions(screen.getByLabelText("Responsável"), "0")
    expect(screen.getByRole("button", { name: "Publicar no catálogo" })).toBeDisabled()
    expect(screen.getByText("Falta definir o responsável.")).toBeInTheDocument()
  })

  test("shows server errors", async () => {
    const user = userEvent.setup()
    vi.spyOn(api, "saveApp").mockRejectedValue(new (await import("@/lib/api")).ApiError(409, "Já existe um FICHA.md"))
    renderForm({ item: scanItem(), people: [row(henry)], onPublished: () => {} })
    await user.click(screen.getByRole("button", { name: "Publicar no catálogo" }))
    expect(await screen.findByRole("alert")).toHaveTextContent("Já existe um FICHA.md")
  })

  test("already registered apps offer an update", () => {
    renderForm({ item: scanItem({ registered_app_id: 3, has_manifest: true }), people: [row(henry)], onPublished: () => {} })
    expect(screen.getByRole("button", { name: "Atualizar ficha" })).toBeInTheDocument()
  })

  test("blocks publishing until the area comes from the list", async () => {
    const user = userEvent.setup()
    renderForm({ item: scanItem({ manifest: manifest({ area: "Marketing" }) }), areas: [RESEARCH] })
    expect(screen.getByText(/Falta definir.*a área/)).toBeInTheDocument()
    await user.selectOptions(screen.getByLabelText("Área"), "Research")
    expect(screen.queryByText(/Falta definir/)).toBeNull()
  })
})
