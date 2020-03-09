package worker

import (
	"errors"
	"fmt"
	"io/ioutil"
	"regexp"
	"time"

	"github.com/anmitsu/go-shlex"
)

type cmdConfig struct {
	name                        string
	upstreamURL, command        string
	workingDir, logDir, logFile string
	interval                    time.Duration
	retry                       int
	env                         map[string]string
	failOnMatch                 string
}

type cmdProvider struct {
	baseProvider
	cmdConfig
	command     []string
	failOnMatch *regexp.Regexp
}

func newCmdProvider(c cmdConfig) (*cmdProvider, error) {
	// TODO: check config options
	if c.retry == 0 {
		c.retry = defaultMaxRetry
	}
	provider := &cmdProvider{
		baseProvider: baseProvider{
			name:     c.name,
			ctx:      NewContext(),
			interval: c.interval,
			retry:    c.retry,
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
	if len(c.failOnMatch) > 0 {
		var err error
		failOnMatch, err := regexp.Compile(c.failOnMatch)
		if err != nil {
			return nil, errors.New("fail-on-match regexp error: " + err.Error())
		}
		provider.failOnMatch = failOnMatch
	}

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
	if err := p.Wait(); err != nil {
		return err
	}
	if p.failOnMatch != nil {
		if logContent, err := ioutil.ReadFile(p.LogFile()); err == nil {
			matches := p.failOnMatch.FindAllSubmatch(logContent, -1)
			if len(matches) != 0 {
				logger.Debug("Fail-on-match: %r", matches)
				return errors.New(
					fmt.Sprintf("Fail-on-match regexp found %d matches", len(matches)))
			}
		} else {
			return err
		}
	}
	return nil
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
