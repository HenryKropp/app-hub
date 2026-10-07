package hostinfo

import "testing"

func TestParseMeminfo(t *testing.T) {
	total, used, err := parseMeminfo("MemTotal:       16303072 kB\nMemFree:  1000 kB\nMemAvailable:   12000000 kB\n")
	if err != nil || total != 16303072*1024 || used != (16303072-12000000)*1024 {
		t.Fatalf("%d %d %v", total, used, err)
	}
}

func TestReadWorksHere(t *testing.T) {
	info, err := Read(t.TempDir())
	if err != nil || info.MemTotal == 0 || info.DiskTotal == 0 {
		t.Fatalf("%+v %v", info, err)
	}
}

// Like df: blocks reserved for root count neither as used nor as space this
// Hub can use, so the warning percentage matches what df shows.
func TestDiskUsageLikeDf(t *testing.T) {
	// 1000 blocks of 4 KiB: 300 used, 700 free, of which 50 reserved for root.
	total, used := diskUsage(1000, 700, 650, 4096)
	if used != 300*4096 || total != (300+650)*4096 {
		t.Fatalf("total %d used %d", total, used)
	}
}
