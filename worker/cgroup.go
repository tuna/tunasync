package worker

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/codeskyblue/go-sh"
	"github.com/moby/moby/pkg/reexec"
	cgv1 "github.com/containerd/cgroups"
	cgv2 "github.com/containerd/cgroups/v2"
	contspecs "github.com/opencontainers/runtime-spec/specs-go"
)

type cgroupHook struct {
	emptyHook
	basePath	string
	baseGroup string
	created	 bool
	subsystem string
	memLimit	MemBytes
}

func init () {
	reexec.Register("tunasync-exec", waitExec)
}

func waitExec () {
	binary, lookErr := exec.LookPath(os.Args[1])
	if lookErr != nil {
		panic(lookErr)
	}

	pipe := os.NewFile(3, "pipe")
	if pipe != nil {
		for {
			tmpBytes := make([]byte, 1)
			nRead, err := pipe.Read(tmpBytes)
			if err != nil {
				break
			}
			if nRead == 0 {
				break
			}
		}
		err := pipe.Close()
		if err != nil {
		}
	}

	args := os.Args[1:]
	env := os.Environ()
	execErr := syscall.Exec(binary, args, env)
	if execErr != nil {
		panic(execErr)
	}
	panic("Exec failed.")
}

func initCgroup(cfg *cgroupConfig) (error) {

	logger.Debugf("Initializing cgroup")
	baseGroup := cfg.Group
	//subsystem := cfg.Subsystem

	// If baseGroup is empty, it implies using the cgroup of the current process
	// otherwise, it refers to a absolute group path
	if baseGroup != "" {
		baseGroup = filepath.Join("/", baseGroup)
	}

	cfg.isUnified = cgv1.Mode() == cgv1.Unified

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
		if cfg.cgMgrV2, err = cgv2.LoadManager("/sys/fs/cgroup", g); err != nil {
			return err
		}
		if baseGroup == "" {
			logger.Debugf("Creating a sub group and move all processes into it")
			wkrMgr, err := cfg.cgMgrV2.NewChild("__worker", nil);
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
				for _, p := range(procs) {
					if err := wkrMgr.AddProc(p); err != nil{
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
			testMgr, err := cfg.cgMgrV2.NewChild("__test", nil);
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
			pather = (func(p cgv1.Path) (cgv1.Path){
				return func(subsys cgv1.Name) (string, error){
					path, err := p(subsys);
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
		if cfg.cgMgrV1, err = cgv1.Load(cgv1.V1, pather); err != nil {
			return err
		}
		logger.Debugf("Available subsystems:")
		for _, subsys := range(cfg.cgMgrV1.Subsystems()) {
			p, err := pather(subsys.Name())
			if err != nil {
				return err
			}
			logger.Debugf("%s: %s", subsys.Name(), p)
		}
		if baseGroup == "" {
			logger.Debugf("Creating a sub group and move all processes into it")
			wkrMgr, err := cfg.cgMgrV1.New("__worker", &contspecs.LinuxResources{});
			if err != nil {
				return err
			}
			for _, subsys := range(cfg.cgMgrV1.Subsystems()) {
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
					for _, proc := range(procs) {
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
			testMgr, err := cfg.cgMgrV1.New("__test", &contspecs.LinuxResources{});
			if err != nil {
				logger.Errorf("Cannot create a sub group in the cgroup")
				return err
			}
			if err := testMgr.Delete(); err != nil {
				return err
			}
			for _, subsys := range(cfg.cgMgrV1.Subsystems()) {
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
	var (
		basePath = cfg.BasePath
		baseGroup = cfg.Group
		subsystem = cfg.Subsystem
	)
	if basePath == "" {
		basePath = "/sys/fs/cgroup"
	}
	if baseGroup == "" {
		baseGroup = "tunasync"
	}
	if subsystem == "" {
		subsystem = "cpu"
	}
	return &cgroupHook{
		emptyHook: emptyHook{
			provider: p,
		},
		basePath:  basePath,
		baseGroup: baseGroup,
		subsystem: subsystem,
	}
}

func (c *cgroupHook) preExec() error {
	c.created = true
	if err := sh.Command("cgcreate", "-g", c.Cgroup()).Run(); err != nil {
		return err
	}
	if c.subsystem != "memory" {
		return nil
	}
	if c.memLimit != 0 {
		gname := fmt.Sprintf("%s/%s", c.baseGroup, c.provider.Name())
		return sh.Command(
			"cgset", "-r",
			fmt.Sprintf("memory.limit_in_bytes=%d", c.memLimit.Value()),
			gname,
		).Run()
	}
	return nil
}

func (c *cgroupHook) postExec() error {
	err := c.killAll()
	if err != nil {
		logger.Errorf("Error killing tasks: %s", err.Error())
	}

	c.created = false
	return sh.Command("cgdelete", c.Cgroup()).Run()
}

func (c *cgroupHook) Cgroup() string {
	name := c.provider.Name()
	return fmt.Sprintf("%s:%s/%s", c.subsystem, c.baseGroup, name)
}

func (c *cgroupHook) killAll() error {
	if !c.created {
		return nil
	}
	name := c.provider.Name()

	readTaskList := func() ([]int, error) {
		taskList := []int{}
		taskFile, err := os.Open(filepath.Join(c.basePath, c.subsystem, c.baseGroup, name, "tasks"))
		if err != nil {
			return taskList, err
		}
		defer taskFile.Close()

		scanner := bufio.NewScanner(taskFile)
		for scanner.Scan() {
			pid, err := strconv.Atoi(scanner.Text())
			if err != nil {
				return taskList, err
			}
			taskList = append(taskList, pid)
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
