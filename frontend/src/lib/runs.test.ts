import { expect, test } from "vitest"
import { formatDuration, formatSize, formatWhen, isLive } from "./runs"

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
