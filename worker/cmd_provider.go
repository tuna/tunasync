package worker

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/anmitsu/go-shlex"
	"github.com/tuna/tunasync/internal"
)

type cmdConfig struct {
	name                        string
	upstreamURL, command        string
	workingDir, logDir, logFile string
	interval                    time.Duration
	retry                       int
	timeout                     time.Duration
	env                         map[string]string
	failOnMatch                 string
	sizePattern                 string
}

type cmdProvider struct {
	baseProvider
	cmdConfig
	command     []string
	dataSize    string
	failOnMatch *regexp.Regexp
	sizePattern *regexp.Regexp
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
			timeout:  c.timeout,
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
	if len(c.sizePattern) > 0 {
		var err error
		sizePattern, err := regexp.Compile(c.sizePattern)
		if err != nil {
			return nil, errors.New("size-pattern regexp error: " + err.Error())
		}
		provider.sizePattern = sizePattern
	}

	return provider, nil
}

func (p *cmdProvider) Type() providerEnum {
	return provCommand
}

func (p *cmdProvider) Upstream() string {
	return p.upstreamURL
}

func (p *cmdProvider) DataSize() string {
	return p.dataSize
}

func (p *cmdProvider) Run(started chan empty) error {
	p.dataSize = ""
	defer p.closeLogFile()
	if err := p.Start(); err != nil {
		return err
	}
	started <- empty{}
	if err := p.Wait(); err != nil {
		return err
	}
	if p.failOnMatch != nil {
		matches, err := internal.FindAllSubmatchInFile(p.LogFile(), p.failOnMatch)
		logger.Infof("FindAllSubmatchInFile: %q\n", matches)
		if err != nil {
			return err
		}
		if len(matches) != 0 {
			logger.Debug("Fail-on-match: %r", matches)
			return fmt.Errorf("fail-on-match regexp found %d matches", len(matches))
		}
	}
	if p.sizePattern != nil {
		p.dataSize = internal.ExtractSizeFromLog(p.LogFile(), p.sizePattern)
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
	logger.Debugf("set isRunning to true: %s", p.Name())
	return nil
}
