package worker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	. "github.com/tuna/tunasync/internal"
)

func TestMirrorJob(t *testing.T) {

	InitLogger(true, true, false)

	Convey("MirrorJob should work", t, func(ctx C) {
		tmpDir, err := ioutil.TempDir("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)
		scriptFile := filepath.Join(tmpDir, "cmd.sh")
		tmpFile := filepath.Join(tmpDir, "log_file")

		c := cmdConfig{
			name:        "tuna-cmd-jobtest",
			upstreamURL: "http://mirrors.tuna.moe/",
			command:     "bash " + scriptFile,
			workingDir:  tmpDir,
			logDir:      tmpDir,
			logFile:     tmpFile,
			interval:    1 * time.Second,
		}

		provider, err := newCmdProvider(c)
		So(err, ShouldBeNil)

		So(provider.Name(), ShouldEqual, c.name)
		So(provider.WorkingDir(), ShouldEqual, c.workingDir)
		So(provider.LogDir(), ShouldEqual, c.logDir)
		So(provider.LogFile(), ShouldEqual, c.logFile)
		So(provider.Interval(), ShouldEqual, c.interval)

		Convey("For a normal mirror job", func(ctx C) {
			scriptContent := `#!/bin/bash
			echo $TUNASYNC_WORKING_DIR
			echo $TUNASYNC_MIRROR_NAME
			echo $TUNASYNC_UPSTREAM_URL
			echo $TUNASYNC_LOG_FILE
			`
			exceptedOutput := fmt.Sprintf(
				"%s\n%s\n%s\n%s\n",
				provider.WorkingDir(),
				provider.Name(),
				provider.upstreamURL,
				provider.LogFile(),
			)
			err = ioutil.WriteFile(scriptFile, []byte(scriptContent), 0755)
			So(err, ShouldBeNil)
			readedScriptContent, err := ioutil.ReadFile(scriptFile)
			So(err, ShouldBeNil)
			So(readedScriptContent, ShouldResemble, []byte(scriptContent))

			Convey("If we let it run several times", func(ctx C) {
				ctrlChan := make(chan ctrlAction)
				managerChan := make(chan jobMessage, 10)
				semaphore := make(chan empty, 1)

				go runMirrorJob(provider, ctrlChan, managerChan, semaphore)
				for i := 0; i < 2; i++ {
					msg := <-managerChan
					So(msg.status, ShouldEqual, PreSyncing)
					msg = <-managerChan
					So(msg.status, ShouldEqual, Syncing)
					msg = <-managerChan
					So(msg.status, ShouldEqual, Success)
					loggedContent, err := ioutil.ReadFile(provider.LogFile())
					So(err, ShouldBeNil)
					So(string(loggedContent), ShouldEqual, exceptedOutput)
					ctrlChan <- jobStart
				}
				select {
				case msg := <-managerChan:
					So(msg.status, ShouldEqual, PreSyncing)
					msg = <-managerChan
					So(msg.status, ShouldEqual, Syncing)
					msg = <-managerChan
					So(msg.status, ShouldEqual, Success)

				case <-time.After(2 * time.Second):
					So(0, ShouldEqual, 1)
				}

				ctrlChan <- jobDisable
				select {
				case <-managerChan:
					So(0, ShouldEqual, 1) // made this fail
				case <-time.After(2 * time.Second):
					So(0, ShouldEqual, 0)
				}
			})

		})

		Convey("When running long jobs", func(ctx C) {
			scriptContent := `#!/bin/bash
echo $TUNASYNC_WORKING_DIR
sleep 5
echo $TUNASYNC_WORKING_DIR
			`
			err = ioutil.WriteFile(scriptFile, []byte(scriptContent), 0755)
			So(err, ShouldBeNil)

			ctrlChan := make(chan ctrlAction)
			managerChan := make(chan jobMessage, 10)
			semaphore := make(chan empty, 1)

			Convey("If we kill it", func(ctx C) {
				go runMirrorJob(provider, ctrlChan, managerChan, semaphore)

				time.Sleep(1 * time.Second)
				msg := <-managerChan
				So(msg.status, ShouldEqual, PreSyncing)
				msg = <-managerChan
				So(msg.status, ShouldEqual, Syncing)

				ctrlChan <- jobStop

				msg = <-managerChan
				So(msg.status, ShouldEqual, Failed)

				exceptedOutput := fmt.Sprintf("%s\n", provider.WorkingDir())
				loggedContent, err := ioutil.ReadFile(provider.LogFile())
				So(err, ShouldBeNil)
				So(string(loggedContent), ShouldEqual, exceptedOutput)
				ctrlChan <- jobDisable
			})

			Convey("If we don't kill it", func(ctx C) {
				go runMirrorJob(provider, ctrlChan, managerChan, semaphore)
				msg := <-managerChan
				So(msg.status, ShouldEqual, PreSyncing)
				msg = <-managerChan
				So(msg.status, ShouldEqual, Syncing)
				msg = <-managerChan
				So(msg.status, ShouldEqual, Success)

				exceptedOutput := fmt.Sprintf(
					"%s\n%s\n",
					provider.WorkingDir(), provider.WorkingDir(),
				)

				loggedContent, err := ioutil.ReadFile(provider.LogFile())
				So(err, ShouldBeNil)
				So(string(loggedContent), ShouldEqual, exceptedOutput)
				ctrlChan <- jobDisable
			})
		})

	})

}
