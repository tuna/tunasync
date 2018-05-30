package worker

import (
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// baseProvider is the base mixin of providers

type baseProvider struct {
	sync.Mutex

	ctx      *Context
	name     string
	interval time.Duration
	isMaster bool

	cmd       *cmdJob
	isRunning atomic.Value

	cgroup *cgroupHook
	zfs    *zfsHook
	docker *dockerHook

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

func (p *baseProvider) IsMaster() bool {
	return p.isMaster
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
	panic("log file is impossible to be unavailable")
}

func (p *baseProvider) AddHook(hook jobHook) {
	switch v := hook.(type) {
	case *cgroupHook:
		p.cgroup = v
	case *zfsHook:
		p.zfs = v
	case *dockerHook:
		p.docker = v
	}
	p.hooks = append(p.hooks, hook)
}

func (p *baseProvider) Hooks() []jobHook {
	return p.hooks
}

func (p *baseProvider) Cgroup() *cgroupHook {
	return p.cgroup
}

func (p *baseProvider) ZFS() *zfsHook {
	return p.zfs
}

func (p *baseProvider) Docker() *dockerHook {
	return p.docker
}

func (p *baseProvider) prepareLogFile(append bool) error {
	if p.LogFile() == "/dev/null" {
		p.cmd.SetLogFile(nil)
		return nil
	}
	appendMode := 0
	if append {
		appendMode = os.O_APPEND
	}
	logFile, err := os.OpenFile(p.LogFile(), os.O_WRONLY|os.O_CREATE|appendMode, 0644)
	if err != nil {
		logger.Errorf("Error opening logfile %s: %s", p.LogFile(), err.Error())
		return err
	}
	p.cmd.SetLogFile(logFile)
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
		logger.Debugf("set isRunning to false: %s", p.Name())
		p.isRunning.Store(false)
	}()
	logger.Debugf("calling Wait: %s", p.Name())
	return p.cmd.Wait()
}

func (p *baseProvider) Terminate() error {
	p.Lock()
	defer p.Unlock()
	logger.Debugf("terminating provider: %s", p.Name())
	if !p.IsRunning() {
		return nil
	}

	err := p.cmd.Terminate()

	return err
}
