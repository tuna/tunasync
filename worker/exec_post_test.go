package worker

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	. "github.com/tuna/tunasync/internal"
)

func TestExecPost(t *testing.T) {
	Convey("ExecPost should work", t, func(ctx C) {
		tmpDir, err := os.MkdirTemp("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)
		scriptFile := filepath.Join(tmpDir, "cmd.sh")

		c := cmdConfig{
			name:        "tuna-exec-post",
			upstreamURL: "http://mirrors.tuna.moe/",
			command:     scriptFile,
			workingDir:  tmpDir,
			logDir:      tmpDir,
			logFile:     filepath.Join(tmpDir, "latest.log"),
			interval:    600 * time.Second,
		}

		provider, err := newCmdProvider(c)
		So(err, ShouldBeNil)

		Convey("On success", func() {
			hook, err := newExecPostHook(provider, execOnSuccess, "bash -c 'echo ${TUNASYNC_JOB_EXIT_STATUS} > ${TUNASYNC_WORKING_DIR}/exit_status'")
			So(err, ShouldBeNil)
			provider.AddHook(hook)
			managerChan := make(chan jobMessage)
			semaphore := make(chan empty, 1)
			job := newMirrorJob(provider)

			scriptContent := `#!/bin/bash
echo $TUNASYNC_WORKING_DIR
echo $TUNASYNC_MIRROR_NAME
echo $TUNASYNC_UPSTREAM_URL
echo $TUNASYNC_LOG_FILE
			`

			err = os.WriteFile(scriptFile, []byte(scriptContent), 0755)
			So(err, ShouldBeNil)

			go job.Run(managerChan, semaphore)
			job.ctrlChan <- jobStart
			msg := <-managerChan
			So(msg.status, ShouldEqual, PreSyncing)
			msg = <-managerChan
			So(msg.status, ShouldEqual, Syncing)
			msg = <-managerChan
			So(msg.status, ShouldEqual, Success)

			time.Sleep(200 * time.Millisecond)
			job.ctrlChan <- jobDisable
			<-job.disabled

			expectedOutput := "success\n"

			outputContent, err := os.ReadFile(filepath.Join(provider.WorkingDir(), "exit_status"))
			So(err, ShouldBeNil)
			So(string(outputContent), ShouldEqual, expectedOutput)
		})

		Convey("On failure", func() {
			hook, err := newExecPostHook(provider, execOnFailure, "bash -c 'echo ${TUNASYNC_JOB_EXIT_STATUS} > ${TUNASYNC_WORKING_DIR}/exit_status'")
			So(err, ShouldBeNil)
			provider.AddHook(hook)
			managerChan := make(chan jobMessage)
			semaphore := make(chan empty, 1)
			job := newMirrorJob(provider)

			scriptContent := `#!/bin/bash
echo $TUNASYNC_WORKING_DIR
echo $TUNASYNC_MIRROR_NAME
echo $TUNASYNC_UPSTREAM_URL
echo $TUNASYNC_LOG_FILE
exit 1
			`

			err = os.WriteFile(scriptFile, []byte(scriptContent), 0755)
			So(err, ShouldBeNil)

			go job.Run(managerChan, semaphore)
			job.ctrlChan <- jobStart
			msg := <-managerChan
			So(msg.status, ShouldEqual, PreSyncing)
			for i := 0; i < defaultMaxRetry; i++ {
				msg = <-managerChan
				So(msg.status, ShouldEqual, Syncing)
				msg = <-managerChan
				So(msg.status, ShouldEqual, Failed)
			}

			time.Sleep(200 * time.Millisecond)
			job.ctrlChan <- jobDisable
			<-job.disabled

			expectedOutput := "failure\n"

			outputContent, err := os.ReadFile(filepath.Join(provider.WorkingDir(), "exit_status"))
			So(err, ShouldBeNil)
			So(string(outputContent), ShouldEqual, expectedOutput)
		})
	})
}
