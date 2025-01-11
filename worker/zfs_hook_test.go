package worker

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestZFSHook(t *testing.T) {

	Convey("ZFS Hook should work", t, func(ctx C) {
		tmpDir, err := os.MkdirTemp("", "tunasync")
		tmpFile := filepath.Join(tmpDir, "log_file")

		c := cmdConfig{
			name:        "tuna_zfs_hook_test",
			upstreamURL: "http://mirrors.tuna.moe/",
			command:     "ls",
			workingDir:  tmpDir,
			logDir:      tmpDir,
			logFile:     tmpFile,
			interval:    1 * time.Second,
		}

		provider, err := newCmdProvider(c)
		So(err, ShouldBeNil)
		Convey("When working directory doesn't exist", func(ctx C) {

			errRm := os.RemoveAll(tmpDir)
			So(errRm, ShouldBeNil)

			hook := newZfsHook(provider, "test_pool")
			err := hook.preJob()
			So(err, ShouldNotBeNil)
		})
		Convey("When working directory is not a mount point", func(ctx C) {
			defer os.RemoveAll(tmpDir)

			hook := newZfsHook(provider, "test_pool")
			err := hook.preJob()
			So(err, ShouldNotBeNil)
		})
	})
}
