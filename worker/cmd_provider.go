package worker

import (
	"errors"
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

func (p *cmdProvider) Type() providerEnum {
	return provCommand
}

func (p *cmdProvider) Upstream() string {
	return p.upstreamURL
}

func (p *cmdProvider) Run() error {
	if err := p.Start(); err != nil {
		return err
	}
	return p.Wait()
}

func (p *cmdProvider) Start() error {
	p.Lock()
	defer p.Unlock()

	if p.IsRunning() {
		return errors.New("provider is currently running")
	}

	env := map[string]string{
		"TUNASYNC_MIRROR_NAME":  p.Name(),
		"TUNASYNC_WORKING_DIR":  p.WorkingDir(),
		"TUNASYNC_UPSTREAM_URL": p.upstreamURL,
		"TUNASYNC_LOG_DIR":      p.LogDir(),
		"TUNASYNC_LOG_FILE":     p.LogFile(),
	}
	for k, v := range p.env {
		env[k] = v
	}
	p.cmd = newCmdJob(p, p.command, p.WorkingDir(), env)
	if err := p.prepareLogFile(false); err != nil {
		return err
	}

	if err := p.cmd.Start(); err != nil {
		return err
	}
	p.isRunning.Store(true)
	return nil
}
