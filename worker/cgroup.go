package worker

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	cgroups "github.com/containerd/cgroups/v3"
	cgv1 "github.com/containerd/cgroups/v3/cgroup1"
	cgv2 "github.com/containerd/cgroups/v3/cgroup2"
	"github.com/moby/moby/pkg/reexec"
	contspecs "github.com/opencontainers/runtime-spec/specs-go"
)

type cgroupHook struct {
	emptyHook
	cgCfg    cgroupConfig
	memLimit MemBytes
	cgMgrV1  cgv1.Cgroup
	cgMgrV2  *cgv2.Manager
}

type execCmd string

const (
	cmdCont execCmd = "cont"
	cmdAbrt execCmd = "abrt"
)

func init() {
	reexec.Register("tunasync-exec", waitExec)
}

func waitExec() {
	binary, err := exec.LookPath(os.Args[1])
	if err != nil {
		panic(err)
	}

	pipe := os.NewFile(3, "pipe")
	if pipe != nil {
		if _, err := pipe.Stat(); err == nil {
			cmdBytes, err := io.ReadAll(pipe)
			if err != nil {
				panic(err)
			}
			if err := pipe.Close(); err != nil {
			}
			cmd := execCmd(string(cmdBytes))
			switch cmd {
			case cmdAbrt:
				fallthrough
			default:
				panic("Exited on request")
			case cmdCont:
			}
		}
	}

	args := os.Args[1:]
	env := os.Environ()
	if err := syscall.Exec(binary, args, env); err != nil {
		panic(err)
	}
	panic("Exec failed.")
}

