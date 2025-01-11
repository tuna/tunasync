//go:build !linux
// +build !linux

package worker

type btrfsSnapshotHook struct {
}

func newBtrfsSnapshotHook(provider mirrorProvider, snapshotPath string, mirror mirrorConfig) *btrfsSnapshotHook {
	return &btrfsSnapshotHook{}
}

func (h *btrfsSnapshotHook) postExec() error {
	return nil
}

func (h *btrfsSnapshotHook) postFail() error {
	return nil
}

func (h *btrfsSnapshotHook) postSuccess() error {
	return nil
}

func (h *btrfsSnapshotHook) preExec() error {
	return nil
}

func (h *btrfsSnapshotHook) preJob() error {
	return nil
}
