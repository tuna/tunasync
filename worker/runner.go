package worker

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// runner is to run os commands giving command line, env and log file
// it's an alternative to python-sh or go-sh

var errProcessNotStarted = errors.New("Process Not Started")

type cmdJob struct {
	cmd        *exec.Cmd
	workingDir string
	env        map[string]string
	logFile    *os.File
	finished   chan empty
	provider   mirrorProvider
}

func newCmdJob(provider mirrorProvider, cmdAndArgs []string, workingDir string, env map[string]string) *cmdJob {
	var cmd *exec.Cmd

	if provider.Cgroup() != nil {
		c := "cgexec"
		args := []string{"-g", provider.Cgroup().Cgroup()}
		args = append(args, cmdAndArgs...)
		cmd = exec.Command(c, args...)
	} else {
		if len(cmdAndArgs) == 1 {
			cmd = exec.Command(cmdAndArgs[0])
		} else if len(cmdAndArgs) > 1 {
			c := cmdAndArgs[0]
			args := cmdAndArgs[1:]
			cmd = exec.Command(c, args...)
		} else if len(cmdAndArgs) == 0 {
			panic("Command length should be at least 1!")
		}
	}

	logger.Debug("Executing command %s at %s", cmdAndArgs[0], workingDir)
	if _, err := os.Stat(workingDir); os.IsNotExist(err) {
		logger.Debug("Making dir %s", workingDir)
		if err = os.MkdirAll(workingDir, 0755); err != nil {
			logger.Error("Error making dir %s", workingDir)
		}
	}

	cmd.Dir = workingDir
	cmd.Env = newEnviron(env, true)

	return &cmdJob{
		cmd:        cmd,
		workingDir: workingDir,
		env:        env,
	}
}

func (c *cmdJob) Start() error {
	c.finished = make(chan empty, 1)
	return c.cmd.Start()
}

func (c *cmdJob) Wait() error {
	err := c.cmd.Wait()
	close(c.finished)
	return err
}

func (c *cmdJob) SetLogFile(logFile *os.File) {
	c.cmd.Stdout = logFile
	c.cmd.Stderr = logFile
}

func (c *cmdJob) Terminate() error {
	if c.cmd == nil || c.cmd.Process == nil {
		return errProcessNotStarted
	}
	err := unix.Kill(c.cmd.Process.Pid, syscall.SIGTERM)
	if err != nil {
		return err
	}

	select {
	case <-time.After(2 * time.Second):
		unix.Kill(c.cmd.Process.Pid, syscall.SIGKILL)
		return errors.New("SIGTERM failed to kill the job")
	case <-c.finished:
		return nil
	}
}

// Copied from go-sh
func newEnviron(env map[string]string, inherit bool) []string { //map[string]string {
	environ := make([]string, 0, len(env))
	if inherit {
		for _, line := range os.Environ() {
			// if os environment and env collapses,
			// omit the os one
			k := strings.Split(line, "=")[0]
			if _, ok := env[k]; ok {
				continue
			}
			environ = append(environ, line)
		}
	}
	for k, v := range env {
		environ = append(environ, k+"="+v)
	}
	return environ
}
