import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { ApiError, api, type ArtifactDetail, type Area } from "@/lib/api"
import * as auth from "@/lib/auth"
import { bia, henry } from "@/test/fixtures"
import { NewArtifactPage } from "./NewArtifactPage"

const areas: Area[] = [
  { id: 1, name: "Comercial", app_count: 0, leaders: [], members: [] },
  { id: 2, name: "Gente", app_count: 0, leaders: [], members: [] },
]
const created = { id: 9 } as ArtifactDetail

function Where() {
  const l = useLocation()
  return <p data-testid="where">{l.pathname + l.hash}</p>
}

beforeEach(() => {
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person: henry, refresh: async () => {}, setPerson: () => {} })
  vi.spyOn(api, "areas").mockResolvedValue(areas)
  vi.spyOn(api, "artifacts").mockResolvedValue([])
})
afterEach(() => vi.restoreAllMocks())

const mount = () =>
  render(
    <MemoryRouter initialEntries={["/artifacts/novo"]}>
      <Routes>
        <Route path="/artifacts/novo" element={<NewArtifactPage />} />
        <Route path="*" element={<Where />} />
      </Routes>
    </MemoryRouter>,
  )

async function fillBasics(user: ReturnType<typeof userEvent.setup>) {
  await user.type(await screen.findByLabelText("Nome"), "Vendas")
  await user.type(screen.getByLabelText("Descrição"), "Por região")
  await user.type(screen.getByLabelText("Etiquetas"), "Vendas, mensal")
  await waitFor(() => expect(screen.getByLabelText("Área")).toHaveValue("1"))
}

test("creates a link artifact", async () => {
  const user = userEvent.setup()
  const create = vi.spyOn(api, "createArtifact").mockResolvedValue(created)
  mount()
  await fillBasics(user)
  await user.type(screen.getByLabelText("Link"), "https://claude.ai/artifact/abc")
  await user.click(screen.getByRole("button", { name: "Criar artifact" }))
  await waitFor(() => expect(create).toHaveBeenCalled())
  expect(create).toHaveBeenCalledWith({ name: "Vendas", summary: "Por região", tags: ["vendas", "mensal"], area_id: 1, kind: "link", url: "https://claude.ai/artifact/abc" })
  expect(await screen.findByTestId("where")).toHaveTextContent("/artifacts/9")
  expect(screen.getByTestId("where").textContent).not.toContain("#")
})

test("chosen viewers lands on the access section", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "createArtifact").mockResolvedValue(created)
  mount()
  await fillBasics(user)
  await user.type(screen.getByLabelText("Link"), "https://x.com/a")
  await user.click(screen.getByRole("radio", { name: "Só quem eu escolher" }))
  await user.click(screen.getByRole("button", { name: "Criar artifact" }))
  expect(await screen.findByTestId("where")).toHaveTextContent("/artifacts/9#acesso")
})

test("uploads a file as base64", async () => {
  const user = userEvent.setup()
  const create = vi.spyOn(api, "createArtifact").mockResolvedValue(created)
  mount()
  await fillBasics(user)
  await user.click(screen.getByRole("tab", { name: "Subir arquivo" }))
  await user.upload(screen.getByLabelText("Arquivo HTML"), new File(["<h1>oi</h1>"], "painel.html", { type: "text/html" }))
  await user.click(screen.getByRole("button", { name: "Criar artifact" }))
  await waitFor(() => expect(create).toHaveBeenCalled())
  expect(create).toHaveBeenCalledWith(expect.objectContaining({ kind: "file", file_name: "painel.html", content_base64: btoa("<h1>oi</h1>") }))
})

test("rejects a non-html file before sending", async () => {
  const user = userEvent.setup({ applyAccept: false })
  const create = vi.spyOn(api, "createArtifact")
  mount()
  await fillBasics(user)
  await user.click(screen.getByRole("tab", { name: "Subir arquivo" }))
  await user.upload(screen.getByLabelText("Arquivo HTML"), new File(["x"], "nota.txt", { type: "text/plain" }))
  expect(screen.getByRole("alert")).toHaveTextContent("Envie um arquivo .html.")
  await user.click(screen.getByRole("button", { name: "Criar artifact" }))
  expect(create).not.toHaveBeenCalled()
})

test("asks to confirm secrets on 409 and resends with confirm_secrets", async () => {
  const user = userEvent.setup()
  const create = vi
    .spyOn(api, "createArtifact")
    .mockRejectedValueOnce(new ApiError(409, "confirme", { code: "artifact_secrets_confirmation_required", secret_count: 2 }))
    .mockResolvedValueOnce(created)
  mount()
  await fillBasics(user)
  await user.type(screen.getByLabelText("Link"), "https://x.com/a")
  await user.click(screen.getByRole("button", { name: "Criar artifact" }))
  expect(await screen.findByText("Encontrei 2 possíveis senhas ou chaves nesse arquivo. Quem abrir o artifact poderá vê-las.")).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Enviar mesmo assim" }))
  await waitFor(() => expect(create).toHaveBeenCalledTimes(2))
  expect(create.mock.calls[1][0]).toEqual(expect.objectContaining({ confirm_secrets: true }))
})

test("shows the 422 message", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "createArtifact").mockRejectedValue(new ApiError(422, "Tem senha no arquivo.", { code: "artifact_secrets_blocked", secret_count: 1, error: "Tem senha no arquivo." }))
  mount()
  await fillBasics(user)
  await user.type(screen.getByLabelText("Link"), "https://x.com/a")
  await user.click(screen.getByRole("button", { name: "Criar artifact" }))
  expect(await screen.findByRole("alert")).toHaveTextContent("Tem senha no arquivo.")
})

test("people who cannot register see a message", async () => {
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person: bia, refresh: async () => {}, setPerson: () => {} })
  mount()
  expect(screen.getByRole("link", { name: "Voltar aos artifacts" })).toHaveAttribute("href", "/artifacts")
})
