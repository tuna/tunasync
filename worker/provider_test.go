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
		tmpDir, err := ioutil.TempDir("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)
		scriptFile := filepath.Join(tmpDir, "myrsync")
		tmpFile := filepath.Join(tmpDir, "log_file")

		c := rsyncConfig{
			name:        "tuna",
			upstreamURL: "rsync://rsync.tuna.moe/tuna/",
			rsyncCmd:    scriptFile,
			workingDir:  tmpDir,
			logDir:      tmpDir,
			logFile:     tmpFile,
			useIPv6:     true,
			interval:    600 * time.Second,
		}

		provider, err := newRsyncProvider(c)
		So(err, ShouldBeNil)

		So(provider.Type(), ShouldEqual, provRsync)
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

		Convey("Let's try a run", func() {
			scriptContent := `#!/bin/bash
echo "syncing to $(pwd)"
echo $RSYNC_PASSWORD $@
sleep 1
echo "Done"
exit 0
			`
			err = ioutil.WriteFile(scriptFile, []byte(scriptContent), 0755)
			So(err, ShouldBeNil)

			targetDir, _ := filepath.EvalSymlinks(provider.WorkingDir())
			expectedOutput := fmt.Sprintf(
				"syncing to %s\n"+
					"%s\n"+
					"Done\n",
				targetDir,
				fmt.Sprintf(
					"-aHvh --no-o --no-g --stats --exclude .~tmp~/ "+
						"--delete --delete-after --delay-updates --safe-links "+
						"--timeout=120 --contimeout=120 -6 %s %s",
					provider.upstreamURL, provider.WorkingDir(),
				),
			)

			err = provider.Run()
			So(err, ShouldBeNil)
			loggedContent, err := ioutil.ReadFile(provider.LogFile())
			So(err, ShouldBeNil)
			So(string(loggedContent), ShouldEqual, expectedOutput)
			// fmt.Println(string(loggedContent))
		})

	})
}

