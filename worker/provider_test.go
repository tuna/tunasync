package worker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestCmdProvider(t *testing.T) {
	Convey("Command Provider should work", t, func(ctx C) {
		tmpDir, err := ioutil.TempDir("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)
		scriptFile := filepath.Join(tmpDir, "cmd.sh")
		tmpFile := filepath.Join(tmpDir, "log_file")

		c := cmdConfig{
			name:        "tuna-cmd",
			upstreamURL: "http://mirrors.tuna.moe/",
			command:     "bash " + scriptFile,
			workingDir:  tmpDir,
			logDir:      tmpDir,
			logFile:     tmpFile,
			interval:    600,
			env: map[string]string{
				"AOSP_REPO_BIN": "/usr/local/bin/repo",
			},
		}

		provider, err := newCmdProvider(c)
		So(err, ShouldBeNil)

		So(provider.Name(), ShouldEqual, c.name)
		So(provider.WorkingDir(), ShouldEqual, c.workingDir)
		So(provider.LogDir(), ShouldEqual, c.logDir)
		So(provider.LogFile(), ShouldEqual, c.logFile)
		So(provider.Interval(), ShouldEqual, c.interval)

		Convey("Let's try to run a simple command", func() {
			scriptContent := `#!/bin/bash
echo $TUNASYNC_WORKING_DIR
echo $TUNASYNC_MIRROR_NAME
echo $TUNASYNC_UPSTREAM_URL
echo $TUNASYNC_LOG_FILE
echo $AOSP_REPO_BIN
`
			exceptedOutput := fmt.Sprintf(
				"%s\n%s\n%s\n%s\n%s\n",
				provider.WorkingDir(),
				provider.Name(),
				provider.upstreamURL,
				provider.LogFile(),
				"/usr/local/bin/repo",
			)
			err = ioutil.WriteFile(scriptFile, []byte(scriptContent), 0755)
			So(err, ShouldBeNil)
			readedScriptContent, err := ioutil.ReadFile(scriptFile)
			So(err, ShouldBeNil)
			So(readedScriptContent, ShouldResemble, []byte(scriptContent))

			err = provider.Run()
			So(err, ShouldBeNil)

			loggedContent, err := ioutil.ReadFile(provider.LogFile())
			So(err, ShouldBeNil)
			So(string(loggedContent), ShouldEqual, exceptedOutput)
		})

		Convey("If a command fails", func() {
			scriptContent := `exit 1`
			err = ioutil.WriteFile(scriptFile, []byte(scriptContent), 0755)
			So(err, ShouldBeNil)
			readedScriptContent, err := ioutil.ReadFile(scriptFile)
			So(err, ShouldBeNil)
			So(readedScriptContent, ShouldResemble, []byte(scriptContent))

			err = provider.Run()
			So(err, ShouldNotBeNil)

		})

		Convey("If a long job is killed", func(ctx C) {
			scriptContent := `#!/bin/bash
sleep 5
			`
			err = ioutil.WriteFile(scriptFile, []byte(scriptContent), 0755)
			So(err, ShouldBeNil)

			go func() {
				err = provider.Run()
				ctx.So(err, ShouldNotBeNil)
			}()

			time.Sleep(1 * time.Second)
			err = provider.Terminate()
			So(err, ShouldBeNil)

		})
	})
}
