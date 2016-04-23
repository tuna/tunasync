package worker

import "time"

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

	// TODO: implement Run, Terminate and Hooks
	// run mirror job in background
	Start() error
	// Wait job to finish
	Wait() error
	// terminate mirror job
	Terminate() error
	// job hooks
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
	ctx      *Context
	name     string
	interval time.Duration
	hooks    []jobHook
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
