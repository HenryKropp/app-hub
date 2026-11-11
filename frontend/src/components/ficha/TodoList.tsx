import type { ReactNode } from "react"

/** What is left to do on an app, shown on the Início tab (ADR 039). */
export type Todo = { key: string; title: string; detail?: string; action?: ReactNode }

/** One thing to do on the app, with the button that does it on the right. */
export function TodoList({ todos }: { todos: Todo[] }) {
  return (
    <ul className="flex flex-col gap-2">
      {todos.map((t) => (
        <li key={t.key} className="flex flex-col gap-2 rounded-xl border border-border px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm">
            <span className="font-medium">{t.title}</span>
            {t.detail && <span className="text-muted-foreground"> · {t.detail}</span>}
          </p>
          {t.action && <div className="flex shrink-0 gap-2">{t.action}</div>}
        </li>
      ))}
    </ul>
  )
}
