package worker

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/codeskyblue/go-sh"
)

var cgSubsystem string = "cpu"

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
	if cgSubsystem != "memory" {
		return nil
	}
	if c.provider.Type() == provRsync || c.provider.Type() == provTwoStageRsync {
		gname := fmt.Sprintf("%s/%s", c.baseGroup, c.provider.Name())
		return sh.Command(
			"cgset", "-r", "memory.limit_in_bytes=128M", gname,
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
	return fmt.Sprintf("%s:%s/%s", cgSubsystem, c.baseGroup, name)
}

func (c *cgroupHook) killAll() error {
	if !c.created {
		return nil
	}
	name := c.provider.Name()
	taskFile, err := os.Open(filepath.Join(c.basePath, cgSubsystem, c.baseGroup, name, "tasks"))
	if err != nil {
		return err
	}
	defer taskFile.Close()
	taskList := []int{}
	scanner := bufio.NewScanner(taskFile)
	for scanner.Scan() {
		pid, err := strconv.Atoi(scanner.Text())
		if err != nil {
			return err
		}
		taskList = append(taskList, pid)
	}
	for _, pid := range taskList {
		logger.Debugf("Killing process: %d", pid)
		unix.Kill(pid, syscall.SIGKILL)
	}

	return nil
}
