package worker

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/codeskyblue/go-sh"
	"golang.org/x/sys/unix"
)

// runner is to run os commands giving command line, env and log file
// it's an alternative to python-sh or go-sh

var errProcessNotStarted = errors.New("Process Not Started")

type cmdJob struct {
	sync.Mutex
	cmd        *exec.Cmd
	workingDir string
	env        map[string]string
	logFile    *os.File
	finished   chan empty
	provider   mirrorProvider
	retErr     error
}

func newCmdJob(provider mirrorProvider, cmdAndArgs []string, workingDir string, env map[string]string) *cmdJob {
	var cmd *exec.Cmd

	if d := provider.Docker(); d != nil {
		c := "docker"
		args := []string{
			"run", "--rm",
			"-a", "STDOUT", "-a", "STDERR",
			"--name", d.Name(),
			"-w", workingDir,
		}
		// specify user
		args = append(
			args, "-u",
			fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		)
		// add volumes
		for _, vol := range d.Volumes() {
			logger.Debugf("volume: %s", vol)
			args = append(args, "-v", vol)
		}
		// set env
		for k, v := range env {
			kv := fmt.Sprintf("%s=%s", k, v)
			args = append(args, "-e", kv)
		}
		// apply options
		args = append(args, d.options...)
		// apply image and command
		args = append(args, d.image)
		// apply command
		args = append(args, cmdAndArgs...)

		cmd = exec.Command(c, args...)

	} else if provider.Cgroup() != nil {
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

	if provider.Docker() == nil {
		logger.Debugf("Executing command %s at %s", cmdAndArgs[0], workingDir)
		if _, err := os.Stat(workingDir); os.IsNotExist(err) {
			logger.Debugf("Making dir %s", workingDir)
			if err = os.MkdirAll(workingDir, 0755); err != nil {
				logger.Errorf("Error making dir %s: %s", workingDir, err.Error())
			}
		}
		cmd.Dir = workingDir
		cmd.Env = newEnviron(env, true)
	}

	return &cmdJob{
		cmd:        cmd,
		workingDir: workingDir,
		env:        env,
		provider:   provider,
	}
}

func (c *cmdJob) Start() error {
	// logger.Debugf("Command start: %v", c.cmd.Args)
	c.finished = make(chan empty, 1)
	return c.cmd.Start()
}

func (c *cmdJob) Wait() error {
	c.Lock()
	defer c.Unlock()

	select {
	case <-c.finished:
		return c.retErr
	default:
		err := c.cmd.Wait()
		if c.cmd.Stdout != nil {
			c.cmd.Stdout.(*os.File).Close()
		}
		c.retErr = err
		close(c.finished)
		return err
	}
}

func (c *cmdJob) SetLogFile(logFile *os.File) {
	c.cmd.Stdout = logFile
	c.cmd.Stderr = logFile
}

func (c *cmdJob) Terminate() error {
	if c.cmd == nil || c.cmd.Process == nil {
		return errProcessNotStarted
	}

	if d := c.provider.Docker(); d != nil {
		sh.Command(
			"docker", "stop", "-t", "2", d.Name(),
		).Run()
		return nil
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
