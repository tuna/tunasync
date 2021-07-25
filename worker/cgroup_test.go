package worker

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	cgv1 "github.com/containerd/cgroups"
	units "github.com/docker/go-units"
	"github.com/moby/moby/pkg/reexec"

	. "github.com/smartystreets/goconvey/convey"
)

func init() {
	reexec.Init()
}

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

		cgcf := cgroupConfig{BasePath: "/sys/fs/cgroup", Group: "tunasync", Subsystem: "cpu"}
		err = initCgroup(&cgcf)
		So(err, ShouldBeNil)
		if cgcf.isUnified {
			So(cgcf.cgMgrV2, ShouldNotBeNil)
		} else {
			So(cgcf.cgMgrV1, ShouldNotBeNil)
		}
		cg := newCgroupHook(provider, cgcf, 0)
		provider.AddHook(cg)

		err = cg.preExec()
		So(err, ShouldBeNil)

		go func() {
			err := provider.Run(make(chan empty, 1))
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

		cgcf := cgroupConfig{BasePath: "/sys/fs/cgroup", Group: "tunasync", Subsystem: "cpu"}
		err = initCgroup(&cgcf)
		So(err, ShouldBeNil)
		if cgcf.isUnified {
			So(cgcf.cgMgrV2, ShouldNotBeNil)
		} else {
			So(cgcf.cgMgrV1, ShouldNotBeNil)
		}
		cg := newCgroupHook(provider, cgcf, 512 * units.MiB)
		provider.AddHook(cg)

		err = cg.preExec()
		So(err, ShouldBeNil)
		if cgcf.isUnified {
			memoLimit, err := ioutil.ReadFile(filepath.Join(cgcf.BasePath, cgcf.Group, provider.Name(), "memory.max"))
			So(err, ShouldBeNil)
			So(strings.Trim(string(memoLimit), "\n"), ShouldEqual, strconv.Itoa(512*1024*1024))
		} else {
			for _, subsys := range(cg.cgMgrV1.Subsystems()) {
				if subsys.Name() == cgv1.Memory {
					memoLimit, err := ioutil.ReadFile(filepath.Join(cgcf.BasePath, "memory", cgcf.Group, provider.Name(), "memory.limit_in_bytes"))
					So(err, ShouldBeNil)
					So(strings.Trim(string(memoLimit), "\n"), ShouldEqual, strconv.Itoa(512*1024*1024))
				}
			}
		}
		cg.postExec()
		So(cg.cgMgrV1, ShouldBeNil)
	})
}
