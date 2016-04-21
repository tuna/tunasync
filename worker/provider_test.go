package worker

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRsyncProvider(t *testing.T) {
	Convey("Rsync Provider should work", t, func() {

		c := rsyncConfig{
			name:        "tuna",
			upstreamURL: "rsync://rsync.tuna.moe/tuna/",
			workingDir:  "/srv/mirror/production/tuna",
			logDir:      "/var/log/tunasync",
			logFile:     "tuna.log",
			useIPv6:     true,
			interval:    600,
		}

		provider, err := newRsyncProvider(c)
		So(err, ShouldBeNil)

		So(provider.Name(), ShouldEqual, c.name)
		So(provider.WorkingDir(), ShouldEqual, c.workingDir)
		So(provider.LogDir(), ShouldEqual, c.logDir)
		So(provider.LogFile(), ShouldEqual, c.logFile)
		So(provider.Interval(), ShouldEqual, c.interval)

		Convey("When entering a context (auto exit)", func() {
			func() {
				ctx := provider.EnterContext()
				defer provider.ExitContext()
				So(provider.WorkingDir(), ShouldEqual, c.workingDir)
				newWorkingDir := "/srv/mirror/working/tuna"
				ctx.Set(_WorkingDirKey, newWorkingDir)
				So(provider.WorkingDir(), ShouldEqual, newWorkingDir)
			}()

			Convey("After context is done", func() {
				So(provider.WorkingDir(), ShouldEqual, c.workingDir)
			})
		})

		Convey("When entering a context (manually exit)", func() {
			ctx := provider.EnterContext()
			So(provider.WorkingDir(), ShouldEqual, c.workingDir)
			newWorkingDir := "/srv/mirror/working/tuna"
			ctx.Set(_WorkingDirKey, newWorkingDir)
			So(provider.WorkingDir(), ShouldEqual, newWorkingDir)

			Convey("After context is done", func() {
				provider.ExitContext()
				So(provider.WorkingDir(), ShouldEqual, c.workingDir)
			})
		})

	})
}
