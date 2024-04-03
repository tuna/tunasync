package worker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
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
			timeout:     100 * time.Second,
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
		So(provider.Timeout(), ShouldEqual, c.timeout)

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
echo "Total file size: 1.33T bytes"
echo "Done"
exit 0
			`
			err = ioutil.WriteFile(scriptFile, []byte(scriptContent), 0755)
			So(err, ShouldBeNil)

			targetDir, _ := filepath.EvalSymlinks(provider.WorkingDir())
			expectedOutput := fmt.Sprintf(
				"syncing to %s\n"+
					"%s\n"+
					"Total file size: 1.33T bytes\n"+
					"Done\n",
				targetDir,
				fmt.Sprintf(
					"-aHvh --no-o --no-g --stats --filter risk .~tmp~/ --exclude .~tmp~/ "+
						"--delete --delete-after --delay-updates --safe-links "+
						"--timeout=120 -6 %s %s",
					provider.upstreamURL, provider.WorkingDir(),
				),
			)

			err = provider.Run(make(chan empty, 1))
			So(err, ShouldBeNil)
			loggedContent, err := ioutil.ReadFile(provider.LogFile())
			So(err, ShouldBeNil)
			So(string(loggedContent), ShouldEqual, expectedOutput)
			// fmt.Println(string(loggedContent))
			So(provider.DataSize(), ShouldEqual, "1.33T")
		})

	})
	Convey("If the rsync program fails", t, func() {
		tmpDir, err := ioutil.TempDir("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)
		tmpFile := filepath.Join(tmpDir, "log_file")

		Convey("in the rsyncProvider", func() {

			c := rsyncConfig{
				name:         "tuna",
				upstreamURL:  "rsync://rsync.tuna.moe/tuna/",
				workingDir:   tmpDir,
				logDir:       tmpDir,
				logFile:      tmpFile,
				extraOptions: []string{"--somethine-invalid"},
				interval:     600 * time.Second,
			}

			provider, err := newRsyncProvider(c)
			So(err, ShouldBeNil)

			err = provider.Run(make(chan empty, 1))
			So(err, ShouldNotBeNil)
			loggedContent, err := ioutil.ReadFile(provider.LogFile())
			So(err, ShouldBeNil)
			So(string(loggedContent), ShouldContainSubstring, "Syntax or usage error")
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
		proxyAddr := "127.0.0.1:1233"

		c := rsyncConfig{
			name:              "tuna",
			upstreamURL:       "rsync://rsync.tuna.moe/tuna/",
			rsyncCmd:          scriptFile,
			username:          "tunasync",
			password:          "tunasyncpassword",
			workingDir:        tmpDir,
			extraOptions:      []string{"--delete-excluded"},
			rsyncTimeoutValue: 30,
			rsyncEnv:          map[string]string{"RSYNC_PROXY": proxyAddr},
			logDir:            tmpDir,
			logFile:           tmpFile,
			useIPv4:           true,
			interval:          600 * time.Second,
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
echo $USER $RSYNC_PASSWORD $RSYNC_PROXY $@
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
					"%s %s %s -aHvh --no-o --no-g --stats --filter risk .~tmp~/ --exclude .~tmp~/ "+
						"--delete --delete-after --delay-updates --safe-links "+
						"--timeout=30 -4 --delete-excluded %s %s",
					provider.username, provider.password, proxyAddr,
					provider.upstreamURL, provider.WorkingDir(),
				),
			)

			err = provider.Run(make(chan empty, 1))
			So(err, ShouldBeNil)
			loggedContent, err := ioutil.ReadFile(provider.LogFile())
			So(err, ShouldBeNil)
			So(string(loggedContent), ShouldEqual, expectedOutput)
			// fmt.Println(string(loggedContent))
		})

	})
}

func TestRsyncProviderWithOverriddenOptions(t *testing.T) {
	Convey("Rsync Provider with overridden options should work", t, func() {
		tmpDir, err := ioutil.TempDir("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)
		scriptFile := filepath.Join(tmpDir, "myrsync")
		tmpFile := filepath.Join(tmpDir, "log_file")

		c := rsyncConfig{
			name:              "tuna",
			upstreamURL:       "rsync://rsync.tuna.moe/tuna/",
			rsyncCmd:          scriptFile,
			workingDir:        tmpDir,
			rsyncNeverTimeout: true,
			overriddenOptions: []string{"-aHvh", "--no-o", "--no-g", "--stats"},
			extraOptions:      []string{"--delete-excluded"},
			logDir:            tmpDir,
			logFile:           tmpFile,
			useIPv6:           true,
			interval:          600 * time.Second,
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
echo $@
sleep 1
echo "Done"
exit 0
			`
			err = ioutil.WriteFile(scriptFile, []byte(scriptContent), 0755)
			So(err, ShouldBeNil)

			targetDir, _ := filepath.EvalSymlinks(provider.WorkingDir())
			expectedOutput := fmt.Sprintf(
				"syncing to %s\n"+
					"-aHvh --no-o --no-g --stats -6 --delete-excluded %s %s\n"+
					"Done\n",
				targetDir,
				provider.upstreamURL,
				provider.WorkingDir(),
			)

			err = provider.Run(make(chan empty, 1))
			So(err, ShouldBeNil)
			loggedContent, err := ioutil.ReadFile(provider.LogFile())
			So(err, ShouldBeNil)
			So(string(loggedContent), ShouldEqual, expectedOutput)
			// fmt.Println(string(loggedContent))
		})

	})
}

