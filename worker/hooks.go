package worker

/*
hooks to exec before/after syncing
                                                                        failed
                              +------------------ post-fail hooks -------------------+
                              |                                                      |
 job start -> pre-job hooks --v-> pre-exec hooks --> (syncing) --> post-exec hooks --+---------> post-success --> end
                                                                                       success
*/

type jobHook interface {
	preJob() error
	preExec() error
	postExec() error
	postSuccess() error
	postFail() error
}

type emptyHook struct {
	provider mirrorProvider
}

func (h *emptyHook) preJob() error {
	return nil
}

func (h *emptyHook) preExec() error {
	return nil
}

func (h *emptyHook) postExec() error {
	return nil
}

func (h *emptyHook) postSuccess() error {
	return nil
}

func (h *emptyHook) postFail() error {
	return nil
}
