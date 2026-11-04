import { useState } from "react"
import { Star } from "lucide-react"
import { api } from "@/lib/api"
import { cn } from "@/lib/utils"

/** Toggle for the Home's favorites. It answers at once and goes back if the server refuses. */
export function FavoriteStar({ kind, id, on, name, onChange }: { kind: "app" | "artifact"; id: number; on: boolean; name: string; onChange?: (on: boolean) => void }) {
  const [value, setValue] = useState(on)
  const [last, setLast] = useState(on)
  // The list or the page loaded again with another answer.
  if (on !== last) {
    setLast(on)
    setValue(on)
  }
  const toggle = () => {
    const next = !value
    setValue(next)
    api.setFavorite(kind, id, next).then(
      () => onChange?.(next),
      () => setValue(!next),
    )
  }
  return (
    <button
      type="button"
      title="Favorito"
      aria-pressed={value}
      aria-label={value ? `Tirar ${name} dos favoritos` : `Marcar ${name} como favorito`}
      onClick={toggle}
      className="relative z-10 inline-flex size-7 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <Star className={cn("size-4", value && "fill-foreground text-foreground")} />
    </button>
  )
}
