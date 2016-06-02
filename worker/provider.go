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
			password:    mirror.Password,
			excludeFile: mirror.ExcludeFile,
			workingDir:  mirrorDir,
			logDir:      logDir,
			logFile:     filepath.Join(logDir, "latest.log"),
			useIPv6:     mirror.UseIPv6,
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

	// Add Cgroup Hook
	if cfg.Cgroup.Enable {
		provider.AddHook(
			newCgroupHook(provider, cfg.Cgroup.BasePath, cfg.Cgroup.Group),
		)
	}

	// ExecOnSuccess hook
	if mirror.ExecOnSuccess != "" {
		h, err := newExecPostHook(provider, execOnSuccess, mirror.ExecOnSuccess)
		if err != nil {
			logger.Errorf("Error initializing mirror %s: %s", mirror.Name, err.Error())
			panic(err)
		}
		provider.AddHook(h)
	}
	// ExecOnFailure hook
	if mirror.ExecOnFailure != "" {
		h, err := newExecPostHook(provider, execOnFailure, mirror.ExecOnFailure)
		if err != nil {
			logger.Errorf("Error initializing mirror %s: %s", mirror.Name, err.Error())
			panic(err)
		}
		provider.AddHook(h)
	}

	return provider
}
