package worker

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	tunasync "github.com/tuna/tunasync/internal"
)

// this file contains the workflow of a mirror jb

type ctrlAction uint8

const (
	jobStart      ctrlAction = iota
	jobStop                  // stop syncing keep the job
	jobDisable               // disable the job (stops goroutine)
	jobRestart               // restart syncing
	jobPing                  // ensure the goroutine is alive
	jobHalt                  // worker halts
	jobForceStart            // ignore concurrent limit
)

type jobMessage struct {
	status   tunasync.SyncStatus
	name     string
	msg      string
	schedule bool
}

const (
	// empty state
	stateNone uint32 = iota
	// ready to run, able to schedule
	stateReady
	// paused by jobStop
	statePaused
	// disabled by jobDisable
	stateDisabled
	// worker is halting
	stateHalting
)

// use to ensure all jobs are finished before
// worker exit
var jobsDone sync.WaitGroup

type mirrorJob struct {
	provider mirrorProvider
	ctrlChan chan ctrlAction
	disabled chan empty
	state    uint32
}

func newMirrorJob(provider mirrorProvider) *mirrorJob {
	return &mirrorJob{
		provider: provider,
		ctrlChan: make(chan ctrlAction, 1),
		state:    stateNone,
	}
}

func (m *mirrorJob) Name() string {
	return m.provider.Name()
}

func (m *mirrorJob) State() uint32 {
	return atomic.LoadUint32(&(m.state))
}

func (m *mirrorJob) SetState(state uint32) {
	atomic.StoreUint32(&(m.state), state)
}

func (m *mirrorJob) SetProvider(provider mirrorProvider) error {
	s := m.State()
	if (s != stateNone) && (s != stateDisabled) {
		return fmt.Errorf("Provider cannot be switched when job state is %d", s)
	}
	m.provider = provider
	return nil
}

