package internal

import (
	"encoding/json"
	"errors"
	"fmt"
)

type SyncStatus uint8

const (
	None SyncStatus = iota
	Failed
	Success
	Syncing
	PreSyncing
	Paused
	Disabled
)

func (s SyncStatus) MarshalJSON() ([]byte, error) {
	var strStatus string
	switch s {
	case None:
		strStatus = "none"
	case Failed:
		strStatus = "failed"
	case Success:
		strStatus = "success"
	case Syncing:
		strStatus = "syncing"
	case PreSyncing:
		strStatus = "pre-syncing"
	case Paused:
		strStatus = "paused"
	case Disabled:
		strStatus = "disabled"
	default:
		return []byte{}, errors.New("Invalid status value")
	}

	return json.Marshal(strStatus)
}

func (s *SyncStatus) UnmarshalJSON(v []byte) error {
	sv := string(v)
	switch sv {
	case `"none"`:
		*s = None
	case `"failed"`:
		*s = Failed
	case `"success"`:
		*s = Success
	case `"syncing"`:
		*s = Syncing
	case `"pre-syncing"`:
		*s = PreSyncing
	case `"paused"`:
		*s = Paused
	case `"disabled"`:
		*s = Disabled
	default:
		return fmt.Errorf("Invalid status value: %s", string(v))
	}
	return nil
}
