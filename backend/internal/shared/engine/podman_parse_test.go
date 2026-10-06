package engine

import (
	"os"
	"testing"
)

func TestParseSize(t *testing.T) {
	cases := map[string]int64{"0B": 0, "512B": 512, "1.5kB": 1500, "12.3MB": 12_300_000, "2GB": 2_000_000_000,
		"1KiB": 1024, "3MiB": 3 << 20, "1.5GiB": 1610612736}
	for in, want := range cases {
		got, err := ParseSize(in)
		if err != nil || got != want {
			t.Errorf("%s: %d %v", in, got, err)
		}
	}
	if _, err := ParseSize("muito"); err == nil {
		t.Error("accepted junk")
	}
}

func TestParseInspect(t *testing.T) {
	raw, _ := os.ReadFile("testdata/podman-inspect.json")
	st, err := parseInspect(raw, 80)
	if err != nil || !st.Exists || !st.Running || st.HostPort == 0 {
		t.Fatalf("%+v %v", st, err)
	}
}

func TestParseStats(t *testing.T) {
	raw, _ := os.ReadFile("testdata/podman-stats.json")
	u, err := parseStats(raw)
	if err != nil || len(u) != 1 || u[0].MemBytes <= 0 || u[0].ContainerID == "" {
		t.Fatalf("%+v %v", u, err)
	}
}

func TestParsePS(t *testing.T) {
	raw, _ := os.ReadFile("testdata/podman-ps.json")
	list, err := parsePS(raw)
	if err != nil || len(list) != 1 || list[0].Labels[Label] != "0" {
		t.Fatalf("%+v %v", list, err)
	}
}
