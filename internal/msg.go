package internal

import "time"

// A StatusUpdateMsg represents a msg when
// a worker has done syncing
type StatusUpdateMsg struct {
	Name       string     `json:"name"`
	Worker     string     `json:"worker"`
	IsMaster   bool       `json:"is_master"`
	Status     SyncStatus `json:"status"`
	LastUpdate time.Time  `json:"last_update"`
	Upstream   string     `json:"upstream"`
	Size       string     `json:"size"`
	ErrorMsg   string     `json:"error_msg"`
}

// A WorkerInfoMsg is the information struct that describe
// a worker, and sent from the manager to clients.
type WorkerInfoMsg struct {
	ID         string    `json:"id"`
	LastOnline time.Time `json:"last_online"`
}

type CmdVerb uint8

const (
	CmdStart   CmdVerb = iota
	CmdStop            // stop syncing keep the job
	CmdDisable         // disable the job (stops goroutine)
	CmdRestart         // restart syncing
	CmdPing            // ensure the goroutine is alive
)

// A WorkerCmd is the command message send from the
// manager to a worker
type WorkerCmd struct {
	Cmd      CmdVerb  `json:"cmd"`
	MirrorID string   `json:"mirror_id"`
	Args     []string `json:"args"`
}

// A ClientCmd is the command message send from client
// to the manager
type ClientCmd struct {
	Cmd      CmdVerb  `json:"cmd"`
	MirrorID string   `json:"mirror_id"`
	WorkerID string   `json:"worker_id"`
	Args     []string `json:"args"`
}
