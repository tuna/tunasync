package internal

import (
	"encoding/json"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSyncStatus(t *testing.T) {
	Convey("SyncStatus json ser-de should work", t, func() {

		b, err := json.Marshal(PreSyncing)
		So(err, ShouldBeNil)
		So(b, ShouldResemble, []byte(`"pre-syncing"`)) // deep equal should be used

		var s SyncStatus

		err = json.Unmarshal([]byte(`"failed"`), &s)
		So(err, ShouldBeNil)
		So(s, ShouldEqual, Failed)
	})
}
