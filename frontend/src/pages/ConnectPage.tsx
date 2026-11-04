import { useEffect, useState } from "react"
import { Link, useNavigate, useSearchParams } from "react-router-dom"
import { Laptop } from "lucide-react"
import { Button } from "@/components/ui/button"
import { api, ApiError, type Pairing } from "@/lib/api"

export function ConnectPage() {
  const [params] = useSearchParams()
  const code = params.get("codigo") ?? ""
  const navigate = useNavigate()
  const [pairing, setPairing] = useState<Pairing | null>(null)
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const [loadFailed, setLoadFailed] = useState(false)

  useEffect(() => {
    api.pairing(code).then(setPairing, (e: Error) => { setLoadFailed(!code || (e instanceof ApiError && (e.status === 400 || e.status === 404))); setError(e.message || "Algo deu errado. Tente de novo; se continuar, avise quem administra o Ziv.") })
  }, [code])

  const approve = async () => {
    setBusy(true)
    setLoadFailed(false)
    try {
      await api.approvePairing(code)
      navigate("/configuracoes")
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível conectar.")
      setBusy(false)
    }
  }

  return (
    <section className="mx-auto mt-10 max-w-lg rounded-xl border border-border bg-card p-8 text-center">
      <Laptop className="mx-auto size-10 text-muted-foreground" />
      {error ? (
        <div className="mt-4">
          <h1 className="font-heading text-2xl">{loadFailed ? "Não encontramos esse pedido de conexão" : pairing ? "Não foi possível conectar" : "Não foi possível abrir o pedido de conexão"}</h1>
          <p role="alert" className="mt-3 text-muted-foreground">{loadFailed ? "Abra o Agente do Ziv no seu computador de novo; ele abre esta página com o código certo." : error}</p>
          <div className="mt-6 flex flex-wrap justify-center gap-4 text-sm">
            <Link className="underline underline-offset-4" to="/configuracoes">Ir para Configurações</Link>
            <Link className="underline underline-offset-4" to="/instalar-agente">Como instalar o Agente</Link>
          </div>
        </div>
      ) : !pairing ? (
        <p className="mt-4 text-muted-foreground">Carregando…</p>
      ) : pairing.expired || pairing.approved ? (
        <p className="mt-4">Esse pedido de conexão venceu ou já foi usado. Abra o Agente de novo no seu computador.</p>
      ) : (
        <>
          <h1 className="mt-4 font-heading text-3xl">Conectar este computador à sua conta?</h1>
          <p className="mt-4 font-mono text-lg">{pairing.computer_name}</p>
          <p className="mt-2 text-sm text-muted-foreground">Depois disso, o Ziv poderá abrir apps neste computador quando você clicar em Rodar.</p>
          <div className="mt-6 flex justify-center gap-3">
            <Button variant="outline" onClick={() => navigate("/")}>Cancelar</Button>
            <Button onClick={approve} disabled={busy}>Conectar</Button>
          </div>
        </>
      )}
    </section>
  )
}
