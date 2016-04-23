package worker

import "time"

type rsyncConfig struct {
	name                               string
	upstreamURL, password, excludeFile string
	workingDir, logDir, logFile        string
	useIPv6                            bool
	interval                           time.Duration
}

// An RsyncProvider provides the implementation to rsync-based syncing jobs
type rsyncProvider struct {
	baseProvider
	rsyncConfig
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

	provider.ctx.Set(_WorkingDirKey, c.workingDir)
	provider.ctx.Set(_LogDirKey, c.logDir)
	provider.ctx.Set(_LogFileKey, c.logFile)

	return provider, nil
}

// TODO: implement this
func (p *rsyncProvider) Start() {

}

// TODO: implement this
func (p *rsyncProvider) Terminate() {

}
