import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { ana, henry, manifest, scanItem } from "@/test/fixtures"
import { missingForPublish, ReviewForm } from "./ReviewForm"

afterEach(() => vi.restoreAllMocks())

describe("missingForPublish", () => {
  test.each([
    ["complete", manifest(), 1, 0, []],
    ["no owner", manifest(), 0, 0, ["o responsável"]],
    ["same backup", manifest(), 1, 1, ["um reserva diferente do responsável"]],
    ["no entrypoint for script", manifest({ entrypoint: "" }), 1, 0, ["o arquivo que inicia o app"]],
    ["documents need no entrypoint", manifest({ kind: "reference", entrypoint: "" }), 1, 0, []],
  ])("%s", (_name, m, owner, backup, want) => {
    expect(missingForPublish(m, owner, backup)).toEqual(want)
  })
})

describe("ReviewForm", () => {
  test("shows questions, secrets and defaults the owner to me", () => {
    render(
      <ReviewForm
        item={scanItem({ secrets: [{ name: "API_KEY", file: "config.json", line: 2, hint: "Chave" }] })}
        people={[henry, ana]}
        me={henry}
        onPublished={() => {}}
        onSkip={() => {}}
      />,
    )
    expect(screen.getByText(/Confirme este ponto/)).toBeInTheDocument()
    expect(screen.getByText("config.json:2 · Chave")).toBeInTheDocument()
    expect(screen.getByLabelText("Responsável")).toHaveValue("1")
    expect(screen.getByRole("button", { name: "Publicar no catálogo" })).toBeEnabled()
  })

  test("removing an input and publishing sends the edited manifest", async () => {
    const user = userEvent.setup()
    const save = vi.spyOn(api, "saveApp").mockResolvedValue({ id: 9, folder_path: "", folder_display: "", owner_id: 1, backup_id: 2, manifest: manifest(), updated_at: "", can_run: false, version: null, folder_readable: true, remembered_folder: "" })
    const onPublished = vi.fn()
    render(<ReviewForm item={scanItem()} people={[henry, ana]} me={henry} onPublished={onPublished} onSkip={() => {}} />)

    await user.click(screen.getByRole("button", { name: "Remover Arquivo cnpjs.csv" }))
    expect(screen.queryAllByTestId("input-row")).toHaveLength(0)
    await user.selectOptions(screen.getByLabelText("Reserva"), "2")
    await user.type(screen.getByLabelText("Área"), "Investimentos")
    await user.click(screen.getByRole("button", { name: "Publicar no catálogo" }))

    expect(save).toHaveBeenCalledWith(
      expect.objectContaining({ owner_id: 1, backup_id: 2, manifest: expect.objectContaining({ inputs: [], area: "Investimentos" }) }),
    )
    expect(onPublished).toHaveBeenCalled()
  })

  test("blocks publishing without an owner and explains why", async () => {
    const user = userEvent.setup()
    render(<ReviewForm item={scanItem()} people={[henry, ana]} me={henry} onPublished={() => {}} onSkip={() => {}} />)
    await user.selectOptions(screen.getByLabelText("Responsável"), "0")
    expect(screen.getByRole("button", { name: "Publicar no catálogo" })).toBeDisabled()
    expect(screen.getByText("Falta definir o responsável.")).toBeInTheDocument()
  })

  test("shows server errors", async () => {
    const user = userEvent.setup()
    vi.spyOn(api, "saveApp").mockRejectedValue(new (await import("@/lib/api")).ApiError(409, "Já existe um FICHA.md"))
    render(<ReviewForm item={scanItem()} people={[henry]} me={henry} onPublished={() => {}} onSkip={() => {}} />)
    await user.click(screen.getByRole("button", { name: "Publicar no catálogo" }))
    expect(await screen.findByRole("alert")).toHaveTextContent("Já existe um FICHA.md")
  })

  test("already registered apps offer an update", () => {
    render(<ReviewForm item={scanItem({ registered_app_id: 3, has_manifest: true })} people={[henry]} me={henry} onPublished={() => {}} onSkip={() => {}} />)
    expect(screen.getByRole("button", { name: "Atualizar ficha" })).toBeInTheDocument()
  })
})
