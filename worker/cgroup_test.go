package worker

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	cgv1 "github.com/containerd/cgroups"
	cgv2 "github.com/containerd/cgroups/v2"
	units "github.com/docker/go-units"
	"github.com/moby/moby/pkg/reexec"

	. "github.com/smartystreets/goconvey/convey"
)

func init() {
	_, testReexec := os.LookupEnv("TESTREEXEC")
	if !testReexec {
		reexec.Init()
	}
}

func TestReexec(t *testing.T) {
	testCase, testReexec := os.LookupEnv("TESTREEXEC")
	if !testReexec {
		return
	}
	for len(os.Args) > 1 {
		thisArg := os.Args[1]
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		if thisArg == "--" {
			break
		}
	}
	switch testCase {
	case "1":
		Convey("Reexec should panic when command not found", t, func(ctx C) {
			So(func() {
				reexec.Init()
			}, ShouldPanicWith, exec.ErrNotFound)
		})
	case "2":
		Convey("Reexec should run when fd 3 is not open", t, func(ctx C) {
			So((func() error {
				pipe := os.NewFile(3, "pipe")
				if pipe == nil {
					return errors.New("pipe is nil")
				} else {
					_, err := pipe.Stat()
					return err
				}
			})(), ShouldNotBeNil)
			So(func() {
				reexec.Init()
			}, ShouldPanicWith, syscall.ENOEXEC)
		})
	case "3":
		Convey("Reexec should fail when fd 3 is sent with abrt cmd", t, func(ctx C) {
			So(func() {
				reexec.Init()
			}, ShouldPanicWith, "Exited on request")
		})
	case "4":
		Convey("Reexec should run when fd 3 is sent with cont cmd", t, func(ctx C) {
			So(func() {
				reexec.Init()
			}, ShouldPanicWith, syscall.ENOEXEC)
		})
	case "5":
		Convey("Reexec should not be triggered when argv[0] is not reexec", t, func(ctx C) {
			So(func() {
				reexec.Init()
			}, ShouldNotPanic)
		})
	}
}

