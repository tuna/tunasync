package worker

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCgroup(t *testing.T) {
	Convey("Cgroup Should Work", t, func(ctx C) {
		tmpDir, err := ioutil.TempDir("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)
		cmdScript := filepath.Join(tmpDir, "cmd.sh")
		daemonScript := filepath.Join(tmpDir, "daemon.sh")
		tmpFile := filepath.Join(tmpDir, "log_file")
		bgPidfile := filepath.Join(tmpDir, "bg.pid")

		c := cmdConfig{
			name:        "tuna-cgroup",
			upstreamURL: "http://mirrors.tuna.moe/",
			command:     cmdScript + " " + daemonScript,
			workingDir:  tmpDir,
			logDir:      tmpDir,
			logFile:     tmpFile,
			interval:    600 * time.Second,
			env: map[string]string{
				"BG_PIDFILE": bgPidfile,
			},
		}
		cmdScriptContent := `#!/bin/bash
redirect-std() {
    [[ -t 0 ]] && exec </dev/null
    [[ -t 1 ]] && exec >/dev/null
    [[ -t 2 ]] && exec 2>/dev/null
}

# close all non-std* fds
close-fds() {
    eval exec {3..255}\>\&-
}
 
# full daemonization of external command with setsid
daemonize() {
    (
        redirect-std    
        cd /            
        close-fds       
        exec setsid "$@"
    ) &
}

echo $$
daemonize $@
sleep 5
`
		daemonScriptContent := `#!/bin/bash
echo $$ > $BG_PIDFILE
sleep 30
`
		err = ioutil.WriteFile(cmdScript, []byte(cmdScriptContent), 0755)
		So(err, ShouldBeNil)
		err = ioutil.WriteFile(daemonScript, []byte(daemonScriptContent), 0755)
		So(err, ShouldBeNil)

		provider, err := newCmdProvider(c)
		So(err, ShouldBeNil)

		initCgroup("/sys/fs/cgroup")
		cg := newCgroupHook(provider, "/sys/fs/cgroup", "tunasync")
		provider.AddHook(cg)

		err = cg.preExec()
		So(err, ShouldBeNil)

		go func() {
			err = provider.Run()
			ctx.So(err, ShouldNotBeNil)
		}()

		time.Sleep(1 * time.Second)
		// Deamon should be started
		daemonPidBytes, err := ioutil.ReadFile(bgPidfile)
		So(err, ShouldBeNil)
		daemonPid := strings.Trim(string(daemonPidBytes), " \n")
		logger.Debug("daemon pid: %s", daemonPid)
		procDir := filepath.Join("/proc", daemonPid)
		_, err = os.Stat(procDir)
		So(err, ShouldBeNil)

		err = provider.Terminate()
		So(err, ShouldBeNil)

		// Deamon won't be killed
		_, err = os.Stat(procDir)
		So(err, ShouldBeNil)

		// Deamon can be killed by cgroup killer
		cg.postExec()
		_, err = os.Stat(procDir)
		So(os.IsNotExist(err), ShouldBeTrue)

	})

	Convey("Rsync Memory Should Be Limited", t, func() {
		tmpDir, err := ioutil.TempDir("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)
		scriptFile := filepath.Join(tmpDir, "myrsync")
		tmpFile := filepath.Join(tmpDir, "log_file")

		c := rsyncConfig{
			name:        "tuna-cgroup",
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

		initCgroup("/sys/fs/cgroup")
		cg := newCgroupHook(provider, "/sys/fs/cgroup", "tunasync")
		provider.AddHook(cg)

		cg.preExec()
		if cgSubsystem == "memory" {
			memoLimit, err := ioutil.ReadFile(filepath.Join(cg.basePath, "memory", cg.baseGroup, provider.Name(), "memory.limit_in_bytes"))
			So(err, ShouldBeNil)
			So(strings.Trim(string(memoLimit), "\n"), ShouldEqual, strconv.Itoa(128*1024*1024))
		}
		cg.postExec()
	})
}
