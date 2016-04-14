package status

import (
	"encoding/json"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStatus(t *testing.T) {
	Convey("status json ser-de should work", t, func() {
		tz := "Asia/Shanghai"
		loc, err := time.LoadLocation(tz)
		So(err, ShouldBeNil)

		m := MirrorStatus{
			Name:       "tunalinux",
			Status:     Success,
			LastUpdate: time.Date(2016, time.April, 16, 23, 8, 10, 0, loc),
			Size:       "5GB",
			Upstream:   "rsync://mirrors.tuna.tsinghua.edu.cn/tunalinux/",
		}

		b, err := json.Marshal(m)
		So(err, ShouldBeNil)
		// fmt.Println(string(b))
		var m2 MirrorStatus
		err = json.Unmarshal(b, &m2)
		So(err, ShouldBeNil)
		// fmt.Printf("%#v", m2)
		So(m2.Name, ShouldEqual, m.Name)
		So(m2.Status, ShouldEqual, m.Status)
		So(m2.LastUpdate.Unix(), ShouldEqual, m.LastUpdate.Unix())
		So(m2.LastUpdate.UnixNano(), ShouldEqual, m.LastUpdate.UnixNano())
		So(m2.Size, ShouldEqual, m.Size)
		So(m2.Upstream, ShouldEqual, m.Upstream)
	})
}
