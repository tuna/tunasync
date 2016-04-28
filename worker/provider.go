package worker

import (
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// mirror provider is the wrapper of mirror jobs

type providerType uint8

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

	AddHook(hook jobHook)
	Hooks() []jobHook

	Interval() time.Duration

	WorkingDir() string
	LogDir() string
	LogFile() string

	// enter context
	EnterContext() *Context
	// exit context
	ExitContext() *Context
	// return context
	Context() *Context
}

type baseProvider struct {
	sync.Mutex

	ctx      *Context
	name     string
	interval time.Duration

	cmd       *cmdJob
	isRunning atomic.Value

	logFile *os.File

	hooks []jobHook
}

func (p *baseProvider) Name() string {
	return p.name
}

func (p *baseProvider) EnterContext() *Context {
	p.ctx = p.ctx.Enter()
	return p.ctx
}

func (p *baseProvider) ExitContext() *Context {
	p.ctx, _ = p.ctx.Exit()
	return p.ctx
}

func (p *baseProvider) Context() *Context {
	return p.ctx
}

func (p *baseProvider) Interval() time.Duration {
	// logger.Debug("interval for %s: %v", p.Name(), p.interval)
	return p.interval
}

func (p *baseProvider) WorkingDir() string {
	if v, ok := p.ctx.Get(_WorkingDirKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	panic("working dir is impossible to be non-exist")
}

func (p *baseProvider) LogDir() string {
	if v, ok := p.ctx.Get(_LogDirKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	panic("log dir is impossible to be unavailable")
}

func (p *baseProvider) LogFile() string {
	if v, ok := p.ctx.Get(_LogFileKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	panic("log dir is impossible to be unavailable")
}

func (p *baseProvider) AddHook(hook jobHook) {
	p.hooks = append(p.hooks, hook)
}

func (p *baseProvider) Hooks() []jobHook {
	return p.hooks
}

func (p *baseProvider) prepareLogFile() error {
	if p.LogFile() == "/dev/null" {
		p.cmd.SetLogFile(nil)
		return nil
	}
	if p.logFile == nil {
		logFile, err := os.OpenFile(p.LogFile(), os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			logger.Error("Error opening logfile %s: %s", p.LogFile(), err.Error())
			return err
		}
		p.logFile = logFile
	}
	p.cmd.SetLogFile(p.logFile)
	return nil
}

func (p *baseProvider) Run() error {
	panic("Not Implemented")
}

func (p *baseProvider) Start() error {
	panic("Not Implemented")
}

func (p *baseProvider) IsRunning() bool {
	isRunning, _ := p.isRunning.Load().(bool)
	return isRunning
}

func (p *baseProvider) Wait() error {
	defer func() {
		p.Lock()
		p.isRunning.Store(false)
		if p.logFile != nil {
			p.logFile.Close()
			p.logFile = nil
		}
		p.Unlock()
	}()
	return p.cmd.Wait()
}

func (p *baseProvider) Terminate() error {
	logger.Debug("terminating provider: %s", p.Name())
	if !p.IsRunning() {
		return nil
	}

	p.Lock()
	if p.logFile != nil {
		p.logFile.Close()
		p.logFile = nil
	}
	p.Unlock()

	err := p.cmd.Terminate()
	p.isRunning.Store(false)

	return err
}
