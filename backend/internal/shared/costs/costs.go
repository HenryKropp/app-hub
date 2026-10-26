// Package costs estimates how much of the server's monthly cost each shared
// app used, by reserved memory × time on.
package costs

import "time"

type AppUse struct {
	AppID    int64
	MemoryMB int   // reserved memory
	Minutes  int64 // minutes on in the period
}

// Estimate splits what the server cost in the period among the apps by
// reserved memory × time on. budget = monthly × elapsed / month; an app's
// share = budget × (mem × minutes) / (totalMem × elapsed). If the
// reservations add up to more than the server (overcommit), shares are
// scaled down so they never pass the budget. What is left is the Hub's.
func Estimate(monthlyCents int64, totalMemMB int, elapsed, month time.Duration, apps []AppUse) (map[int64]int64, int64) {
	per := map[int64]int64{}
	if monthlyCents <= 0 || elapsed <= 0 || month <= 0 || totalMemMB <= 0 {
		for _, a := range apps {
			per[a.AppID] = 0
		}
		return per, 0
	}
	budget := float64(monthlyCents) * elapsed.Minutes() / month.Minutes()
	capacity := float64(totalMemMB) * elapsed.Minutes()
	var used float64
	for _, a := range apps {
		used += float64(a.MemoryMB) * float64(a.Minutes)
	}
	scale := 1.0
	if used > capacity {
		scale = capacity / used
	}
	var sum int64
	for _, a := range apps {
		share := budget * float64(a.MemoryMB) * float64(a.Minutes) / capacity * scale
		per[a.AppID] = int64(share + 0.5)
		sum += per[a.AppID]
	}
	hub := int64(budget+0.5) - sum
	if hub < 0 {
		hub = 0
	}
	return per, hub
}
