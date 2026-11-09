// Package agentproto holds the messages exchanged between the Hub and the
// Agent do Ziv running on people's Windows computers.
package agentproto

import (
	"encoding/json"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// Version is the agent version this Hub expects.
const Version = "0.3.5"

// Job kinds.
const (
	KindRun          = "run"
	KindChooseFolder = "choose_folder"
	KindSendFile     = "send_file"
	KindUpdate       = "update"
	KindStop         = "stop"
)

// Result events.
const (
	EventStarted  = "started"  // run: app is about to open
	EventFinished = "finished" // run: window closed, files listed
	EventFailed   = "failed"   // any job: could not do it
	EventMissing  = "missing"  // send_file: file no longer in the folder
	EventDone     = "done"     // choose_folder: answered (folder or canceled); stop: app closed
)

// Job is what GET /api/agent/next returns.
type Job struct {
	ID      int64           `json:"id"`
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

// RunPayload opens AppFolder\Exe with OutputFolder as the place results go.
// Paths are Windows paths ("C:\...").
type RunPayload struct {
	RunID        int64          `json:"run_id"`
	AppFolder    string         `json:"app_folder"`
	Exe          string         `json:"exe"`
	OutputFolder string         `json:"output_folder"`
	Script       *ScriptPayload `json:"script,omitempty"`
	// Web starts a screen app on this computer and opens the browser (0.3.5+).
	Web *WebPayload `json:"web,omitempty"`
	// AppName names the app when another run needs this folder (0.3.2+).
	AppName string `json:"app_name,omitempty"`
	// Keys: the app has a vault; fetch the values once from
	// GET /api/agent/jobs/{id}/keys and give them only to the app: the script (0.3.4+) or the program it opens (0.3.5+).
	Keys bool `json:"keys,omitempty"`
	// KeyValues holds the fetched keys in the Agente's memory only. Never
	// serialized, logged or written to disk.
	KeyValues map[string]string `json:"-"`
}

type ScriptPayload struct {
	Sources  map[string]manifest.InputFile `json:"sources,omitempty"`
	Manifest manifest.Manifest             `json:"manifest"`
	Inputs   map[string][]string           `json:"inputs,omitempty"`
	Files    map[string]manifest.InputFile `json:"files,omitempty"`
}

// WebPayload starts a screen app on the computer. The Hub works out how to
// start it (the same recipe as the 24h version) and sends the code it keeps.
type WebPayload struct {
	AppID int64 `json:"app_id"`
	// Runtime is "python" or "node". Args follow the Python interpreter
	// ("-m", "streamlit", …) or npm ("start"); "{PORT}" becomes the port.
	Runtime string   `json:"runtime"`
	Args    []string `json:"args"`
	// DepsFile is the app's requirements.txt; Requirements are the libraries
	// to install besides it (detected imports or the local web server).
	DepsFile     string                        `json:"deps_file,omitempty"`
	Requirements []string                      `json:"requirements,omitempty"`
	Sources      map[string]manifest.InputFile `json:"sources"`
}

// PortPlaceholder in WebPayload.Args is replaced by the port the app gets.
const PortPlaceholder = "{PORT}"

// StopPayload closes the app opened for RunID, with everything it opened.
type StopPayload struct {
	RunID int64 `json:"run_id"`
}

type ChooseFolderPayload struct {
	Initial string `json:"initial"`
}

// SendFilePayload asks for one file, or several zipped, from a run's results.
// Paths are relative to the run's output folder, with forward slashes.
type SendFilePayload struct {
	RunID int64    `json:"run_id"`
	Paths []string `json:"paths"`
	Zip   bool     `json:"zip"`
}

// FileEntry is a new or changed file found after a run.
type FileEntry struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Change   string    `json:"change"` // "new" or "changed"
}

// Result is posted to POST /api/agent/jobs/{id}/result.
type Result struct {
	Event      string      `json:"event"`
	Error      string      `json:"error,omitempty"`
	ExitCode   int         `json:"exit_code,omitempty"`
	AppVersion string      `json:"app_version,omitempty"`
	Files      []FileEntry `json:"files,omitempty"`
	Folder     string      `json:"folder,omitempty"`
	Canceled   bool        `json:"canceled,omitempty"`
	Stopped    bool        `json:"stopped,omitempty"` // run: closed by a stop job
}

// UpdatePayload asks the agent to replace itself with the Hub's agent binary.
type UpdatePayload struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

type Hello struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	CanUpdate bool   `json:"can_update,omitempty"` // 0.2.0+: understands update jobs
	CanStop   bool   `json:"can_stop,omitempty"`   // 0.3.1+: understands stop jobs
}

type PairStartRequest struct {
	ComputerName string `json:"computer_name"`
}

type PairStartResponse struct {
	Code       string `json:"code"`
	Secret     string `json:"secret"`
	ApproveURL string `json:"approve_url"`
}

type PairPollRequest struct {
	Secret string `json:"secret"`
}

// PairPollResponse.Status is "pending" or "approved" (with Token).
type PairPollResponse struct {
	Status string `json:"status"`
	Token  string `json:"token,omitempty"`
}