func TestCgroup(t *testing.T) {
	var cgcf *cgroupConfig
	Convey("init cgroup", t, func(ctx C) {
		_, useCurrentCgroup := os.LookupEnv("USECURCGROUP")
		cgcf = &cgroupConfig{BasePath: "/sys/fs/cgroup", Group: "tunasync", Subsystem: "cpu"}
		if useCurrentCgroup {
			cgcf.Group = ""
		}
		err := initCgroup(cgcf)
		So(err, ShouldBeNil)
		if cgcf.isUnified {
			So(cgcf.cgMgrV2, ShouldNotBeNil)
		} else {
			So(cgcf.cgMgrV1, ShouldNotBeNil)
		}

		Convey("Cgroup Should Work", func(ctx C) {
			tmpDir, err := os.MkdirTemp("", "tunasync")
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
			err = os.WriteFile(cmdScript, []byte(cmdScriptContent), 0755)
			So(err, ShouldBeNil)
			err = os.WriteFile(daemonScript, []byte(daemonScriptContent), 0755)
			So(err, ShouldBeNil)

			provider, err := newCmdProvider(c)
			So(err, ShouldBeNil)

			cg := newCgroupHook(provider, *cgcf, 0)
			provider.AddHook(cg)

			err = cg.preExec()
			So(err, ShouldBeNil)

			go func() {
				err := provider.Run(make(chan empty, 1))
				ctx.So(err, ShouldNotBeNil)
			}()

			time.Sleep(1 * time.Second)
			// Deamon should be started
			daemonPidBytes, err := os.ReadFile(bgPidfile)
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

		Convey("Rsync Memory Should Be Limited", func() {
			tmpDir, err := os.MkdirTemp("", "tunasync")
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

			cg := newCgroupHook(provider, *cgcf, 512*units.MiB)
			provider.AddHook(cg)

			err = cg.preExec()
			So(err, ShouldBeNil)
			if cgcf.isUnified {
				cgpath := filepath.Join(cgcf.BasePath, cgcf.Group, provider.Name())
				if useCurrentCgroup {
					group, err := cgv2.NestedGroupPath(filepath.Join("..", provider.Name()))
					So(err, ShouldBeNil)
					cgpath = filepath.Join(cgcf.BasePath, group)
				}
				memoLimit, err := os.ReadFile(filepath.Join(cgpath, "memory.max"))
				So(err, ShouldBeNil)
				So(strings.Trim(string(memoLimit), "\n"), ShouldEqual, strconv.Itoa(512*1024*1024))
			} else {
				for _, subsys := range cg.cgMgrV1.Subsystems() {
					if subsys.Name() == cgv1.Memory {
						cgpath := filepath.Join(cgcf.Group, provider.Name())
						if useCurrentCgroup {
							p, err := cgv1.NestedPath(filepath.Join("..", provider.Name()))(cgv1.Memory)
							So(err, ShouldBeNil)
							cgpath = p
						}
						memoLimit, err := os.ReadFile(filepath.Join(cgcf.BasePath, "memory", cgpath, "memory.limit_in_bytes"))
						So(err, ShouldBeNil)
						So(strings.Trim(string(memoLimit), "\n"), ShouldEqual, strconv.Itoa(512*1024*1024))
					}
				}
			}
			cg.postExec()
			So(cg.cgMgrV1, ShouldBeNil)
		})
		Reset(func() {
			if cgcf.isUnified {
				if cgcf.Group == "" {
					wkrg, err := cgv2.NestedGroupPath("")
					So(err, ShouldBeNil)
					wkrMgr, err := cgv2.LoadManager("/sys/fs/cgroup", wkrg)
					allCtrls, err := wkrMgr.Controllers()
					So(err, ShouldBeNil)
					err = wkrMgr.ToggleControllers(allCtrls, cgv2.Disable)
					So(err, ShouldBeNil)
					origMgr := cgcf.cgMgrV2
					for {
						logger.Debugf("Restoring pids")
						procs, err := wkrMgr.Procs(false)
						So(err, ShouldBeNil)
						if len(procs) == 0 {
							break
						}
						for _, p := range procs {
							if err := origMgr.AddProc(p); err != nil {
								if errors.Is(err, syscall.ESRCH) {
									logger.Debugf("Write pid %d to sub group failed: process vanished, ignoring")
								} else {
									So(err, ShouldBeNil)
								}
							}
						}
					}
					err = wkrMgr.Delete()
					So(err, ShouldBeNil)
				}
			} else {
				if cgcf.Group == "" {
					pather := (func(p cgv1.Path) cgv1.Path {
						return func(subsys cgv1.Name) (string, error) {
							path, err := p(subsys)
							if err != nil {
								return "", err
							}
							if path == "/" {
								return "", cgv1.ErrControllerNotActive
							}
							return path, err
						}
					})(cgv1.NestedPath(""))
					wkrMgr, err := cgv1.Load(cgv1.V1, pather, func(cfg *cgv1.InitConfig) error {
						cfg.InitCheck = cgv1.AllowAny
						return nil
					})
					So(err, ShouldBeNil)
					origMgr := cgcf.cgMgrV1
					for _, subsys := range wkrMgr.Subsystems() {
						for {
							procs, err := wkrMgr.Processes(subsys.Name(), false)
							So(err, ShouldBeNil)
							if len(procs) == 0 {
								break
							}
							for _, proc := range procs {
								if err := origMgr.Add(proc); err != nil {
									if errors.Is(err, syscall.ESRCH) {
										logger.Debugf("Write pid %d to sub group failed: process vanished, ignoring")
									} else {
										So(err, ShouldBeNil)
									}
								}
							}
						}
					}
					err = wkrMgr.Delete()
					So(err, ShouldBeNil)
				}
			}
		})
	})
}
