import { expect, test } from "vitest"
import { formatDuration, formatSize, formatWhen, isLive, relativeDate, relativeTime } from "./runs"

test("durations", () => {
  const t0 = "2026-09-30T14:10:00Z"
  expect(formatDuration(t0, "2026-09-30T14:10:45Z")).toBe("45 s")
  expect(formatDuration(t0, "2026-09-30T14:22:10Z")).toBe("12 min")
  expect(formatDuration(t0, "2026-09-30T15:15:00Z")).toBe("1 h 05 min")
  expect(formatDuration(null, null)).toBe("—")
  expect(formatDuration(t0, null, Date.parse("2026-09-30T14:11:00Z"))).toBe("1 min")
})

test("sizes", () => {
  expect(formatSize(820)).toBe("820 B")
  expect(formatSize(12 * 1024)).toBe("12 KB")
  expect(formatSize(79291437)).toBe("75,6 MB")
  expect(formatSize(68099.6 * 2 ** 20)).toBe("66,5 GB")
  expect(formatSize(1031018.4 * 2 ** 20)).toBe("1006,9 GB")
})

test("when and live", () => {
  expect(formatWhen("2026-09-30T14:10:00-03:00")).toMatch(/30\/09 às \d\d:10/)
  expect(isLive("running")).toBe(true)
  expect(isLive("done")).toBe(false)
})

test("relative dates compare calendar days in local time", () => {
  const morning = new Date(2026, 10, 4, 8, 0).getTime()
  const lastNight = new Date(2026, 10, 3, 23, 0).toISOString() // 9 h earlier, previous day
  expect(relativeDate(lastNight, morning)).toBe("ontem")
  expect(relativeTime(lastNight, morning)).toBe("ontem")
  const sameDay = new Date(2026, 10, 4, 1, 0).toISOString()
  expect(relativeDate(sameDay, morning)).toBe("hoje")
  expect(relativeTime(sameDay, morning)).toBe("há 7 h")
  expect(relativeDate(new Date(2026, 10, 1, 12, 0).toISOString(), morning)).toBe("há 3 dias")
  expect(relativeTime(new Date(2026, 10, 4, 7, 30).toISOString(), morning)).toBe("há 30 min")
})
