package worker

import (
	"bytes"
	"errors"
	"html/template"
	"path/filepath"
	"time"
)

// mirror provider is the wrapper of mirror jobs

const (
	_WorkingDirKey = "working_dir"
	_LogDirKey     = "log_dir"
	_LogFileKey    = "log_file"
)

// A mirrorProvider instance
type mirrorProvider interface {
	// name
	Name() string
	Upstream() string

	Type() providerEnum

	// run mirror job in background
	Run() error
	// run mirror job in background
	Start() error
	// Wait job to finish
	Wait() error
	// terminate mirror job
	Terminate() error
	// job hooks
	IsRunning() bool
	// Cgroup
	Cgroup() *cgroupHook
	// ZFS
	ZFS() *zfsHook
	// Docker
	Docker() *dockerHook

	AddHook(hook jobHook)
	Hooks() []jobHook

	Interval() time.Duration

	WorkingDir() string
	LogDir() string
	LogFile() string
	IsMaster() bool

	// enter context
	EnterContext() *Context
	// exit context
	ExitContext() *Context
	// return context
	Context() *Context
}

// newProvider creates a mirrorProvider instance
// using a mirrorCfg and the global cfg
func newMirrorProvider(mirror mirrorConfig, cfg *Config) mirrorProvider {

	formatLogDir := func(logDir string, m mirrorConfig) string {
		tmpl, err := template.New("logDirTmpl-" + m.Name).Parse(logDir)
		if err != nil {
			panic(err)
		}
		var formatedLogDir bytes.Buffer
		tmpl.Execute(&formatedLogDir, m)
		return formatedLogDir.String()
	}

	logDir := mirror.LogDir
	mirrorDir := mirror.MirrorDir
	if logDir == "" {
		logDir = cfg.Global.LogDir
	}
	if mirrorDir == "" {
		mirrorDir = filepath.Join(
			cfg.Global.MirrorDir, mirror.Name,
		)
	}
	if mirror.Interval == 0 {
		mirror.Interval = cfg.Global.Interval
	}
	logDir = formatLogDir(logDir, mirror)

	// IsMaster
	isMaster := true
	if mirror.Role == "slave" {
		isMaster = false
	} else {
		if mirror.Role != "" && mirror.Role != "master" {
			logger.Warningf("Invalid role configuration for %s", mirror.Name)
		}
	}

	var provider mirrorProvider

	switch mirror.Provider {
	case provCommand:
		pc := cmdConfig{
			name:        mirror.Name,
			upstreamURL: mirror.Upstream,
			command:     mirror.Command,
			workingDir:  mirrorDir,
			logDir:      logDir,
			logFile:     filepath.Join(logDir, "latest.log"),
			interval:    time.Duration(mirror.Interval) * time.Minute,
			env:         mirror.Env,
		}
		p, err := newCmdProvider(pc)
		p.isMaster = isMaster
		if err != nil {
			panic(err)
		}
		provider = p
	case provRsync:
		rc := rsyncConfig{
			name:        mirror.Name,
			upstreamURL: mirror.Upstream,
			rsyncCmd:    mirror.Command,
			username:    mirror.Username,
			password:    mirror.Password,
			excludeFile: mirror.ExcludeFile,
			workingDir:  mirrorDir,
			logDir:      logDir,
			logFile:     filepath.Join(logDir, "latest.log"),
			useIPv6:     mirror.UseIPv6,
			useIPv4:     mirror.UseIPv4,
			interval:    time.Duration(mirror.Interval) * time.Minute,
		}
		p, err := newRsyncProvider(rc)
		p.isMaster = isMaster
		if err != nil {
			panic(err)
		}
		provider = p
	case provTwoStageRsync:
		rc := twoStageRsyncConfig{
			name:          mirror.Name,
			stage1Profile: mirror.Stage1Profile,
			upstreamURL:   mirror.Upstream,
			rsyncCmd:      mirror.Command,
			username:      mirror.Username,
			password:      mirror.Password,
			excludeFile:   mirror.ExcludeFile,
			workingDir:    mirrorDir,
			logDir:        logDir,
			logFile:       filepath.Join(logDir, "latest.log"),
			useIPv6:       mirror.UseIPv6,
			interval:      time.Duration(mirror.Interval) * time.Minute,
		}
		p, err := newTwoStageRsyncProvider(rc)
		p.isMaster = isMaster
		if err != nil {
			panic(err)
		}
		provider = p
	default:
		panic(errors.New("Invalid mirror provider"))
	}

	// Add Logging Hook
	provider.AddHook(newLogLimiter(provider))

	// Add ZFS Hook
	if cfg.ZFS.Enable {
		provider.AddHook(newZfsHook(provider, cfg.ZFS.Zpool))
	}

	// Add Docker Hook
	if cfg.Docker.Enable && len(mirror.DockerImage) > 0 {
		provider.AddHook(newDockerHook(provider, cfg.Docker, mirror))

	} else if cfg.Cgroup.Enable {
		// Add Cgroup Hook
		provider.AddHook(
			newCgroupHook(
				provider, cfg.Cgroup.BasePath, cfg.Cgroup.Group,
				cfg.Cgroup.Subsystem, mirror.MemoryLimit,
			),
		)
	}

	addHookFromCmdList := func(cmdList []string, execOn uint8) {
		if execOn != execOnSuccess && execOn != execOnFailure {
			panic("Invalid option for exec-on")
		}
		for _, cmd := range cmdList {
			h, err := newExecPostHook(provider, execOn, cmd)
			if err != nil {
				logger.Errorf("Error initializing mirror %s: %s", mirror.Name, err.Error())
				panic(err)
			}
			provider.AddHook(h)
		}
	}

	// ExecOnSuccess hook
	if len(mirror.ExecOnSuccess) > 0 {
		addHookFromCmdList(mirror.ExecOnSuccess, execOnSuccess)
	} else {
		addHookFromCmdList(cfg.Global.ExecOnSuccess, execOnSuccess)
	}
	addHookFromCmdList(mirror.ExecOnSuccessExtra, execOnSuccess)

	// ExecOnFailure hook
	if len(mirror.ExecOnFailure) > 0 {
		addHookFromCmdList(mirror.ExecOnFailure, execOnFailure)
	} else {
		addHookFromCmdList(cfg.Global.ExecOnFailure, execOnFailure)
	}
	addHookFromCmdList(mirror.ExecOnFailureExtra, execOnFailure)

	return provider
}
