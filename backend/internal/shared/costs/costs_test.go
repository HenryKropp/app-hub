package costs

import (
	"testing"
	"time"
)

const day = 24 * time.Hour

func TestSplitByMemoryAndTime(t *testing.T) {
	// servidor de R$ 300/mês com 4 GB; metade do mês passou → orçamento R$ 150
	per, hub := Estimate(30000, 4096, 15*day, 30*day, []AppUse{
		{AppID: 1, MemoryMB: 1024, Minutes: int64((15 * day).Minutes())},    // ligado o tempo todo, 1/4 da memória
		{AppID: 2, MemoryMB: 512, Minutes: int64((15 * day).Minutes()) / 2}, // metade do tempo, 1/8
	})
	if per[1] != 3750 || per[2] != 938 || hub != 15000-3750-938 {
		t.Fatalf("per %v hub %d", per, hub)
	}
}

func TestOvercommitNeverPassesBudget(t *testing.T) {
	per, hub := Estimate(10000, 1024, 30*day, 30*day, []AppUse{
		{AppID: 1, MemoryMB: 1024, Minutes: int64((30 * day).Minutes())},
		{AppID: 2, MemoryMB: 1024, Minutes: int64((30 * day).Minutes())},
	})
	if per[1]+per[2] > 10000 || hub < 0 || per[1] != per[2] {
		t.Fatalf("per %v hub %d", per, hub)
	}
}

func TestNoCostOrNoTime(t *testing.T) {
	per, hub := Estimate(0, 4096, 10*day, 30*day, []AppUse{{AppID: 1, MemoryMB: 512, Minutes: 100}})
	if per[1] != 0 || hub != 0 {
		t.Fatal("zero cost should split nothing")
	}
	per, _ = Estimate(30000, 4096, 0, 30*day, []AppUse{{AppID: 1, MemoryMB: 512}})
	if per[1] != 0 {
		t.Fatal("no elapsed time")
	}
}
