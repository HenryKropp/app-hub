// Package agentproto holds the messages exchanged between the Hub and the
// Agent do Hub running on people's Windows computers.
package agentproto

import (
	"encoding/json"
	"time"
)

// Version is the agent version this Hub expects.
const Version = "0.2.0"

// Job kinds.
const (
	KindRun          = "run"
	KindChooseFolder = "choose_folder"
	KindSendFile     = "send_file"
	KindUpdate       = "update"
)

// Result events.
const (
	EventStarted  = "started"  // run: app is about to open
	EventFinished = "finished" // run: window closed, files listed
	EventFailed   = "failed"   // any job: could not do it
	EventMissing  = "missing"  // send_file: file no longer in the folder
	EventDone     = "done"     // choose_folder: answered (folder or canceled)
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
	RunID        int64  `json:"run_id"`
	AppFolder    string `json:"app_folder"`
	Exe          string `json:"exe"`
	OutputFolder string `json:"output_folder"`
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
