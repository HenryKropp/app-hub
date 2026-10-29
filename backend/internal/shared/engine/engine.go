// Package engine abstracts the container engine that runs shared apps.
package engine

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Label marks every container and image Ziv creates. Ziv never touches
// anything without it (or one of its predecessors' labels, LegacyLabels).
const (
	Label           = "ziv.app"
	DeploymentLabel = "ziv.deployment"
	ImagePrefix     = "localhost/ziv/"
)

// Names from before Ziv, newest first: Kova, then the App Hub. Boxes
// published back then keep running with them and are still recognized.
var (
	LegacyLabels           = []string{"kova.app", "app-hub.app"}
	LegacyDeploymentLabels = []string{"kova.deployment", "app-hub.deployment"}
	LegacyImagePrefixes    = []string{"localhost/kova/", "localhost/apphub/"}
)

// AllLabels is Label followed by LegacyLabels.
func AllLabels() []string { return append([]string{Label}, LegacyLabels...) }

// AppLabel returns the app id a container is labelled with, new label first.
func AppLabel(labels map[string]string) (string, bool) {
	for _, l := range AllLabels() {
		if v, ok := labels[l]; ok {
			return v, true
		}
	}
	return "", false
}

// DeploymentID returns the deployment a container belongs to, new label first.
func DeploymentID(labels map[string]string) string {
	for _, l := range append([]string{DeploymentLabel}, LegacyDeploymentLabels...) {
		if v, ok := labels[l]; ok {
			return v
		}
	}
	return ""
}

// NetworkPrefix names each app's own network ("ziv-apps-<app_id>"): apps
// cannot call each other, only the gateway reaches them.
const NetworkPrefix = "ziv-apps-"

// LegacyNetworkPrefixes are the networks Kova and the App Hub gave each app.
var LegacyNetworkPrefixes = []string{"kova-apps-", "app-hub-apps-"}

// AppNetwork is the network of one app (live and test boxes share it).
func AppNetwork(appID int64) string { return NetworkPrefix + strconv.FormatInt(appID, 10) }

// LegacyAppNetworks are the networks older versions gave the app.
func LegacyAppNetworks(appID int64) []string {
	out := make([]string, 0, len(LegacyNetworkPrefixes))
	for _, p := range LegacyNetworkPrefixes {
		out = append(out, p+strconv.FormatInt(appID, 10))
	}
	return out
}

var networkRe = func() *regexp.Regexp {
	alts := []string{regexp.QuoteMeta(NetworkPrefix)}
	for _, p := range LegacyNetworkPrefixes {
		alts = append(alts, regexp.QuoteMeta(p))
	}
	return regexp.MustCompile(`^(` + strings.Join(alts, "|") + `)[0-9]+$`)
}()

// HubNetwork reports whether a network name is one Ziv (or Kova, or the App Hub) creates.
func HubNetwork(name string) bool { return networkRe.MatchString(name) }

var ErrUnavailable = errors.New("o servidor de apps não está disponível")
var ErrNotFound = errors.New("caixa não encontrada")

type BuildSpec struct {
	ContextDir     string            // área de montagem (cópia da pasta)
	DockerfilePath string            // caminho dentro de ContextDir; "" = "Dockerfile"
	Tag            string            // localhost/ziv/<slug>:<deployment_id>
	Labels         map[string]string // sempre inclui Label
}

type StartSpec struct {
	Name     string // ziv-<slug>-<deployment_id>
	Image    string
	Labels   map[string]string
	Env      map[string]string // nunca vai para a linha de comando (arquivo 0600 apagado em seguida)
	Port     int               // porta dentro da caixa
	MemoryMB int
	CPUs     float64
	DataDir  string // pasta no servidor montada em /data
	Network  string // AppNetwork(app_id); criada se não existe
}

type State struct {
	Exists   bool
	Running  bool
	ExitCode int
	HostPort int // porta em 127.0.0.1 que leva à porta da caixa
}

type Usage struct {
	ContainerID string
	MemBytes    int64
	CPUPct      float64
}

type Container struct {
	ID      string
	Name    string
	Running bool
	Labels  map[string]string
}

type Engine interface {
	Available(ctx context.Context) error
	Build(ctx context.Context, b BuildSpec) (imageID string, log string, err error)
	Start(ctx context.Context, s StartSpec) (containerID string, err error)
	Stop(ctx context.Context, containerID string) error
	Remove(ctx context.Context, containerID string) error // para e remove; sem erro se não existe
	RemoveImage(ctx context.Context, imageID string) error
	Inspect(ctx context.Context, containerID string) (State, error)
	Stats(ctx context.Context, containerIDs []string) ([]Usage, error)
	Logs(ctx context.Context, containerID string, lines int) (string, error)
	Exec(ctx context.Context, container string, stdin io.Reader, stdout io.Writer, cmd ...string) error
	List(ctx context.Context) ([]Container, error)        // só as que têm Label
	RemoveNetwork(ctx context.Context, name string) error // só redes do Ziv (HubNetwork + Label); sem erro se não existe
}
