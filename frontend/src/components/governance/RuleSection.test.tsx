import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, expect, test, vi } from "vitest"
import { api, type ItemGovernance } from "@/lib/api"
import { RuleSection } from "./RuleSection"

afterEach(() => vi.restoreAllMocks())

const gov = (o: Partial<ItemGovernance> = {}): ItemGovernance => ({
  rule_company: "risk", rule_item: "", rule_effective: "risk", it_on: false, open_request: null, history: [],
  can_loosen: false, can_change: true, ...o,
})

test("without can_loosen only stricter rules are offered", () => {
  render(<RuleSection kind="app" id={4} gov={gov()} onChanged={() => {}} />)
  const select = screen.getByLabelText("Versão nova precisa de aprovação")
  const labels = Array.from((select as HTMLSelectElement).options).map((o) => o.text)
  expect(labels).toEqual(["Igual à empresa (só quando muda o risco)", "Toda versão"])
})

test("the admin also sees lighter rules and saves the choice", async () => {
  const set = vi.spyOn(api, "setAppRule").mockResolvedValue(gov({ rule_item: "first", rule_effective: "first", can_loosen: true }))
  const changed = vi.fn()
  render(<RuleSection kind="app" id={4} gov={gov({ can_loosen: true })} onChanged={changed} />)
  await userEvent.selectOptions(screen.getByLabelText("Versão nova precisa de aprovação"), "first")
  expect(set).toHaveBeenCalledWith(4, "first")
  expect(changed).toHaveBeenCalled()
})

test("who can't change it only reads the rule", () => {
  render(<RuleSection kind="app" id={4} gov={gov({ can_change: false, rule_item: "every", rule_effective: "every" })} onChanged={() => {}} />)
  expect(screen.queryByLabelText("Versão nova precisa de aprovação")).not.toBeInTheDocument()
  expect(screen.getByText(/Toda versão/)).toBeInTheDocument()
})
