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
