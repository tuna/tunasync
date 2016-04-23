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
}

// A WorkerInfoMsg is
type WorkerInfoMsg struct {
	Name string `json:"name"`
}

type CmdVerb uint8

const (
	CmdStart   CmdVerb = iota
	CmdStop            // stop syncing keep the job
	CmdDisable         // disable the job (stops goroutine)
	CmdRestart         // restart syncing
	CmdPing            // ensure the goroutine is alive
)

type WorkerCmd struct {
	Cmd  CmdVerb  `json:"cmd"`
	Args []string `json:"args"`
}

type ClientCmd struct {
	Cmd      CmdVerb  `json:"cmd"`
	MirrorID string   `json:"mirror_id"`
	WorkerID string   `json:"worker_id"`
	Args     []string `json:"args"`
}
