package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

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
	raw := mustRead(t, "testdata/podman-inspect.json")
	st, err := parseInspect(raw, 80)
	if err != nil || !st.Exists || !st.Running || st.HostPort == 0 {
		t.Fatalf("%+v %v", st, err)
	}
	st0, err := parseInspect(raw, 0)
	if err != nil || st0.HostPort != st.HostPort {
		t.Fatalf("port 0: %+v %v", st0, err)
	}
}

func TestParseStats(t *testing.T) {
	raw := mustRead(t, "testdata/podman-stats.json")
	u, err := parseStats(raw)
	if err != nil || len(u) != 1 || u[0].MemBytes <= 0 || u[0].ContainerID == "" {
		t.Fatalf("%+v %v", u, err)
	}
}

func TestParsePS(t *testing.T) {
	raw := mustRead(t, "testdata/podman-ps.json")
	list, err := parsePS(raw)
	if err != nil || len(list) != 1 || list[0].Labels[LegacyLabel] != "0" { // fixture captured in the App Hub era
		t.Fatalf("%+v %v", list, err)
	}
}

func TestNormalizeStats(t *testing.T) {
	var insp []struct{ Id string }
	if err := json.Unmarshal(mustRead(t, "testdata/podman-inspect.json"), &insp); err != nil || len(insp) != 1 || len(insp[0].Id) != 64 {
		t.Fatalf("%+v %v", insp, err)
	}
	full := insp[0].Id
	all, err := parseStats(mustRead(t, "testdata/podman-stats.json"))
	if err != nil || len(all) != 1 || len(all[0].ContainerID) != 12 {
		t.Fatalf("%+v %v", all, err)
	}
	got := normalizeStats(all, []string{"", full, "ffffffffffffffffffff"})
	if len(got) != 1 || got[0].ContainerID != full || got[0].MemBytes <= 0 {
		t.Fatalf("%+v", got)
	}
}

func TestSafetyDecisions(t *testing.T) {
	if hubOwned("") || hubOwned("  \n") || !hubOwned("12") {
		t.Error("hubOwned")
	}
	if imageAllowed("", nil) || imageAllowed("", []string{"docker.io/library/busybox:latest"}) {
		t.Error("foreign image allowed")
	}
	if !imageAllowed("7", nil) || !imageAllowed("", []string{"localhost/apphub/x:1"}) {
		t.Error("hub image refused")
	}
}
