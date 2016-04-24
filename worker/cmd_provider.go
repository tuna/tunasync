package worker

import (
	"time"

	"github.com/anmitsu/go-shlex"
)

type cmdConfig struct {
	name                        string
	upstreamURL, command        string
	workingDir, logDir, logFile string
	interval                    time.Duration
	env                         map[string]string
}

type cmdProvider struct {
	baseProvider
	cmdConfig
	command []string
}

func newCmdProvider(c cmdConfig) (*cmdProvider, error) {
	// TODO: check config options
	provider := &cmdProvider{
		baseProvider: baseProvider{
			name:     c.name,
			ctx:      NewContext(),
			interval: c.interval,
		},
		cmdConfig: c,
	}

	provider.ctx.Set(_WorkingDirKey, c.workingDir)
	provider.ctx.Set(_LogDirKey, c.logDir)
	provider.ctx.Set(_LogFileKey, c.logFile)

	cmd, err := shlex.Split(c.command, true)
	if err != nil {
		return nil, err
	}
	provider.command = cmd

	return provider, nil
}

func (p *cmdProvider) Start() error {
	env := map[string]string{
		"TUNASYNC_MIRROR_NAME":  p.Name(),
		"TUNASYNC_WORKING_DIR":  p.WorkingDir(),
		"TUNASYNC_UPSTREAM_URL": p.upstreamURL,
		"TUNASYNC_LOG_FILE":     p.LogFile(),
	}
	for k, v := range p.env {
		env[k] = v
	}
	p.cmd = newCmdJob(p.command, p.WorkingDir(), env)
	if err := p.setLogFile(); err != nil {
		return err
	}

	return p.cmd.Start()
}