func initCgroup(cfg *cgroupConfig) error {

	logger.Debugf("Initializing cgroup")
	baseGroup := cfg.Group
	//subsystem := cfg.Subsystem

	// If baseGroup is empty, it implies using the cgroup of the current process
	// otherwise, it refers to a absolute group path
	if baseGroup != "" {
		baseGroup = filepath.Join("/", baseGroup)
	}

	cfg.isUnified = cgroups.Mode() == cgroups.Unified

	if cfg.isUnified {
		logger.Debugf("Cgroup V2 detected")
		g := baseGroup
		if g == "" {
			logger.Debugf("Detecting my cgroup path")
			var err error
			if g, err = cgv2.NestedGroupPath(""); err != nil {
				return err
			}
		}
		logger.Infof("Using cgroup path: %s", g)

		var err error
		if cfg.cgMgrV2, err = cgv2.Load(g); err != nil {
			return err
		}
		if baseGroup == "" {
			logger.Debugf("Creating a sub group and move all processes into it")
			wkrMgr, err := cfg.cgMgrV2.NewChild("__worker", nil)
			if err != nil {
				return err
			}
			for {
				logger.Debugf("Reading pids")
				procs, err := cfg.cgMgrV2.Procs(false)
				if err != nil {
					logger.Errorf("Cannot read pids in that group")
					return err
				}
				if len(procs) == 0 {
					break
				}
				for _, p := range procs {
					if err := wkrMgr.AddProc(p); err != nil {
						if errors.Is(err, syscall.ESRCH) {
							logger.Debugf("Write pid %d to sub group failed: process vanished, ignoring")
						} else {
							return err
						}
					}
				}
			}
		} else {
			logger.Debugf("Trying to create a sub group in that group")
			testMgr, err := cfg.cgMgrV2.NewChild("__test", nil)
			if err != nil {
				logger.Errorf("Cannot create a sub group in the cgroup")
				return err
			}
			if err := testMgr.Delete(); err != nil {
				return err
			}
			procs, err := cfg.cgMgrV2.Procs(false)
			if err != nil {
				logger.Errorf("Cannot read pids in that group")
				return err
			}
			if len(procs) != 0 {
				return fmt.Errorf("There are remaining processes in cgroup %s", baseGroup)
			}
		}
	} else {
		logger.Debugf("Cgroup V1 detected")
		var pather cgv1.Path
		if baseGroup != "" {
			pather = cgv1.StaticPath(baseGroup)
		} else {
			pather = (func(p cgv1.Path) cgv1.Path {
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
		}
		logger.Infof("Loading cgroup")
		var err error
		if cfg.cgMgrV1, err = cgv1.Load(pather, func(cfg *cgv1.InitConfig) error {
			cfg.InitCheck = cgv1.AllowAny
			return nil
		}); err != nil {
			return err
		}
		logger.Debugf("Available subsystems:")
		for _, subsys := range cfg.cgMgrV1.Subsystems() {
			p, err := pather(subsys.Name())
			if err != nil {
				return err
			}
			logger.Debugf("%s: %s", subsys.Name(), p)
		}
		if baseGroup == "" {
			logger.Debugf("Creating a sub group and move all processes into it")
			wkrMgr, err := cfg.cgMgrV1.New("__worker", &contspecs.LinuxResources{})
			if err != nil {
				return err
			}
			for _, subsys := range cfg.cgMgrV1.Subsystems() {
				logger.Debugf("Reading pids for subsystem %s", subsys.Name())
				for {
					procs, err := cfg.cgMgrV1.Processes(subsys.Name(), false)
					if err != nil {
						p, err := pather(subsys.Name())
						if err != nil {
							return err
						}
						logger.Errorf("Cannot read pids in group %s of subsystem %s", p, subsys.Name())
						return err
					}
					if len(procs) == 0 {
						break
					}
					for _, proc := range procs {
						if err := wkrMgr.Add(proc); err != nil {
							if errors.Is(err, syscall.ESRCH) {
								logger.Debugf("Write pid %d to sub group failed: process vanished, ignoring")
							} else {
								return err
							}
						}
					}
				}
			}
		} else {
			logger.Debugf("Trying to create a sub group in that group")
			testMgr, err := cfg.cgMgrV1.New("__test", &contspecs.LinuxResources{})
			if err != nil {
				logger.Errorf("Cannot create a sub group in the cgroup")
				return err
			}
			if err := testMgr.Delete(); err != nil {
				return err
			}
			for _, subsys := range cfg.cgMgrV1.Subsystems() {
				logger.Debugf("Reading pids for subsystem %s", subsys.Name())
				procs, err := cfg.cgMgrV1.Processes(subsys.Name(), false)
				if err != nil {
					p, err := pather(subsys.Name())
					if err != nil {
						return err
					}
					logger.Errorf("Cannot read pids in group %s of subsystem %s", p, subsys.Name())
					return err
				}
				if len(procs) != 0 {
					p, err := pather(subsys.Name())
					if err != nil {
						return err
					}
					return fmt.Errorf("There are remaining processes in cgroup %s of subsystem %s", p, subsys.Name())
				}
			}
		}
	}

	return nil
}

func newCgroupHook(p mirrorProvider, cfg cgroupConfig, memLimit MemBytes) *cgroupHook {
	return &cgroupHook{
		emptyHook: emptyHook{
			provider: p,
		},
		cgCfg:    cfg,
		memLimit: memLimit,
	}
}

func (c *cgroupHook) preExec() error {
	if c.cgCfg.isUnified {
		logger.Debugf("Creating v2 cgroup for task %s", c.provider.Name())
		var resSet *cgv2.Resources
		if c.memLimit != 0 {
			resSet = &cgv2.Resources{
				Memory: &cgv2.Memory{
					Max: func(i int64) *int64 { return &i }(c.memLimit.Value()),
				},
			}
		}
		subMgr, err := c.cgCfg.cgMgrV2.NewChild(c.provider.Name(), resSet)
		if err != nil {
			logger.Errorf("Failed to create cgroup for task %s: %s", c.provider.Name(), err.Error())
			return err
		}
		c.cgMgrV2 = subMgr
	} else {
		logger.Debugf("Creating v1 cgroup for task %s", c.provider.Name())
		var resSet contspecs.LinuxResources
		if c.memLimit != 0 {
			resSet = contspecs.LinuxResources{
				Memory: &contspecs.LinuxMemory{
					Limit: func(i int64) *int64 { return &i }(c.memLimit.Value()),
				},
			}
		}
		subMgr, err := c.cgCfg.cgMgrV1.New(c.provider.Name(), &resSet)
		if err != nil {
			logger.Errorf("Failed to create cgroup for task %s: %s", c.provider.Name(), err.Error())
			return err
		}
		c.cgMgrV1 = subMgr
	}
	return nil
}

func (c *cgroupHook) postExec() error {
	err := c.killAll()
	if err != nil {
		logger.Errorf("Error killing tasks: %s", err.Error())
	}

	if c.cgCfg.isUnified {
		logger.Debugf("Deleting v2 cgroup for task %s", c.provider.Name())
		if err := c.cgMgrV2.Delete(); err != nil {
			logger.Errorf("Failed to delete cgroup for task %s: %s", c.provider.Name(), err.Error())
			return err
		}
		c.cgMgrV2 = nil
	} else {
		logger.Debugf("Deleting v1 cgroup for task %s", c.provider.Name())
		if err := c.cgMgrV1.Delete(); err != nil {
			logger.Errorf("Failed to delete cgroup for task %s: %s", c.provider.Name(), err.Error())
			return err
		}
		c.cgMgrV1 = nil
	}
	return nil
}

func (c *cgroupHook) killAll() error {
	if c.cgCfg.isUnified {
		if c.cgMgrV2 == nil {
			return nil
		}
	} else {
		if c.cgMgrV1 == nil {
			return nil
		}
	}

	readTaskList := func() ([]int, error) {
		taskList := []int{}
		if c.cgCfg.isUnified {
			procs, err := c.cgMgrV2.Procs(false)
			if err != nil {
				return []int{}, err
			}
			for _, proc := range procs {
				taskList = append(taskList, int(proc))
			}
		} else {
			taskSet := make(map[int]struct{})
			for _, subsys := range c.cgMgrV1.Subsystems() {
				procs, err := c.cgMgrV1.Processes(subsys.Name(), false)
				if err != nil {
					return []int{}, err
				}
				for _, proc := range procs {
					taskSet[proc.Pid] = struct{}{}
				}
			}
			for proc := range taskSet {
				taskList = append(taskList, proc)
			}
		}
		return taskList, nil
	}

	for i := 0; i < 4; i++ {
		if i == 3 {
			return errors.New("Unable to kill all child tasks")
		}
		taskList, err := readTaskList()
		if err != nil {
			return err
		}
		if len(taskList) == 0 {
			return nil
		}
		for _, pid := range taskList {
			// TODO: deal with defunct processes
			logger.Debugf("Killing process: %d", pid)
			unix.Kill(pid, syscall.SIGKILL)
		}
		// sleep 10ms for the first round, and 1.01s, 2.01s, 3.01s for the rest
		time.Sleep(time.Duration(i)*time.Second + 10*time.Millisecond)
	}

	return nil
}
