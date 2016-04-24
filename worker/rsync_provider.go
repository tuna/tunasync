package worker

import "time"

type rsyncConfig struct {
	name                               string
	rsyncCmd                           string
	upstreamURL, password, excludeFile string
	workingDir, logDir, logFile        string
	useIPv6                            bool
	interval                           time.Duration
}

// An RsyncProvider provides the implementation to rsync-based syncing jobs
type rsyncProvider struct {
	baseProvider
	rsyncConfig
	options []string
}

func newRsyncProvider(c rsyncConfig) (*rsyncProvider, error) {
	// TODO: check config options
	provider := &rsyncProvider{
		baseProvider: baseProvider{
			name:     c.name,
			ctx:      NewContext(),
			interval: c.interval,
		},
		rsyncConfig: c,
	}

	if c.rsyncCmd == "" {
		provider.rsyncCmd = "rsync"
	}

	options := []string{
		"-aHvh", "--no-o", "--no-g", "--stats",
		"--exclude", ".~tmp~/",
		"--delete", "--delete-after", "--delay-updates",
		"--safe-links", "--timeout=120", "--contimeout=120",
	}

	if c.useIPv6 {
		options = append(options, "-6")
	}

	if c.excludeFile != "" {
		options = append(options, "--exclude-from", c.excludeFile)
	}

	provider.ctx.Set(_WorkingDirKey, c.workingDir)
	provider.ctx.Set(_LogDirKey, c.logDir)
	provider.ctx.Set(_LogFileKey, c.logFile)

	return provider, nil
}

func (p *rsyncProvider) Start() error {
	env := map[string]string{}
	if p.password != "" {
		env["RSYNC_PASSWORD"] = p.password
	}
	command := []string{p.rsyncCmd}
	command = append(command, p.options...)
	command = append(command, p.upstreamURL, p.WorkingDir())

	p.cmd = newCmdJob(command, p.WorkingDir(), env)
	if err := p.setLogFile(); err != nil {
		return err
	}

	return p.cmd.Start()
}