// runMirrorJob is the goroutine where syncing job runs in
// arguments:
//    provider: mirror provider object
//    ctrlChan: receives messages from the manager
//    managerChan: push messages to the manager, this channel should have a larger buffer
//    sempaphore: make sure the concurrent running syncing job won't explode
// TODO: message struct for managerChan
func (m *mirrorJob) Run(managerChan chan<- jobMessage, semaphore chan empty) error {
	jobsDone.Add(1)
	m.disabled = make(chan empty)
	defer func() {
		close(m.disabled)
		jobsDone.Done()
	}()

	provider := m.provider

	// to make code shorter
	runHooks := func(Hooks []jobHook, action func(h jobHook) error, hookname string) error {
		for _, hook := range Hooks {
			if err := action(hook); err != nil {
				logger.Errorf(
					"failed at %s hooks for %s: %s",
					hookname, m.Name(), err.Error(),
				)
				managerChan <- jobMessage{
					tunasync.Failed, m.Name(),
					fmt.Sprintf("error exec hook %s: %s", hookname, err.Error()),
					false,
				}
				return err
			}
		}
		return nil
	}

	runJobWrapper := func(kill <-chan empty, jobDone chan<- empty) error {
		defer close(jobDone)

		managerChan <- jobMessage{tunasync.PreSyncing, m.Name(), "", false}
		logger.Noticef("start syncing: %s", m.Name())

		Hooks := provider.Hooks()
		rHooks := []jobHook{}
		for i := len(Hooks); i > 0; i-- {
			rHooks = append(rHooks, Hooks[i-1])
		}

		logger.Debug("hooks: pre-job")
		err := runHooks(Hooks, func(h jobHook) error { return h.preJob() }, "pre-job")
		if err != nil {
			return err
		}

		for retry := 0; retry < maxRetry; retry++ {
			stopASAP := false // stop job as soon as possible

			if retry > 0 {
				logger.Noticef("retry syncing: %s, retry: %d", m.Name(), retry)
			}
			err := runHooks(Hooks, func(h jobHook) error { return h.preExec() }, "pre-exec")
			if err != nil {
				return err
			}

			// start syncing
			managerChan <- jobMessage{tunasync.Syncing, m.Name(), "", false}

			var syncErr error
			syncDone := make(chan error, 1)
			go func() {
				err := provider.Run()
				syncDone <- err
			}()

			select {
			case syncErr = <-syncDone:
				logger.Debug("syncing done")
			case <-kill:
				logger.Debug("received kill")
				stopASAP = true
				err := provider.Terminate()
				if err != nil {
					logger.Errorf("failed to terminate provider %s: %s", m.Name(), err.Error())
					return err
				}
				syncErr = errors.New("killed by manager")
			}

			// post-exec hooks
			herr := runHooks(rHooks, func(h jobHook) error { return h.postExec() }, "post-exec")
			if herr != nil {
				return herr
			}

			if syncErr == nil {
				// syncing success
				logger.Noticef("succeeded syncing %s", m.Name())
				managerChan <- jobMessage{tunasync.Success, m.Name(), "", (m.State() == stateReady)}
				// post-success hooks
				err := runHooks(rHooks, func(h jobHook) error { return h.postSuccess() }, "post-success")
				if err != nil {
					return err
				}
				return nil

			}

			// syncing failed
			logger.Warningf("failed syncing %s: %s", m.Name(), syncErr.Error())
			managerChan <- jobMessage{tunasync.Failed, m.Name(), syncErr.Error(), (retry == maxRetry-1) && (m.State() == stateReady)}

			// post-fail hooks
			logger.Debug("post-fail hooks")
			err = runHooks(rHooks, func(h jobHook) error { return h.postFail() }, "post-fail")
			if err != nil {
				return err
			}
			// gracefully exit
			if stopASAP {
				logger.Debug("No retry, exit directly")
				return nil
			}
			// continue to next retry
		} // for retry
		return nil
	}

	runJob := func(kill <-chan empty, jobDone chan<- empty, bypassSemaphore <-chan empty) {
		select {
		case semaphore <- empty{}:
			defer func() { <-semaphore }()
			runJobWrapper(kill, jobDone)
		case <-bypassSemaphore:
			logger.Noticef("Concurrent limit ignored by %s", m.Name())
			runJobWrapper(kill, jobDone)
		case <-kill:
			jobDone <- empty{}
			return
		}
	}

	bypassSemaphore := make(chan empty, 1)
	for {
		if m.State() == stateReady {
			kill := make(chan empty)
			jobDone := make(chan empty)
			go runJob(kill, jobDone, bypassSemaphore)

		_wait_for_job:
			select {
			case <-jobDone:
				logger.Debug("job done")
			case ctrl := <-m.ctrlChan:
				switch ctrl {
				case jobStop:
					m.SetState(statePaused)
					close(kill)
					<-jobDone
				case jobDisable:
					m.SetState(stateDisabled)
					close(kill)
					<-jobDone
					return nil
				case jobRestart:
					m.SetState(stateReady)
					close(kill)
					<-jobDone
					time.Sleep(time.Second) // Restart may fail if the process was not exited yet
					continue
				case jobForceStart:
					select { //non-blocking
					default:
					case bypassSemaphore <- empty{}:
					}
					fallthrough
				case jobStart:
					m.SetState(stateReady)
					goto _wait_for_job
				case jobHalt:
					m.SetState(stateHalting)
					close(kill)
					<-jobDone
					return nil
				default:
					// TODO: implement this
					close(kill)
					return nil
				}
			}
		}

		ctrl := <-m.ctrlChan
		switch ctrl {
		case jobStop:
			m.SetState(statePaused)
		case jobDisable:
			m.SetState(stateDisabled)
			return nil
		case jobForceStart:
			select { //non-blocking
			default:
			case bypassSemaphore <- empty{}:
			}
			fallthrough
		case jobRestart:
			fallthrough
		case jobStart:
			m.SetState(stateReady)
		default:
			// TODO
			return nil
		}
	}
}
