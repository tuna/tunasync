package worker

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/codeskyblue/go-sh"
)

var cgSubsystem = "cpuset"

type cgroupHook struct {
	emptyHook
	provider  mirrorProvider
	basePath  string
	baseGroup string
	created   bool
}

func initCgroup(basePath string) {
	if _, err := os.Stat(filepath.Join(basePath, "memory")); err == nil {
		cgSubsystem = "memory"
		return
	}
	logger.Warning("Memory subsystem of cgroup not enabled, fallback to cpu")
}

func newCgroupHook(p mirrorProvider, basePath, baseGroup string) *cgroupHook {
	if basePath == "" {
		basePath = "/sys/fs/cgroup"
	}
	if baseGroup == "" {
		baseGroup = "tunasync"
	}
	return &cgroupHook{
		provider:  p,
		basePath:  basePath,
		baseGroup: baseGroup,
	}
}

func (c *cgroupHook) preExec() error {
	c.created = true
	if err := sh.Command("cgcreate", "-g", c.Cgroup()).Run(); err != nil {
		return err
	}
	// if cgSubsystem != "memory" {
	// 	return nil
	// }
	// if c.provider.Type() == provRsync || c.provider.Type() == provTwoStageRsync {
	// 	gname := fmt.Sprintf("%s/%s", c.baseGroup, c.provider.Name())
	// 	return sh.Command(
	// 		"cgset", "-r", "memory.limit_in_bytes=512M", gname,
	// 	).Run()
	// }
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
	return fmt.Sprintf("%s:%s/%s", cgSubsystem, c.baseGroup, name)
}

func (c *cgroupHook) killAll() error {
	if !c.created {
		return nil
	}
	name := c.provider.Name()

	readTaskList := func() ([]int, error) {
		taskList := []int{}
		taskFile, err := os.Open(filepath.Join(c.basePath, cgSubsystem, c.baseGroup, name, "tasks"))
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
