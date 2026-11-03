export function parseTags(text: string): string[] {
  const seen = new Set<string>()
  for (const part of text.split(",")) {
    const t = part.trim().toLowerCase()
    if (t) seen.add(t)
  }
  return [...seen]
}
