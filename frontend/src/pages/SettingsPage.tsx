import { PageHeader, ThemeToggle } from "@/components/bits"
import { ComputerSection } from "./ComputerSection"

export function SettingsPage() {
  return (
    <>
      <PageHeader title="Configurações" />
      <div className="flex flex-col gap-12">
        <ComputerSection />
        <section aria-labelledby="aparencia">
          <h2 id="aparencia" className="font-heading text-lg">Aparência</h2>
          <p className="mt-1 mb-5 max-w-[68ch] text-muted-foreground">Vale só para este navegador.</p>
          <div className="max-w-md">
            <ThemeToggle size="lg" />
          </div>
        </section>
      </div>
    </>
  )
}
