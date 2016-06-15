package internal

import (
	"fmt"
	"time"
)

// A StatusUpdateMsg represents a msg when
// a worker has done syncing
type MirrorStatus struct {
	Name       string     `json:"name"`
	Worker     string     `json:"worker"`
	IsMaster   bool       `json:"is_master"`
	Status     SyncStatus `json:"status"`
	LastUpdate time.Time  `json:"last_update"`
	Upstream   string     `json:"upstream"`
	Size       string     `json:"size"`
	ErrorMsg   string     `json:"error_msg"`
}

// A WorkerStatus is the information struct that describe
// a worker, and sent from the manager to clients.
type WorkerStatus struct {
	ID         string    `json:"id"`
	URL        string    `json:"url"`         // worker url
	Token      string    `json:"token"`       // session token
	LastOnline time.Time `json:"last_online"` // last seen
}

type CmdVerb uint8

const (
	CmdStart   CmdVerb = iota
	CmdStop            // stop syncing keep the job
	CmdDisable         // disable the job (stops goroutine)
	CmdRestart         // restart syncing
	CmdPing            // ensure the goroutine is alive
	CmdReload          // reload mirror config
)

func (c CmdVerb) String() string {
	switch c {
	case CmdStart:
		return "start"
	case CmdStop:
		return "stop"
	case CmdDisable:
		return "disable"
	case CmdRestart:
		return "restart"
	case CmdPing:
		return "ping"
	case CmdReload:
		return "reload"
	}
	return "unknown"
}

// A WorkerCmd is the command message send from the
// manager to a worker
type WorkerCmd struct {
	Cmd      CmdVerb  `json:"cmd"`
	MirrorID string   `json:"mirror_id"`
	Args     []string `json:"args"`
}

func (c WorkerCmd) String() string {
	if len(c.Args) > 0 {
		return fmt.Sprintf("%v (%s, %v)", c.Cmd, c.MirrorID, c.Args)
	}
	return fmt.Sprintf("%v (%s)", c.Cmd, c.MirrorID)
}

// A ClientCmd is the command message send from client
// to the manager
type ClientCmd struct {
	Cmd      CmdVerb  `json:"cmd"`
	MirrorID string   `json:"mirror_id"`
	WorkerID string   `json:"worker_id"`
	Args     []string `json:"args"`
}
