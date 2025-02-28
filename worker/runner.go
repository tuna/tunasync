package worker

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/codeskyblue/go-sh"
	cgv1 "github.com/containerd/cgroups/v3/cgroup1"
	"github.com/moby/sys/reexec"
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
		// set memlimit
		if d.memoryLimit != 0 {
			args = append(args, "-m", fmt.Sprint(d.memoryLimit.Value()))
		}
		// apply options
		args = append(args, d.options...)
		// apply image and command
		args = append(args, d.image)
		// apply command
		args = append(args, cmdAndArgs...)

		cmd = exec.Command(c, args...)

	} else if provider.Cgroup() != nil {
		cmd = reexec.Command(append([]string{"tunasync-exec"}, cmdAndArgs...)...)

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
	cg := c.provider.Cgroup()
	var (
		pipeR *os.File
		pipeW *os.File
	)
	if cg != nil {
		logger.Debugf("Preparing cgroup sync pipes for job %s", c.provider.Name())
		var err error
		pipeR, pipeW, err = os.Pipe()
		if err != nil {
			return err
		}
		c.cmd.ExtraFiles = []*os.File{pipeR}
		defer pipeR.Close()
		defer pipeW.Close()
	}

	logger.Debugf("Command start: %v", c.cmd.Args)
	c.finished = make(chan empty, 1)

	if err := c.cmd.Start(); err != nil {
		return err
	}
	if cg != nil {
		if err := pipeR.Close(); err != nil {
			return err
		}
		if c.cmd == nil || c.cmd.Process == nil {
			return errProcessNotStarted
		}
		pid := c.cmd.Process.Pid
		if cg.cgCfg.isUnified {
			if err := cg.cgMgrV2.AddProc(uint64(pid)); err != nil {
				if errors.Is(err, syscall.ESRCH) {
					logger.Infof("Write pid %d to cgroup failed: process vanished, ignoring")
				} else {
					return err
				}
			}
		} else {
			if err := cg.cgMgrV1.Add(cgv1.Process{Pid: pid}); err != nil {
				if errors.Is(err, syscall.ESRCH) {
					logger.Infof("Write pid %d to cgroup failed: process vanished, ignoring")
				} else {
					return err
				}
			}
		}
		if _, err := pipeW.WriteString(string(cmdCont)); err != nil {
			return err
		}
	}
	return nil
}

func (c *cmdJob) Wait() error {
	c.Lock()
	defer c.Unlock()

	select {
	case <-c.finished:
		return c.retErr
	default:
		err := c.cmd.Wait()
		close(c.finished)
		if err != nil {
			code := err.(*exec.ExitError).ExitCode()
			allowedCodes := c.provider.GetSuccessExitCodes()
			if slices.Contains(allowedCodes, code) {
				// process exited with non-success status
				logger.Infof("Command %s exited with code %d: treated as success (allowed: %v)", c.cmd.Args, code, allowedCodes)
			} else {
				c.retErr = err
			}
		}
		return c.retErr
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
		logger.Warningf("SIGTERM failed to kill the job in 2s. SIGKILL sent")
	case <-c.finished:
	}
	return nil
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
