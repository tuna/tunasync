package status

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type syncStatus uint8

const (
	None syncStatus = iota
	Failed
	Success
	Syncing
	PreSyncing
	Paused
	Disabled
)

type MirrorStatus struct {
	Name       string
	Status     syncStatus
	LastUpdate time.Time
	Upstream   string
	Size       string // approximate size
}

func (s MirrorStatus) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"name":           s.Name,
		"status":         s.Status,
		"last_update":    s.LastUpdate.Format("2006-01-02 15:04:05"),
		"last_update_ts": fmt.Sprintf("%d", s.LastUpdate.Unix()),
		"size":           s.Size,
		"upstream":       s.Upstream,
	}
	return json.Marshal(m)
}

func (s *MirrorStatus) UnmarshalJSON(v []byte) error {
	var m map[string]interface{}

	err := json.Unmarshal(v, &m)
	if err != nil {
		return err
	}

	if name, ok := m["name"]; ok {
		if s.Name, ok = name.(string); !ok {
			return errors.New("name should be a string")
		}
	} else {
		return errors.New("key `name` does not exist in the json")
	}
	if upstream, ok := m["upstream"]; ok {
		if s.Upstream, ok = upstream.(string); !ok {
			return errors.New("upstream should be a string")
		}
	} else {
		return errors.New("key `upstream` does not exist in the json")
	}
	if size, ok := m["size"]; ok {
		if s.Size, ok = size.(string); !ok {
			return errors.New("size should be a string")
		}
	} else {
		return errors.New("key `size` does not exist in the json")
	}
	// tricky: status
	if status, ok := m["status"]; ok {
		if ss, ok := status.(string); ok {
			err := json.Unmarshal([]byte(`"`+ss+`"`), &(s.Status))
			if err != nil {
				return err
			}
		} else {
			return errors.New("status should be a string")
		}
	} else {
		return errors.New("key `status` does not exist in the json")
	}
	// tricky: last update
	if lastUpdate, ok := m["last_update_ts"]; ok {
		if sts, ok := lastUpdate.(string); ok {
			ts, err := strconv.Atoi(sts)
			if err != nil {
				return fmt.Errorf("last_update_ts should be a interger, got: %s", sts)
			}
			s.LastUpdate = time.Unix(int64(ts), 0)
		} else {
			return fmt.Errorf("last_update_ts should be a string of integer, got: %s", lastUpdate)
		}
	} else {
		return errors.New("key `last_update_ts` does not exist in the json")
	}
	return nil
}

func (s syncStatus) MarshalJSON() ([]byte, error) {
	var strStatus string
	switch s {
	case None:
		strStatus = "none"
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

func (s *syncStatus) UnmarshalJSON(v []byte) error {
	sv := strings.Trim(string(v), `"`)
	switch sv {
	case "none":
		*s = None
	case "success":
		*s = Success
	case "syncing":
		*s = Syncing
	case "pre-syncing":
		*s = PreSyncing
	case "paused":
		*s = Paused
	case "disabled":
		*s = Disabled
	default:
		return fmt.Errorf("Invalid status value: %s", string(v))
	}
	return nil
}
