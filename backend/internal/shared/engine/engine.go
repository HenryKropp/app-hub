// Package engine abstracts the container engine that runs shared apps.
package engine

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strconv"
)

// Label marks every container and image Kova creates. Kova never touches
// anything without it (or its App Hub predecessor, LegacyLabel).
const (
	Label           = "kova.app"
	DeploymentLabel = "kova.deployment"
	ImagePrefix     = "localhost/kova/"

	// Names from the App Hub, before it became Kova. Boxes published back then
	// keep running with them and are still recognized.
	LegacyLabel           = "app-hub.app"
	LegacyDeploymentLabel = "app-hub.deployment"
	LegacyImagePrefix     = "localhost/apphub/"
)

// AppLabel returns the app id a container is labelled with, new label first.
func AppLabel(labels map[string]string) (string, bool) {
	if v, ok := labels[Label]; ok {
		return v, true
	}
	v, ok := labels[LegacyLabel]
	return v, ok
}

// DeploymentID returns the deployment a container belongs to, new label first.
func DeploymentID(labels map[string]string) string {
	if v, ok := labels[DeploymentLabel]; ok {
		return v
	}
	return labels[LegacyDeploymentLabel]
}

// NetworkPrefix names each app's own network ("kova-apps-<app_id>"): apps
// cannot call each other, only the gateway reaches them.
const (
	NetworkPrefix       = "kova-apps-"
	LegacyNetworkPrefix = "app-hub-apps-"
)

// AppNetwork is the network of one app (live and test boxes share it).
func AppNetwork(appID int64) string { return NetworkPrefix + strconv.FormatInt(appID, 10) }

// LegacyAppNetwork is the network the App Hub gave the app.
func LegacyAppNetwork(appID int64) string { return LegacyNetworkPrefix + strconv.FormatInt(appID, 10) }

var networkRe = regexp.MustCompile(`^(` + regexp.QuoteMeta(NetworkPrefix) + `|` + regexp.QuoteMeta(LegacyNetworkPrefix) + `)[0-9]+$`)

// HubNetwork reports whether a network name is one Kova (or the App Hub) creates.
func HubNetwork(name string) bool { return networkRe.MatchString(name) }

var ErrUnavailable = errors.New("o servidor de apps não está disponível")
var ErrNotFound = errors.New("caixa não encontrada")

type BuildSpec struct {
	ContextDir     string            // área de montagem (cópia da pasta)
	DockerfilePath string            // caminho dentro de ContextDir; "" = "Dockerfile"
	Tag            string            // localhost/kova/<slug>:<deployment_id>
	Labels         map[string]string // sempre inclui Label
}

type StartSpec struct {
	Name     string // kova-<slug>-<deployment_id>
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
	RemoveNetwork(ctx context.Context, name string) error // só redes do Kova (HubNetwork + Label); sem erro se não existe
}
