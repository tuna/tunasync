package worker

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tuna/tunasync/internal"
)

type twoStageRsyncConfig struct {
	name                                         string
	rsyncCmd                                     string
	stage1Profile                                string
	upstreamURL, username, password, excludeFile string
	extraOptions, archInclude, archExclude       []string
	rsyncNeverTimeout                            bool
	rsyncTimeoutValue                            int
	rsyncEnv                                     map[string]string
	workingDir, logDir, logFile                  string
	useIPv6, useIPv4                             bool
	interval                                     time.Duration
	retry                                        int
	timeout                                      time.Duration
}

// An RsyncProvider provides the implementation to rsync-based syncing jobs
type twoStageRsyncProvider struct {
	baseProvider
	twoStageRsyncConfig
	stage1Options []string
	stage2Options []string
	archOptions   []string
	dataSize      string
}

// ref: https://salsa.debian.org/mirror-team/archvsync/-/blob/master/bin/ftpsync#L431
var rsyncStage1Profiles = map[string]([]string){
	"debian": []string{"--include=*.diff/", "--exclude=*.diff/Index", "--exclude=Packages*", "--exclude=Sources*", "--exclude=Release*", "--exclude=InRelease", "--include=i18n/by-hash", "--exclude=i18n/*", "--exclude=ls-lR*"},
	"debian-oldstyle": []string{
		"--exclude=Packages*", "--exclude=Sources*", "--exclude=Release*",
		"--exclude=InRelease", "--exclude=i18n/*", "--exclude=ls-lR*", "--exclude=dep11/*",
	},
}

func genRsyncFiltersForDebianArch(flag string, archList []string) ([]string, error) {
	archFilters := make([]string, 0)
	excludeArch := make([]string, 0)
	allArch := map[string]string{
		"source":         "",
		"alpha":          "",
		"amd64":          "",
		"arm":            "",
		"armel":          "",
		"armhf":          "",
		"hppa":           "",
		"hurd-i386":      "",
		"i386":           "",
		"ia64":           "",
		"kfreebsd-amd64": "",
		"kfreebsd-i386":  "",
		"mips":           "",
		"mipsel":         "",
		"mips64el":       "",
		"powerpc":        "",
		"ppc64el":        "",
		"s390":           "",
		"s390x":          "",
		"sparc":          "",
	}
	if flag == "exclude" {
		excludeArch = archList
	}

	if flag == "include" {
		for _, arch := range archList {
			delete(allArch, arch)
		}
		for k := range allArch {
			excludeArch = append(excludeArch, k)
		}
	}

	for _, arch := range excludeArch {
		if arch == "source" {
			archFilters = append(archFilters, "--exclude=/dists/**/source", "--exclude=/pool/**/*.tar.*", "--exclude=/pool/**/*.diff.*", "--exclude=/pool/**/*.dsc")
		} else {
			archFilters = append(archFilters, fmt.Sprintf("--exclude=/dists/**/binary-%s/", arch))
			archFilters = append(archFilters, fmt.Sprintf("--exclude=/dists/**/installer-%s/", arch))
			archFilters = append(archFilters, fmt.Sprintf("--exclude=Contents-%s.gz", arch))
			archFilters = append(archFilters, fmt.Sprintf("--exclude=Contents-udeb-%s.gz", arch))
			archFilters = append(archFilters, fmt.Sprintf("--exclude=/dists/**/Contents-%s.diff/", arch))
			archFilters = append(archFilters, fmt.Sprintf("--exclude=arch-%s.files", arch))
			archFilters = append(archFilters, fmt.Sprintf("--exclude=arch-%s.list.gz", arch))
			archFilters = append(archFilters, fmt.Sprintf("--exclude=*_%s.deb", arch))
			archFilters = append(archFilters, fmt.Sprintf("--exclude=*_%s.udeb", arch))
			archFilters = append(archFilters, fmt.Sprintf("--exclude=*_%s.changes", arch))
		}
	}
	return archFilters, nil
}

