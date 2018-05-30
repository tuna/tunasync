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
			expectedOutput := fmt.Sprintf(
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
				managerChan := make(chan jobMessage, 10)
				semaphore := make(chan empty, 1)
				job := newMirrorJob(provider)

				go job.Run(managerChan, semaphore)
				// job should not start if we don't start it
				select {
				case <-managerChan:
					So(0, ShouldEqual, 1) // made this fail
				case <-time.After(1 * time.Second):
					So(0, ShouldEqual, 0)
				}

				job.ctrlChan <- jobStart
				for i := 0; i < 2; i++ {
					msg := <-managerChan
					So(msg.status, ShouldEqual, PreSyncing)
					msg = <-managerChan
					So(msg.status, ShouldEqual, Syncing)
					msg = <-managerChan
					So(msg.status, ShouldEqual, Success)
					loggedContent, err := ioutil.ReadFile(provider.LogFile())
					So(err, ShouldBeNil)
					So(string(loggedContent), ShouldEqual, expectedOutput)
					job.ctrlChan <- jobStart
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

				job.ctrlChan <- jobDisable
				select {
				case <-managerChan:
					So(0, ShouldEqual, 1) // made this fail
				case <-job.disabled:
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

			managerChan := make(chan jobMessage, 10)
			semaphore := make(chan empty, 1)
			job := newMirrorJob(provider)

			Convey("If we kill it", func(ctx C) {
				go job.Run(managerChan, semaphore)
				job.ctrlChan <- jobStart

				time.Sleep(1 * time.Second)
				msg := <-managerChan
				So(msg.status, ShouldEqual, PreSyncing)
				msg = <-managerChan
				So(msg.status, ShouldEqual, Syncing)

				job.ctrlChan <- jobStart // should be ignored

				job.ctrlChan <- jobStop

				msg = <-managerChan
				So(msg.status, ShouldEqual, Failed)

				expectedOutput := fmt.Sprintf("%s\n", provider.WorkingDir())
				loggedContent, err := ioutil.ReadFile(provider.LogFile())
				So(err, ShouldBeNil)
				So(string(loggedContent), ShouldEqual, expectedOutput)
				job.ctrlChan <- jobDisable
				<-job.disabled
			})

			Convey("If we don't kill it", func(ctx C) {
				go job.Run(managerChan, semaphore)
				job.ctrlChan <- jobStart

				msg := <-managerChan
				So(msg.status, ShouldEqual, PreSyncing)
				msg = <-managerChan
				So(msg.status, ShouldEqual, Syncing)
				msg = <-managerChan
				So(msg.status, ShouldEqual, Success)

				expectedOutput := fmt.Sprintf(
					"%s\n%s\n",
					provider.WorkingDir(), provider.WorkingDir(),
				)

				loggedContent, err := ioutil.ReadFile(provider.LogFile())
				So(err, ShouldBeNil)
				So(string(loggedContent), ShouldEqual, expectedOutput)
				job.ctrlChan <- jobDisable
				<-job.disabled
			})

			Convey("If we restart it", func(ctx C) {
				go job.Run(managerChan, semaphore)
				job.ctrlChan <- jobStart

				msg := <-managerChan
				So(msg.status, ShouldEqual, PreSyncing)
				msg = <-managerChan
				So(msg.status, ShouldEqual, Syncing)

				job.ctrlChan <- jobRestart

				msg = <-managerChan
				So(msg.status, ShouldEqual, Failed)
				So(msg.msg, ShouldEqual, "killed by manager")

				msg = <-managerChan
				So(msg.status, ShouldEqual, PreSyncing)
				msg = <-managerChan
				So(msg.status, ShouldEqual, Syncing)
				msg = <-managerChan
				So(msg.status, ShouldEqual, Success)

				expectedOutput := fmt.Sprintf(
					"%s\n%s\n",
					provider.WorkingDir(), provider.WorkingDir(),
				)

				loggedContent, err := ioutil.ReadFile(provider.LogFile())
				So(err, ShouldBeNil)
				So(string(loggedContent), ShouldEqual, expectedOutput)
				job.ctrlChan <- jobDisable
				<-job.disabled
			})

			Convey("If we disable it", func(ctx C) {
				go job.Run(managerChan, semaphore)
				job.ctrlChan <- jobStart

				msg := <-managerChan
				So(msg.status, ShouldEqual, PreSyncing)
				msg = <-managerChan
				So(msg.status, ShouldEqual, Syncing)

				job.ctrlChan <- jobDisable

				msg = <-managerChan
				So(msg.status, ShouldEqual, Failed)
				So(msg.msg, ShouldEqual, "killed by manager")

				<-job.disabled
			})

			Convey("If we stop it twice, than start it", func(ctx C) {
				go job.Run(managerChan, semaphore)
				job.ctrlChan <- jobStart

				msg := <-managerChan
				So(msg.status, ShouldEqual, PreSyncing)
				msg = <-managerChan
				So(msg.status, ShouldEqual, Syncing)

				job.ctrlChan <- jobStop

				msg = <-managerChan
				So(msg.status, ShouldEqual, Failed)
				So(msg.msg, ShouldEqual, "killed by manager")

				job.ctrlChan <- jobStop // should be ignored

				job.ctrlChan <- jobStart

				msg = <-managerChan
				So(msg.status, ShouldEqual, PreSyncing)
				msg = <-managerChan
				So(msg.status, ShouldEqual, Syncing)
				msg = <-managerChan
				So(msg.status, ShouldEqual, Success)

				expectedOutput := fmt.Sprintf(
					"%s\n%s\n",
					provider.WorkingDir(), provider.WorkingDir(),
				)

				loggedContent, err := ioutil.ReadFile(provider.LogFile())
				So(err, ShouldBeNil)
				So(string(loggedContent), ShouldEqual, expectedOutput)

				job.ctrlChan <- jobDisable
				<-job.disabled
			})
		})

	})

}
