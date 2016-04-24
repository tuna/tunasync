package worker

import "errors"

// this file contains the workflow of a mirror jb

type ctrlAction uint8

const (
	jobStart   ctrlAction = iota
	jobStop               // stop syncing keep the job
	jobDisable            // disable the job (stops goroutine)
	jobRestart            // restart syncing
	jobPing               // ensure the goroutine is alive
)

// runMirrorJob is the goroutine where syncing job runs in
// arguments:
//    provider: mirror provider object
//    ctrlChan: receives messages from the manager
//    managerChan: push messages to the manager
//    sempaphore: make sure the concurrent running syncing job won't explode
// TODO: message struct for managerChan
func runMirrorJob(provider mirrorProvider, ctrlChan <-chan ctrlAction, managerChan chan<- struct{}, semaphore chan empty) error {

	// to make code shorter
	runHooks := func(Hooks []jobHook, action func(h jobHook) error, hookname string) error {
		for _, hook := range Hooks {
			if err := action(hook); err != nil {
				logger.Error(
					"failed at %s hooks for %s: %s",
					hookname, provider.Name(), err.Error(),
				)
				return err
			}
		}
		return nil
	}

	runJobWrapper := func(kill <-chan empty, jobDone chan<- empty) error {
		defer close(jobDone)

		logger.Info("start syncing: %s", provider.Name())

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
				logger.Info("retry syncing: %s, retry: %d", provider.Name(), retry)
			}
			err := runHooks(Hooks, func(h jobHook) error { return h.preExec() }, "pre-exec")
			if err != nil {
				return err
			}

			// start syncing
			err = provider.Start()
			if err != nil {
				logger.Error(
					"failed to start syncing job for %s: %s",
					provider.Name(), err.Error(),
				)
				return err
			}
			var syncErr error
			syncDone := make(chan error, 1)
			go func() {
				err := provider.Wait()
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
					logger.Error("failed to terminate provider %s: %s", provider.Name(), err.Error())
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
				logger.Info("succeeded syncing %s", provider.Name())
				managerChan <- struct{}{}
				// post-success hooks
				err := runHooks(rHooks, func(h jobHook) error { return h.postSuccess() }, "post-success")
				if err != nil {
					return err
				}
				return nil

			}

			// syncing failed
			logger.Warning("failed syncing %s: %s", provider.Name(), syncErr.Error())
			managerChan <- struct{}{}

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
		case <-semaphore:
			defer func() { semaphore <- empty{} }()
			runJobWrapper(kill, jobDone)
		case <-kill:
			jobDone <- empty{}
			return
		}
	}

	enabled := true // whether this job is stopped by the manager
	for {
		if enabled {
			kill := make(chan empty)
			jobDone := make(chan empty)
			go runJob(kill, jobDone)

		_wait_for_job:
			select {
			case <-jobDone:
				logger.Debug("job done")
			case ctrl := <-ctrlChan:
				switch ctrl {
				case jobStop:
					enabled = false
					close(kill)
					<-jobDone
				case jobDisable:
					close(kill)
					<-jobDone
					return nil
				case jobRestart:
					enabled = true
					close(kill)
					<-jobDone
					continue
				case jobStart:
					enabled = true
					goto _wait_for_job
				default:
					// TODO: implement this
					close(kill)
					return nil
				}
			}
		}

		ctrl := <-ctrlChan
		switch ctrl {
		case jobStop:
			enabled = false
		case jobDisable:
			return nil
		case jobRestart:
			enabled = true
		case jobStart:
			enabled = true
		default:
			// TODO
			return nil
		}
	}
}