func TestRsyncProviderWithAuthentication(t *testing.T) {
	Convey("Rsync Provider with password should work", t, func() {
		tmpDir, err := ioutil.TempDir("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)
		scriptFile := filepath.Join(tmpDir, "myrsync")
		tmpFile := filepath.Join(tmpDir, "log_file")

		c := rsyncConfig{
			name:        "tuna",
			upstreamURL: "rsync://rsync.tuna.moe/tuna/",
			rsyncCmd:    scriptFile,
			username:    "tunasync",
			password:    "tunasyncpassword",
			workingDir:  tmpDir,
			logDir:      tmpDir,
			logFile:     tmpFile,
			useIPv6:     true,
			interval:    600 * time.Second,
		}

		provider, err := newRsyncProvider(c)
		So(err, ShouldBeNil)

		So(provider.Name(), ShouldEqual, c.name)
		So(provider.WorkingDir(), ShouldEqual, c.workingDir)
		So(provider.LogDir(), ShouldEqual, c.logDir)
		So(provider.LogFile(), ShouldEqual, c.logFile)
		So(provider.Interval(), ShouldEqual, c.interval)

		Convey("Let's try a run", func() {
			scriptContent := `#!/bin/bash
echo "syncing to $(pwd)"
echo $USER $RSYNC_PASSWORD $@
sleep 1
echo "Done"
exit 0
			`
			err = ioutil.WriteFile(scriptFile, []byte(scriptContent), 0755)
			So(err, ShouldBeNil)

			targetDir, _ := filepath.EvalSymlinks(provider.WorkingDir())
			expectedOutput := fmt.Sprintf(
				"syncing to %s\n"+
					"%s\n"+
					"Done\n",
				targetDir,
				fmt.Sprintf(
					"%s %s -aHvh --no-o --no-g --stats --exclude .~tmp~/ "+
						"--delete --delete-after --delay-updates --safe-links "+
						"--timeout=120 --contimeout=120 -6 %s %s",
					provider.username, provider.password, provider.upstreamURL, provider.WorkingDir(),
				),
			)

			err = provider.Run()
			So(err, ShouldBeNil)
			loggedContent, err := ioutil.ReadFile(provider.LogFile())
			So(err, ShouldBeNil)
			So(string(loggedContent), ShouldEqual, expectedOutput)
			// fmt.Println(string(loggedContent))
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
			interval:    600 * time.Second,
			env: map[string]string{
				"AOSP_REPO_BIN": "/usr/local/bin/repo",
			},
		}

		provider, err := newCmdProvider(c)
		So(err, ShouldBeNil)

		So(provider.Type(), ShouldEqual, provCommand)
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
			expectedOutput := fmt.Sprintf(
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
			So(string(loggedContent), ShouldEqual, expectedOutput)
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
	Convey("Command Provider without log file should work", t, func(ctx C) {
		tmpDir, err := ioutil.TempDir("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)

		c := cmdConfig{
			name:        "run-ls",
			upstreamURL: "http://mirrors.tuna.moe/",
			command:     "ls",
			workingDir:  tmpDir,
			logDir:      tmpDir,
			logFile:     "/dev/null",
			interval:    600 * time.Second,
		}

		provider, err := newCmdProvider(c)
		So(err, ShouldBeNil)

		So(provider.IsMaster(), ShouldEqual, false)
		So(provider.ZFS(), ShouldBeNil)
		So(provider.Type(), ShouldEqual, provCommand)
		So(provider.Name(), ShouldEqual, c.name)
		So(provider.WorkingDir(), ShouldEqual, c.workingDir)
		So(provider.LogDir(), ShouldEqual, c.logDir)
		So(provider.LogFile(), ShouldEqual, c.logFile)
		So(provider.Interval(), ShouldEqual, c.interval)

		Convey("Run the command", func() {

			err = provider.Run()
			So(err, ShouldBeNil)

		})
	})
}

func TestTwoStageRsyncProvider(t *testing.T) {
	Convey("TwoStageRsync Provider should work", t, func(ctx C) {
		tmpDir, err := ioutil.TempDir("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)
		scriptFile := filepath.Join(tmpDir, "myrsync")
		tmpFile := filepath.Join(tmpDir, "log_file")

		c := twoStageRsyncConfig{
			name:          "tuna-two-stage-rsync",
			upstreamURL:   "rsync://mirrors.tuna.moe/",
			stage1Profile: "debian",
			rsyncCmd:      scriptFile,
			workingDir:    tmpDir,
			logDir:        tmpDir,
			logFile:       tmpFile,
			useIPv6:       true,
			excludeFile:   tmpFile,
			username:      "hello",
			password:      "world",
		}

		provider, err := newTwoStageRsyncProvider(c)
		So(err, ShouldBeNil)

		So(provider.Type(), ShouldEqual, provTwoStageRsync)
		So(provider.Name(), ShouldEqual, c.name)
		So(provider.WorkingDir(), ShouldEqual, c.workingDir)
		So(provider.LogDir(), ShouldEqual, c.logDir)
		So(provider.LogFile(), ShouldEqual, c.logFile)
		So(provider.Interval(), ShouldEqual, c.interval)

		Convey("Try a command", func(ctx C) {
			scriptContent := `#!/bin/bash
echo "syncing to $(pwd)"
echo $@
sleep 1
echo "Done"
exit 0
			`
			err = ioutil.WriteFile(scriptFile, []byte(scriptContent), 0755)
			So(err, ShouldBeNil)

			err = provider.Run()
			So(err, ShouldBeNil)

			targetDir, _ := filepath.EvalSymlinks(provider.WorkingDir())
			expectedOutput := fmt.Sprintf(
				"syncing to %s\n"+
					"%s\n"+
					"Done\n"+
					"syncing to %s\n"+
					"%s\n"+
					"Done\n",
				targetDir,
				fmt.Sprintf(
					"-aHvh --no-o --no-g --stats --exclude .~tmp~/ --safe-links "+
						"--timeout=120 --contimeout=120 --exclude dists/ -6 "+
						"--exclude-from %s %s %s",
					provider.excludeFile, provider.upstreamURL, provider.WorkingDir(),
				),
				targetDir,
				fmt.Sprintf(
					"-aHvh --no-o --no-g --stats --exclude .~tmp~/ "+
						"--delete --delete-after --delay-updates --safe-links "+
						"--timeout=120 --contimeout=120 -6 --exclude-from %s %s %s",
					provider.excludeFile, provider.upstreamURL, provider.WorkingDir(),
				),
			)

			loggedContent, err := ioutil.ReadFile(provider.LogFile())
			So(err, ShouldBeNil)
			So(string(loggedContent), ShouldEqual, expectedOutput)
			// fmt.Println(string(loggedContent))

		})
		Convey("Try terminating", func(ctx C) {
			scriptContent := `#!/bin/bash
echo $@
sleep 4
exit 0
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

			expectedOutput := fmt.Sprintf(
				"-aHvh --no-o --no-g --stats --exclude .~tmp~/ --safe-links "+
					"--timeout=120 --contimeout=120 --exclude dists/ -6 "+
					"--exclude-from %s %s %s\n",
				provider.excludeFile, provider.upstreamURL, provider.WorkingDir(),
			)

			loggedContent, err := ioutil.ReadFile(provider.LogFile())
			So(err, ShouldBeNil)
			So(string(loggedContent), ShouldEqual, expectedOutput)
			// fmt.Println(string(loggedContent))
		})
	})
}