func TestRsyncProviderWithDocker(t *testing.T) {
	Convey("Rsync in Docker should work", t, func() {
		tmpDir, err := ioutil.TempDir("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)
		scriptFile := filepath.Join(tmpDir, "myrsync")
		excludeFile := filepath.Join(tmpDir, "exclude.txt")

		g := &Config{
			Global: globalConfig{
				Retry: 2,
			},
			Docker: dockerConfig{
				Enable: true,
				Volumes: []string{
					scriptFile + ":/bin/myrsync",
					"/etc/gai.conf:/etc/gai.conf:ro",
				},
			},
		}
		c := mirrorConfig{
			Name:        "tuna",
			Provider:    provRsync,
			Upstream:    "rsync://rsync.tuna.moe/tuna/",
			Command:     "/bin/myrsync",
			ExcludeFile: excludeFile,
			DockerImage: "alpine:3.8",
			LogDir:      tmpDir,
			MirrorDir:   tmpDir,
			UseIPv6:     true,
			Timeout:     100,
			Interval:    600,
		}

		provider := newMirrorProvider(c, g)

		So(provider.Type(), ShouldEqual, provRsync)
		So(provider.Name(), ShouldEqual, c.Name)
		So(provider.WorkingDir(), ShouldEqual, c.MirrorDir)
		So(provider.LogDir(), ShouldEqual, c.LogDir)

		cmdScriptContent := `#!/bin/sh
#echo "$@"
while [[ $# -gt 0 ]]; do
if [[ "$1" = "--exclude-from" ]]; then
	cat "$2"
	shift
fi
shift
done
`
		err = ioutil.WriteFile(scriptFile, []byte(cmdScriptContent), 0755)
		So(err, ShouldBeNil)
		err = ioutil.WriteFile(excludeFile, []byte("__some_pattern"), 0755)
		So(err, ShouldBeNil)

		for _, hook := range provider.Hooks() {
			err = hook.preExec()
			So(err, ShouldBeNil)
		}
		err = provider.Run(make(chan empty, 1))
		So(err, ShouldBeNil)
		for _, hook := range provider.Hooks() {
			err = hook.postExec()
			So(err, ShouldBeNil)
		}
		loggedContent, err := ioutil.ReadFile(provider.LogFile())
		So(err, ShouldBeNil)
		So(string(loggedContent), ShouldEqual, "__some_pattern")
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

			err = provider.Run(make(chan empty, 1))
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

			err = provider.Run(make(chan empty, 1))
			So(err, ShouldNotBeNil)

		})

		Convey("If a long job is killed", func(ctx C) {
			scriptContent := `#!/bin/bash
sleep 10
			`
			err = ioutil.WriteFile(scriptFile, []byte(scriptContent), 0755)
			So(err, ShouldBeNil)

			started := make(chan empty, 1)
			go func() {
				err := provider.Run(started)
				ctx.So(err, ShouldNotBeNil)
			}()

			<-started
			So(provider.IsRunning(), ShouldBeTrue)
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

			err = provider.Run(make(chan empty, 1))
			So(err, ShouldBeNil)

		})
	})
	Convey("Command Provider with RegExprs should work", t, func(ctx C) {
		tmpDir, err := ioutil.TempDir("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)
		tmpFile := filepath.Join(tmpDir, "log_file")

		c := cmdConfig{
			name:        "run-uptime",
			upstreamURL: "http://mirrors.tuna.moe/",
			command:     "uptime",
			failOnMatch: "",
			sizePattern: "",
			workingDir:  tmpDir,
			logDir:      tmpDir,
			logFile:     tmpFile,
			interval:    600 * time.Second,
		}

		Convey("when fail-on-match regexp matches", func() {
			c.failOnMatch = `[a-z]+`
			provider, err := newCmdProvider(c)
			So(err, ShouldBeNil)

			err = provider.Run(make(chan empty, 1))
			So(err, ShouldNotBeNil)
			So(provider.DataSize(), ShouldBeEmpty)
		})

		Convey("when fail-on-match regexp does not match", func() {
			c.failOnMatch = `load average_`
			provider, err := newCmdProvider(c)
			So(err, ShouldBeNil)

			err = provider.Run(make(chan empty, 1))
			So(err, ShouldBeNil)
		})

		Convey("when fail-on-match regexp meets /dev/null", func() {
			c.failOnMatch = `load average_`
			c.logFile = "/dev/null"
			provider, err := newCmdProvider(c)
			So(err, ShouldBeNil)

			err = provider.Run(make(chan empty, 1))
			So(err, ShouldNotBeNil)
		})

		Convey("when size-pattern regexp matches", func() {
			c.sizePattern = `load average: ([\d\.]+)`
			provider, err := newCmdProvider(c)
			So(err, ShouldBeNil)

			err = provider.Run(make(chan empty, 1))
			So(err, ShouldBeNil)
			So(provider.DataSize(), ShouldNotBeEmpty)
			_, err = strconv.ParseFloat(provider.DataSize(), 32)
			So(err, ShouldBeNil)
		})

		Convey("when size-pattern regexp does not match", func() {
			c.sizePattern = `load ave: ([\d\.]+)`
			provider, err := newCmdProvider(c)
			So(err, ShouldBeNil)

			err = provider.Run(make(chan empty, 1))
			So(err, ShouldBeNil)
			So(provider.DataSize(), ShouldBeEmpty)
		})

		Convey("when size-pattern regexp meets /dev/null", func() {
			c.sizePattern = `load ave: ([\d\.]+)`
			c.logFile = "/dev/null"
			provider, err := newCmdProvider(c)
			So(err, ShouldBeNil)

			err = provider.Run(make(chan empty, 1))
			So(err, ShouldNotBeNil)
			So(provider.DataSize(), ShouldBeEmpty)
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
			name:              "tuna-two-stage-rsync",
			upstreamURL:       "rsync://mirrors.tuna.moe/",
			stage1Profile:     "debian",
			rsyncCmd:          scriptFile,
			workingDir:        tmpDir,
			logDir:            tmpDir,
			logFile:           tmpFile,
			useIPv6:           true,
			excludeFile:       tmpFile,
			rsyncTimeoutValue: 30,
			extraOptions:      []string{"--delete-excluded", "--cache"},
			username:          "hello",
			password:          "world",
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

			err = provider.Run(make(chan empty, 2))
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
					"-aHvh --no-o --no-g --stats --filter risk .~tmp~/ --exclude .~tmp~/ --safe-links "+
						"--include=*.diff/ --include=by-hash/ --exclude=*.diff/Index --exclude=Contents* --exclude=Packages* --exclude=Sources* --exclude=Release* --exclude=InRelease --exclude=i18n/* --exclude=dep11/* --exclude=installer-*/current --exclude=ls-lR* --timeout=30 -6 "+
						"--exclude-from %s %s %s",
					provider.excludeFile, provider.upstreamURL, provider.WorkingDir(),
				),
				targetDir,
				fmt.Sprintf(
					"-aHvh --no-o --no-g --stats --filter risk .~tmp~/ --exclude .~tmp~/ "+
						"--delete --delete-after --delay-updates --safe-links "+
						"--delete-excluded --cache --timeout=30 -6 --exclude-from %s %s %s",
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
sleep 10
exit 0
			`
			err = ioutil.WriteFile(scriptFile, []byte(scriptContent), 0755)
			So(err, ShouldBeNil)

			started := make(chan empty, 2)
			go func() {
				err := provider.Run(started)
				ctx.So(err, ShouldNotBeNil)
			}()

			<-started
			So(provider.IsRunning(), ShouldBeTrue)
			time.Sleep(1 * time.Second)
			err = provider.Terminate()
			So(err, ShouldBeNil)

			expectedOutput := fmt.Sprintf(
				"-aHvh --no-o --no-g --stats --filter risk .~tmp~/ --exclude .~tmp~/ --safe-links "+
					"--include=*.diff/ --include=by-hash/ --exclude=*.diff/Index --exclude=Contents* --exclude=Packages* --exclude=Sources* --exclude=Release* --exclude=InRelease --exclude=i18n/* --exclude=dep11/* --exclude=installer-*/current --exclude=ls-lR* --timeout=30 -6 "+
					"--exclude-from %s %s %s\n",
				provider.excludeFile, provider.upstreamURL, provider.WorkingDir(),
			)

			loggedContent, err := ioutil.ReadFile(provider.LogFile())
			So(err, ShouldBeNil)
			So(string(loggedContent), ShouldStartWith, expectedOutput)
			// fmt.Println(string(loggedContent))
		})
	})

	Convey("If the rsync program fails", t, func(ctx C) {
		tmpDir, err := ioutil.TempDir("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)
		tmpFile := filepath.Join(tmpDir, "log_file")

		Convey("in the twoStageRsyncProvider", func() {

			c := twoStageRsyncConfig{
				name:          "tuna-two-stage-rsync",
				upstreamURL:   "rsync://0.0.0.1/",
				stage1Profile: "debian",
				workingDir:    tmpDir,
				logDir:        tmpDir,
				logFile:       tmpFile,
				excludeFile:   tmpFile,
			}

			provider, err := newTwoStageRsyncProvider(c)
			So(err, ShouldBeNil)

			err = provider.Run(make(chan empty, 2))
			So(err, ShouldNotBeNil)
			loggedContent, err := ioutil.ReadFile(provider.LogFile())
			So(err, ShouldBeNil)
			So(string(loggedContent), ShouldContainSubstring, "Error in socket I/O")

		})
	})
}
