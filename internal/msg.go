package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// A MirrorStatus represents a msg when
// a worker has done syncing
type MirrorStatus struct {
	Name        string     `json:"name"`
	Worker      string     `json:"worker"`
	IsMaster    bool       `json:"is_master"`
	Status      SyncStatus `json:"status"`
	LastUpdate  time.Time  `json:"last_update"`
	LastStarted time.Time  `json:"last_started"`
	LastEnded   time.Time  `json:"last_ended"`
	Scheduled   time.Time  `json:"next_schedule"`
	Upstream    string     `json:"upstream"`
	Size        string     `json:"size"`
	ErrorMsg    string     `json:"error_msg"`
}

// A WorkerStatus is the information struct that describe
// a worker, and sent from the manager to clients.
type WorkerStatus struct {
	ID           string    `json:"id"`
	URL          string    `json:"url"`           // worker url
	Token        string    `json:"token"`         // session token
	LastOnline   time.Time `json:"last_online"`   // last seen
	LastRegister time.Time `json:"last_register"` // last register time
}

type MirrorSchedules struct {
	Schedules []MirrorSchedule `json:"schedules"`
}

type MirrorSchedule struct {
	MirrorName   string    `json:"name"`
	NextSchedule time.Time `json:"next_schedule"`
}

// A CmdVerb is an action to a job or worker
type CmdVerb uint8

const (
	// CmdStart start a job
	CmdStart CmdVerb = iota
	// CmdStop stop syncing, but keep the job
	CmdStop
	// CmdDisable disable the job (stops goroutine)
	CmdDisable
	// CmdRestart restart a syncing job
	CmdRestart
	// CmdPing ensures the goroutine is alive
	CmdPing

	// CmdReload tells a worker to reload mirror config
	CmdReload
)

func (c CmdVerb) String() string {
	mapping := map[CmdVerb]string{
		CmdStart:   "start",
		CmdStop:    "stop",
		CmdDisable: "disable",
		CmdRestart: "restart",
		CmdPing:    "ping",
		CmdReload:  "reload",
	}
	return mapping[c]
}

func NewCmdVerbFromString(s string) CmdVerb {
	mapping := map[string]CmdVerb{
		"start":   CmdStart,
		"stop":    CmdStop,
		"disable": CmdDisable,
		"restart": CmdRestart,
		"ping":    CmdPing,
		"reload":  CmdReload,
	}
	return mapping[s]
}

// Marshal and Unmarshal for CmdVerb
func (s CmdVerb) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(s.String())
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

func (s *CmdVerb) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	*s = NewCmdVerbFromString(j)
	return nil
}

// A WorkerCmd is the command message send from the
// manager to a worker
type WorkerCmd struct {
	Cmd      CmdVerb         `json:"cmd"`
	MirrorID string          `json:"mirror_id"`
	Args     []string        `json:"args"`
	Options  map[string]bool `json:"options"`
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
	Cmd      CmdVerb         `json:"cmd"`
	MirrorID string          `json:"mirror_id"`
	WorkerID string          `json:"worker_id"`
	Args     []string        `json:"args"`
	Options  map[string]bool `json:"options"`
}
