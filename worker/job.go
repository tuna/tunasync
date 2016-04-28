package worker

import (
	"errors"
	"fmt"

	tunasync "github.com/tuna/tunasync/internal"
)

// this file contains the workflow of a mirror jb

type ctrlAction uint8

const (
	jobStart   ctrlAction = iota
	jobStop               // stop syncing keep the job
	jobDisable            // disable the job (stops goroutine)
	jobRestart            // restart syncing
	jobPing               // ensure the goroutine is alive
)

type jobMessage struct {
	status tunasync.SyncStatus
	name   string
	msg    string
}

type mirrorJob struct {
	provider mirrorProvider
	ctrlChan chan ctrlAction
	stopped  chan empty
	enabled  bool
}

func newMirrorJob(provider mirrorProvider) *mirrorJob {
	return &mirrorJob{
		provider: provider,
		ctrlChan: make(chan ctrlAction, 1),
		enabled:  false,
	}
}

func (m *mirrorJob) Name() string {
	return m.provider.Name()
}

func (m *mirrorJob) Stopped() bool {
	if !m.enabled {
		return true
	}
	select {
	case <-m.stopped:
		return true
	default:
		return false
	}
}

// runMirrorJob is the goroutine where syncing job runs in
// arguments:
//    provider: mirror provider object
//    ctrlChan: receives messages from the manager
//    managerChan: push messages to the manager, this channel should have a larger buffer
//    sempaphore: make sure the concurrent running syncing job won't explode
// TODO: message struct for managerChan
func (m *mirrorJob) Run(managerChan chan<- jobMessage, semaphore chan empty) error {

	m.stopped = make(chan empty)
	defer close(m.stopped)

	provider := m.provider

	// to make code shorter
	runHooks := func(Hooks []jobHook, action func(h jobHook) error, hookname string) error {
		for _, hook := range Hooks {
			if err := action(hook); err != nil {
				logger.Error(
					"failed at %s hooks for %s: %s",
					hookname, m.Name(), err.Error(),
				)
				managerChan <- jobMessage{
					tunasync.Failed, m.Name(),
					fmt.Sprintf("error exec hook %s: %s", hookname, err.Error()),
				}
				return err
			}
		}
		return nil
	}

	runJobWrapper := func(kill <-chan empty, jobDone chan<- empty) error {
		defer close(jobDone)

		managerChan <- jobMessage{tunasync.PreSyncing, m.Name(), ""}
		logger.Info("start syncing: %s", m.Name())

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
				logger.Info("retry syncing: %s, retry: %d", m.Name(), retry)
			}
			err := runHooks(Hooks, func(h jobHook) error { return h.preExec() }, "pre-exec")
			if err != nil {
				return err
			}

			// start syncing
			managerChan <- jobMessage{tunasync.Syncing, m.Name(), ""}

			var syncErr error
			syncDone := make(chan error, 1)
			go func() {
				err := provider.Run()
				if !stopASAP {
					syncDone <- err
				}
			}()

			select {
			case syncErr = <-syncDone:
				logger.Debug("syncing done")
			case <-kill:
				logger.Debug("received kill")
				stopASAP = true
				err := provider.Terminate()
				if err != nil {
					logger.Error("failed to terminate provider %s: %s", m.Name(), err.Error())
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
				logger.Info("succeeded syncing %s", m.Name())
				managerChan <- jobMessage{tunasync.Success, m.Name(), ""}
				// post-success hooks
				err := runHooks(rHooks, func(h jobHook) error { return h.postSuccess() }, "post-success")
				if err != nil {
					return err
				}
				return nil

			}

			// syncing failed
			logger.Warning("failed syncing %s: %s", m.Name(), syncErr.Error())
			managerChan <- jobMessage{tunasync.Failed, m.Name(), syncErr.Error()}

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

	runJob := func(kill <-chan empty, jobDone chan<- empty) {
		select {
		case semaphore <- empty{}:
			defer func() { <-semaphore }()
			runJobWrapper(kill, jobDone)
		case <-kill:
			jobDone <- empty{}
			return
		}
	}

	for {
		if m.enabled {
			kill := make(chan empty)
			jobDone := make(chan empty)
			go runJob(kill, jobDone)

		_wait_for_job:
			select {
			case <-jobDone:
				logger.Debug("job done")
			case ctrl := <-m.ctrlChan:
				switch ctrl {
				case jobStop:
					m.enabled = false
					close(kill)
					<-jobDone
				case jobDisable:
					close(kill)
					<-jobDone
					return nil
				case jobRestart:
					m.enabled = true
					close(kill)
					<-jobDone
					continue
				case jobStart:
					m.enabled = true
					goto _wait_for_job
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
			m.enabled = false
		case jobDisable:
			return nil
		case jobRestart:
			m.enabled = true
		case jobStart:
			m.enabled = true
		default:
			// TODO
			return nil
		}
	}
}