func newTwoStageRsyncProvider(c twoStageRsyncConfig) (*twoStageRsyncProvider, error) {
	// TODO: check config options
	if !strings.HasSuffix(c.upstreamURL, "/") {
		return nil, errors.New("rsync upstream URL should ends with /")
	}
	if c.retry == 0 {
		c.retry = defaultMaxRetry
	}

	if c.archInclude != nil && c.archExclude != nil {
		return nil, errors.New("ARCH_EXCLUDE and ARCH_INCLUDE are mutually exclusive.  Set only one.")
	}

	provider := &twoStageRsyncProvider{
		baseProvider: baseProvider{
			name:     c.name,
			ctx:      NewContext(),
			interval: c.interval,
			retry:    c.retry,
			timeout:  c.timeout,
		},
		twoStageRsyncConfig: c,
		stage1Options: []string{
			"-aHvh", "--no-o", "--no-g", "--stats",
			"--filter", "risk .~tmp~/", "--exclude", ".~tmp~/",
			"--safe-links",
		},
		stage2Options: []string{
			"-aHvh", "--no-o", "--no-g", "--stats",
			"--filter", "risk .~tmp~/", "--exclude", ".~tmp~/",
			"--delete", "--delete-after", "--delay-updates",
			"--safe-links",
		},
		archOptions: []string{},
	}

	if c.stage1Profile == "debian" || c.stage1Profile == "debian-oldstyle" {
		if c.archInclude != nil {
			genedFilters, _ := genRsyncFiltersForDebianArch("include", c.archInclude)
			provider.archOptions = append(provider.archOptions, genedFilters...)

		}
		if c.archExclude != nil {
			genedFilters, _ := genRsyncFiltersForDebianArch("exclude", c.archExclude)
			provider.archOptions = append(provider.archOptions, genedFilters...)
		}

	}

	if c.rsyncEnv == nil {
		provider.rsyncEnv = map[string]string{}
	}
	if c.username != "" {
		provider.rsyncEnv["USER"] = c.username
	}
	if c.password != "" {
		provider.rsyncEnv["RSYNC_PASSWORD"] = c.password
	}
	if c.rsyncCmd == "" {
		provider.rsyncCmd = "rsync"
	}

	provider.ctx.Set(_WorkingDirKey, c.workingDir)
	provider.ctx.Set(_LogDirKey, c.logDir)
	provider.ctx.Set(_LogFileKey, c.logFile)

	return provider, nil
}

func (p *twoStageRsyncProvider) Type() providerEnum {
	return provTwoStageRsync
}

func (p *twoStageRsyncProvider) Upstream() string {
	return p.upstreamURL
}

func (p *twoStageRsyncProvider) DataSize() string {
	return p.dataSize
}

func (p *twoStageRsyncProvider) Options(stage int) ([]string, error) {
	var options []string
	if stage == 1 {
		options = append(options, p.stage1Options...)
		stage1Profile, ok := rsyncStage1Profiles[p.stage1Profile]
		if !ok {
			return nil, errors.New("Invalid Stage 1 Profile")
		}
		options = append(options, stage1Profile...)
		options = append(options, p.archOptions...)
		if p.twoStageRsyncConfig.extraOptions != nil {
			for _, option := range p.extraOptions {
				if option != "--delete-excluded" {
					options = append(options, option)
				}
			}
		}

	} else if stage == 2 {
		options = append(options, p.stage2Options...)
		options = append(options, p.archOptions...)
		if p.twoStageRsyncConfig.extraOptions != nil {
			options = append(options, p.extraOptions...)
		}
	} else {
		return []string{}, fmt.Errorf("Invalid stage: %d", stage)
	}

	if !p.rsyncNeverTimeout {
		timeo := 120
		if p.rsyncTimeoutValue > 0 {
			timeo = p.rsyncTimeoutValue
		}
		options = append(options, fmt.Sprintf("--timeout=%d", timeo))
	}

	if p.useIPv6 {
		options = append(options, "-6")
	} else if p.useIPv4 {
		options = append(options, "-4")
	}

	if p.excludeFile != "" {
		options = append(options, "--exclude-from", p.excludeFile)
	}

	return options, nil
}

func (p *twoStageRsyncProvider) Run(started chan empty) error {
	p.Lock()
	defer p.Unlock()

	if p.IsRunning() {
		return errors.New("provider is currently running")
	}

	p.dataSize = ""
	stages := []int{1, 2}
	for _, stage := range stages {
		command := []string{p.rsyncCmd}
		options, err := p.Options(stage)
		if err != nil {
			return err
		}
		command = append(command, options...)
		command = append(command, p.upstreamURL, p.WorkingDir())

		p.cmd = newCmdJob(p, command, p.WorkingDir(), p.rsyncEnv)
		if err := p.prepareLogFile(stage > 1); err != nil {
			return err
		}
		defer p.closeLogFile()

		if err = p.cmd.Start(); err != nil {
			return err
		}
		p.isRunning.Store(true)
		logger.Debugf("set isRunning to true: %s", p.Name())
		started <- empty{}

		p.Unlock()
		err = p.Wait()
		p.Lock()
		if err != nil {
			code, msg := internal.TranslateRsyncErrorCode(err)
			if code != 0 {
				logger.Debug("Rsync exitcode %d (%s)", code, msg)
				if p.logFileFd != nil {
					p.logFileFd.WriteString(msg + "\n")
				}
			}
			return err
		}
	}
	p.dataSize = internal.ExtractSizeFromRsyncLog(p.LogFile())
	return nil
}
