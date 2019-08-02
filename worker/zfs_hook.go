package worker

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/codeskyblue/go-sh"
)

type zfsHook struct {
	emptyHook
	zpool string
}

func newZfsHook(provider mirrorProvider, zpool string) *zfsHook {
	return &zfsHook{
		emptyHook: emptyHook{
			provider: provider,
		},
		zpool: zpool,
	}
}

func (z *zfsHook) printHelpMessage() {
	zfsDataset := fmt.Sprintf("%s/%s", z.zpool, z.provider.Name())
	zfsDataset = strings.ToLower(zfsDataset)
	workingDir := z.provider.WorkingDir()
	logger.Infof("You may create the ZFS dataset with:")
	logger.Infof("    zfs create '%s'", zfsDataset)
	logger.Infof("    zfs set mountpoint='%s' '%s'", workingDir, zfsDataset)
	usr, err := user.Current()
	if err != nil || usr.Uid == "0" {
		return
	}
	logger.Infof("    chown %s '%s'", usr.Uid, workingDir)
}

// check if working directory is a zfs dataset
func (z *zfsHook) preJob() error {
	workingDir := z.provider.WorkingDir()
	if _, err := os.Stat(workingDir); os.IsNotExist(err) {
		logger.Errorf("Directory %s doesn't exist", workingDir)
		z.printHelpMessage()
		return err
	}
	if err := sh.Command("mountpoint", "-q", workingDir).Run(); err != nil {
		logger.Errorf("%s is not a mount point", workingDir)
		z.printHelpMessage()
		return err
	}
	return nil
}
