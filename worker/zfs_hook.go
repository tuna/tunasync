package worker

import (
	"fmt"
	"os"
	"strings"

	"github.com/codeskyblue/go-sh"
)

type zfsHook struct {
	emptyHook
	provider mirrorProvider
	zpool    string
}

func newZfsHook(provider mirrorProvider, zpool string) *zfsHook {
	return &zfsHook{
		provider: provider,
		zpool:    zpool,
	}
}

// create zfs dataset for a new mirror
func (z *zfsHook) preJob() error {
	workingDir := z.provider.WorkingDir()
	if _, err := os.Stat(workingDir); os.IsNotExist(err) {
		// sudo zfs create $zfsDataset
		// sudo zfs set mountpoint=${absPath} ${zfsDataset}

		zfsDataset := fmt.Sprintf("%s/%s", z.zpool, z.provider.Name())
		// Unknown issue of ZFS:
		// dataset name should not contain upper case letters
		zfsDataset = strings.ToLower(zfsDataset)
		logger.Infof("Creating ZFS dataset %s", zfsDataset)
		if err := sh.Command("sudo", "zfs", "create", zfsDataset).Run(); err != nil {
			return err
		}
		logger.Infof("Mount ZFS dataset %s to %s", zfsDataset, workingDir)
		if err := sh.Command("sudo", "zfs", "set", "mountpoint="+workingDir, zfsDataset).Run(); err != nil {
			return err
		}
	}
	return nil
}
