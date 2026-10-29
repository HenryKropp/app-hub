import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, expect, it, vi } from "vitest"
import { api, ApiError } from "@/lib/api"
import { toast } from "sonner"
import { PublishBar } from "./Publish"

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

beforeEach(() => vi.restoreAllMocks())

it("publishes and follows the progress until it is live", async () => {
  vi.spyOn(api.shared, "publish").mockResolvedValue({ id: 7, kind: "live", status: "building" } as never)
  const dep = vi.spyOn(api.shared, "deployment")
    .mockResolvedValueOnce({ id: 7, kind: "live", status: "building" } as never)
    .mockResolvedValueOnce({ id: 7, kind: "live", status: "live" } as never)
  const onDone = vi.fn()
  // 100 ms between looks, so "Publicando…" is still there when the test looks for it on a busy machine.
  render(<PublishBar appId={1} ready live={false} latest={3} liveVersion={undefined} hasPrevious={false} interval={100} onDone={onDone} />)
  await userEvent.click(screen.getByRole("button", { name: "Publicar" }))
  expect(await screen.findByText("Publicando…")).toBeInTheDocument()
  await waitFor(() => expect(onDone).toHaveBeenCalled())
  expect(dep).toHaveBeenCalledWith(1, 7)
  expect(toast.success).toHaveBeenCalledWith("No ar.")
})

it("offers a new version only when there is one", () => {
  render(<PublishBar appId={1} ready live latest={4} liveVersion={4} hasPrevious interval={10} onDone={() => {}} />)
  expect(screen.queryByRole("button", { name: "Publicar versão nova" })).not.toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Voltar para a anterior" })).toBeInTheDocument()
})

it("shows the failure in plain words and keeps the old version", async () => {
  vi.spyOn(api.shared, "publish").mockResolvedValue({ id: 8, kind: "live", status: "building" } as never)
  vi.spyOn(api.shared, "deployment").mockResolvedValue({ id: 8, kind: "live", status: "failed", error_message: "A versão nova não respondeu. A anterior continua no ar." } as never)
  render(<PublishBar appId={1} ready live latest={5} liveVersion={4} hasPrevious interval={10} onDone={() => {}} />)
  await userEvent.click(screen.getByRole("button", { name: "Publicar versão nova" }))
  expect(await screen.findByText("A versão nova não respondeu. A anterior continua no ar.")).toBeInTheDocument()
  // Whoever does not see the technical part gets no log, so no link to it.
  expect(screen.queryByRole("button", { name: "Ver detalhes" })).not.toBeInTheDocument()
})

it("shows the build log behind Ver detalhes when it came", async () => {
  vi.spyOn(api.shared, "publish").mockResolvedValue({ id: 8, kind: "live", status: "building" } as never)
  vi.spyOn(api.shared, "deployment").mockResolvedValue({ id: 8, kind: "live", status: "failed", error_message: "Não deu para montar o app.", build_log: "pip: no matching distribution" } as never)
  render(<PublishBar appId={1} ready live={false} latest={1} liveVersion={undefined} hasPrevious={false} interval={10} onDone={() => {}} />)
  await userEvent.click(screen.getByRole("button", { name: "Publicar" }))
  await userEvent.click(await screen.findByRole("button", { name: "Ver detalhes" }))
  expect(await screen.findByText("pip: no matching distribution")).toBeInTheDocument()
})

it("builds a test box and offers to open it", async () => {
  vi.spyOn(api.shared, "test").mockResolvedValue({ id: 9, kind: "test", status: "building" } as never)
  vi.spyOn(api.shared, "deployment").mockResolvedValue({ id: 9, kind: "test", status: "ready" } as never)
  render(<PublishBar appId={1} ready live={false} latest={1} liveVersion={undefined} hasPrevious={false} slug="notas" interval={10} onDone={() => {}} />)
  await userEvent.click(screen.getByRole("button", { name: "Testar" }))
  const link = await screen.findByRole("link", { name: /Abrir o teste/ })
  expect(link).toHaveAttribute("href", "/api/shared/authorize?app=notas&teste=1&volta=%2F")
  expect(link).toHaveAttribute("target", "_blank")
  expect(screen.getByText("O teste fica aberto por 30 minutos, com dados separados.")).toBeInTheDocument()
})

it("shows the server's sentence when the app is busy, without retrying", async () => {
  const pub = vi.spyOn(api.shared, "publish").mockRejectedValue(new ApiError(409, "Já tem uma mudança em andamento neste app. Espere terminar e tente de novo."))
  render(<PublishBar appId={1} ready live={false} latest={1} liveVersion={undefined} hasPrevious={false} interval={10} onDone={() => {}} />)
  await userEvent.click(screen.getByRole("button", { name: "Publicar" }))
  await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Já tem uma mudança em andamento neste app. Espere terminar e tente de novo."))
  expect(pub).toHaveBeenCalledTimes(1)
  expect(screen.getByRole("button", { name: "Publicar" })).toBeEnabled()
})

it("asks about the data before going back", async () => {
  const rb = vi.spyOn(api.shared, "rollback").mockResolvedValue({})
  render(<PublishBar appId={1} ready live latest={5} liveVersion={5} hasPrevious interval={10} onDone={() => {}} />)
  await userEvent.click(screen.getByRole("button", { name: "Voltar para a anterior" }))
  await userEvent.click(screen.getByRole("radio", { name: /voltar os dados também/i }))
  await userEvent.click(screen.getByRole("button", { name: "Voltar" }))
  expect(rb).toHaveBeenCalledWith(1, true)
})

it("goes back without the data by default", async () => {
  const rb = vi.spyOn(api.shared, "rollback").mockResolvedValue({})
  render(<PublishBar appId={1} ready live latest={5} liveVersion={5} hasPrevious interval={10} onDone={() => {}} />)
  await userEvent.click(screen.getByRole("button", { name: "Voltar para a anterior" }))
  expect(screen.getByRole("radio", { name: /só a versão/i })).toBeChecked()
  await userEvent.click(screen.getByRole("button", { name: "Voltar" }))
  expect(rb).toHaveBeenCalledWith(1, false)
})

it("disables publishing while the check has pending items", () => {
  render(<PublishBar appId={1} ready={false} live={false} latest={1} liveVersion={undefined} hasPrevious={false} interval={10} onDone={() => {}} />)
  expect(screen.getByRole("button", { name: "Publicar" })).toBeDisabled()
  expect(screen.getByRole("button", { name: "Testar" })).toBeDisabled()
  expect(screen.getByText("Resolva a checagem primeiro.")).toBeInTheDocument()
})

it("only Publicar is coral; Testar is a neutral button", async () => {
  render(<PublishBar appId={1} ready live={false} latest={1} liveVersion={undefined} hasPrevious={false} interval={10} onDone={() => {}} />)
  expect(screen.getByRole("button", { name: "Testar" }).className).not.toContain("bg-brand")
  expect(screen.getByRole("button", { name: "Publicar" }).className).toContain("bg-brand")
})
