package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	. "github.com/tuna/tunasync/internal"
)

func TestLogLimiter(t *testing.T) {
	Convey("LogLimiter should work", t, func(ctx C) {
		tmpDir, _ := os.MkdirTemp("", "tunasync")
		tmpLogDir, err := os.MkdirTemp("", "tunasync-log")
		defer os.RemoveAll(tmpDir)
		defer os.RemoveAll(tmpLogDir)
		So(err, ShouldBeNil)
		scriptFile := filepath.Join(tmpDir, "cmd.sh")

		c := cmdConfig{
			name:        "tuna-loglimit",
			upstreamURL: "http://mirrors.tuna.moe/",
			command:     scriptFile,
			workingDir:  tmpDir,
			logDir:      tmpLogDir,
			logFile:     filepath.Join(tmpLogDir, "latest.log"),
			interval:    600 * time.Second,
		}

		provider, err := newCmdProvider(c)
		So(err, ShouldBeNil)
		limiter := newLogLimiter(provider)
		provider.AddHook(limiter)

		Convey("If logs are created simply", func() {
			for i := 0; i < 15; i++ {
				fn := filepath.Join(tmpLogDir, fmt.Sprintf("%s-%d.log", provider.Name(), i))
				f, _ := os.Create(fn)
				// time.Sleep(1 * time.Second)
				f.Close()
			}

			matches, _ := filepath.Glob(filepath.Join(tmpLogDir, "*.log"))
			So(len(matches), ShouldEqual, 15)

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
			logFile := provider.LogFile()
			msg = <-managerChan
			So(msg.status, ShouldEqual, Success)

			job.ctrlChan <- jobDisable

			So(logFile, ShouldNotEqual, provider.LogFile())

			matches, _ = filepath.Glob(filepath.Join(tmpLogDir, "*.log"))
			So(len(matches), ShouldEqual, 10)

			expectedOutput := fmt.Sprintf(
				"%s\n%s\n%s\n%s\n",
				provider.WorkingDir(),
				provider.Name(),
				provider.upstreamURL,
				logFile,
			)

			loggedContent, err := os.ReadFile(filepath.Join(provider.LogDir(), "latest"))
			So(err, ShouldBeNil)
			So(string(loggedContent), ShouldEqual, expectedOutput)
		})

		Convey("If job failed simply", func() {
			managerChan := make(chan jobMessage)
			semaphore := make(chan empty, 1)
			job := newMirrorJob(provider)

			scriptContent := `#!/bin/bash
echo $TUNASYNC_WORKING_DIR
echo $TUNASYNC_MIRROR_NAME
echo $TUNASYNC_UPSTREAM_URL
echo $TUNASYNC_LOG_FILE
sleep 5
			`

			err = os.WriteFile(scriptFile, []byte(scriptContent), 0755)
			So(err, ShouldBeNil)

			go job.Run(managerChan, semaphore)
			job.ctrlChan <- jobStart
			msg := <-managerChan
			So(msg.status, ShouldEqual, PreSyncing)
			msg = <-managerChan
			So(msg.status, ShouldEqual, Syncing)
			logFile := provider.LogFile()

			time.Sleep(1 * time.Second)
			job.ctrlChan <- jobStop

			msg = <-managerChan
			So(msg.status, ShouldEqual, Failed)

			job.ctrlChan <- jobDisable
			<-job.disabled

			So(logFile, ShouldNotEqual, provider.LogFile())

			expectedOutput := fmt.Sprintf(
				"%s\n%s\n%s\n%s\n",
				provider.WorkingDir(),
				provider.Name(),
				provider.upstreamURL,
				logFile,
			)

			loggedContent, err := os.ReadFile(filepath.Join(provider.LogDir(), "latest"))
			So(err, ShouldBeNil)
			So(string(loggedContent), ShouldEqual, expectedOutput)
			loggedContent, err = os.ReadFile(logFile + ".fail")
			So(err, ShouldBeNil)
			So(string(loggedContent), ShouldEqual, expectedOutput)
		})

	})
}
