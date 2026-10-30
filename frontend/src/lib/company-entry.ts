const SLUG_PATTERN = /^[a-z][a-z0-9-]{2,39}$/

export function validCompanySlug(slug: string) {
  return slug.length >= 3 && slug.length <= 40 && SLUG_PATTERN.test(slug)
}

export function slugFromEntry(value: string, origin: string): string | null {
  const entered = value.trim()
  if (!entered) return null
  if (validCompanySlug(entered)) return entered
  try {
    const url = new URL(entered, origin)
    if (url.origin !== origin) return null
    const match = url.pathname.match(/^\/w\/([a-z][a-z0-9-]{2,39})\/?$/)
    return match?.[1] && validCompanySlug(match[1]) ? match[1] : null
  } catch {
    return null
  }
}
