/** File types a file input accepts, as lowercase extensions with the dot. */

/** Turns what a person typed ("txt, .CSV; md") into [".txt", ".csv", ".md"]. */
export function parseAccept(text: string): string[] {
  const out: string[] = []
  for (const part of text.split(/[\s,;]+/)) {
    const ext = part.trim().toLowerCase().replace(/^\*?\.?/, "")
    if (ext && /^[a-z0-9]{1,10}$/.test(ext) && !out.includes(`.${ext}`)) out.push(`.${ext}`)
  }
  return out
}

/** ".txt, .csv ou .md" */
export function acceptText(list: string[]): string {
  if (list.length <= 1) return list[0] ?? ""
  return `${list.slice(0, -1).join(", ")} ou ${list[list.length - 1]}`
}

/** True when the name ends in one of the types, or the list is empty. */
export function acceptsFile(list: string[] | undefined, name: string): boolean {
  if (!list?.length) return true
  const dot = name.lastIndexOf(".")
  return dot > 0 && list.includes(name.slice(dot).toLowerCase())
}
