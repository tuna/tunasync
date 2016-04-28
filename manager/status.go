package manager

import (
	"encoding/json"
	"strconv"
	"time"

	. "github.com/tuna/tunasync/internal"
)

type textTime struct {
	time.Time
}

func (t textTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Format("2006-01-02 15:04:05"))
}
func (t *textTime) UnmarshalJSON(b []byte) error {
	s := string(b)
	t2, err := time.ParseInLocation(`"2006-01-02 15:04:05"`, s, time.Local)
	*t = textTime{t2}
	return err
}

type stampTime struct {
	time.Time
}

func (t stampTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Unix())
}
func (t *stampTime) UnmarshalJSON(b []byte) error {
	ts, err := strconv.Atoi(string(b))
	if err != nil {
		return err
	}
	*t = stampTime{time.Unix(int64(ts), 0)}
	return err
}

// webMirrorStatus is the mirror status to be shown in the web page
type webMirrorStatus struct {
	Name         string     `json:"name"`
	IsMaster     bool       `json:"is_master"`
	Status       SyncStatus `json:"status"`
	LastUpdate   textTime   `json:"last_update"`
	LastUpdateTs stampTime  `json:"last_update_ts"`
	Upstream     string     `json:"upstream"`
	Size         string     `json:"size"` // approximate size
}

func convertMirrorStatus(m MirrorStatus) webMirrorStatus {
	return webMirrorStatus{
		Name:         m.Name,
		IsMaster:     m.IsMaster,
		Status:       m.Status,
		LastUpdate:   textTime{m.LastUpdate},
		LastUpdateTs: stampTime{m.LastUpdate},
		Upstream:     m.Upstream,
		Size:         m.Size,
	}
}
