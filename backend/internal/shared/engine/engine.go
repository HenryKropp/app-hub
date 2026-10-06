// Package engine abstracts the container engine that runs shared apps.
package engine

import (
	"context"
	"errors"
	"io"
)

// Label marks every container and image the Hub creates. The Hub never
// touches anything without it.
const Label = "app-hub.app"
const Network = "app-hub-apps"

var ErrUnavailable = errors.New("o servidor de apps não está disponível")
var ErrNotFound = errors.New("caixa não encontrada")

type BuildSpec struct {
	ContextDir     string            // área de montagem (cópia da pasta)
	DockerfilePath string            // caminho dentro de ContextDir; "" = "Dockerfile"
	Tag            string            // localhost/apphub/<slug>:<deployment_id>
	Labels         map[string]string // sempre inclui Label
}

type StartSpec struct {
	Name     string // apphub-<slug>-<deployment_id>
	Image    string
	Labels   map[string]string
	Env      map[string]string // nunca vai para a linha de comando (arquivo 0600 apagado em seguida)
	Port     int               // porta dentro da caixa
	MemoryMB int
	CPUs     float64
	DataDir  string // pasta no servidor montada em /data
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
	List(ctx context.Context) ([]Container, error) // só as que têm Label
}
