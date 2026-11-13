package deploy

import (
	"strings"
	"testing"
)

func TestExplainBuildLocalImage(t *testing.T) {
	for _, log := range []string{
		`Error: creating build container: initializing source docker://localhost/ziv/w1/app:1: pinging container registry localhost: Get "https://localhost/v2/": dial tcp 127.0.0.1:443: connect: connection refused`,
		`Error: COPY --from=localhost/ziv/w1/app:1: no stage or image found with that name`,
		`Error: creating build container: pull policy is always but image has been referred to by ID (270605)`,
	} {
		if got := explainBuild(log); !strings.Contains(got, "imagem guardada no servidor") {
			t.Errorf("%q -> %q", log, got)
		}
	}
	// A real network failure still says to try again.
	if got := explainBuild(`initializing source docker://docker.io/library/python:3.12-slim: dial tcp: i/o timeout`); !strings.Contains(got, "Tente de novo") {
		t.Errorf("network: %q", got)
	}
}
