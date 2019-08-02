package worker

import (
	"errors"
	"fmt"

	"github.com/anmitsu/go-shlex"
	"github.com/codeskyblue/go-sh"
)

// hook to execute command after syncing
// typically setting timestamp, etc.

const (
	execOnSuccess uint8 = iota
	execOnFailure
)

type execPostHook struct {
	emptyHook

	// exec on success or on failure
	execOn uint8
	// command
	command []string
}

func newExecPostHook(provider mirrorProvider, execOn uint8, command string) (*execPostHook, error) {
	cmd, err := shlex.Split(command, true)
	if err != nil {
		// logger.Errorf("Failed to create exec-post-hook for command: %s", command)
		return nil, err
	}
	if execOn != execOnSuccess && execOn != execOnFailure {
		return nil, fmt.Errorf("Invalid option for exec-on: %d", execOn)
	}

	return &execPostHook{
		emptyHook: emptyHook{
			provider: provider,
		},
		execOn:  execOn,
		command: cmd,
	}, nil
}

func (h *execPostHook) postSuccess() error {
	if h.execOn == execOnSuccess {
		return h.Do()
	}
	return nil
}

func (h *execPostHook) postFail() error {
	if h.execOn == execOnFailure {
		return h.Do()
	}
	return nil
}

func (h *execPostHook) Do() error {
	p := h.provider

	exitStatus := ""
	if h.execOn == execOnSuccess {
		exitStatus = "success"
	} else {
		exitStatus = "failure"
	}

	env := map[string]string{
		"TUNASYNC_MIRROR_NAME":     p.Name(),
		"TUNASYNC_WORKING_DIR":     p.WorkingDir(),
		"TUNASYNC_UPSTREAM_URL":    p.Upstream(),
		"TUNASYNC_LOG_DIR":         p.LogDir(),
		"TUNASYNC_LOG_FILE":        p.LogFile(),
		"TUNASYNC_JOB_EXIT_STATUS": exitStatus,
	}

	session := sh.NewSession()
	for k, v := range env {
		session.SetEnv(k, v)
	}

	var cmd string
	args := []interface{}{}
	if len(h.command) == 1 {
		cmd = h.command[0]
	} else if len(h.command) > 1 {
		cmd = h.command[0]
		for _, arg := range h.command[1:] {
			args = append(args, arg)
		}
	} else {
		return errors.New("Invalid Command")
	}
	return session.Command(cmd, args...).Run()
}
