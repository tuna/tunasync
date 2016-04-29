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

type cgroupHook struct {
	emptyHook
	provider  mirrorProvider
	basePath  string
	baseGroup string
	created   bool
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
	return sh.Command("cgcreate", "-g", c.Cgroup()).Run()
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
	return fmt.Sprintf("cpu:%s/%s", c.baseGroup, name)
}

func (c *cgroupHook) killAll() error {
	if !c.created {
		return nil
	}
	name := c.provider.Name()
	taskFile, err := os.Open(filepath.Join(c.basePath, "cpu", c.baseGroup, name, "tasks"))
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
