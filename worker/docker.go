package worker

import (
	"fmt"
	"os"
	"time"

	"github.com/codeskyblue/go-sh"
)

type dockerHook struct {
	emptyHook
	image       string
	volumes     []string
	options     []string
	memoryLimit MemBytes
}

func newDockerHook(p mirrorProvider, gCfg dockerConfig, mCfg mirrorConfig) *dockerHook {
	volumes := []string{}
	volumes = append(volumes, gCfg.Volumes...)
	volumes = append(volumes, mCfg.DockerVolumes...)
	if len(mCfg.ExcludeFile) > 0 {
		arg := fmt.Sprintf("%s:%s:ro", mCfg.ExcludeFile, mCfg.ExcludeFile)
		volumes = append(volumes, arg)
	}

	options := []string{}
	options = append(options, gCfg.Options...)
	options = append(options, mCfg.DockerOptions...)

	return &dockerHook{
		emptyHook: emptyHook{
			provider: p,
		},
		image:       mCfg.DockerImage,
		volumes:     volumes,
		options:     options,
		memoryLimit: mCfg.MemoryLimit,
	}
}

func (d *dockerHook) preExec() error {
	p := d.provider
	logDir := p.LogDir()
	logFile := p.LogFile()
	workingDir := p.WorkingDir()

	if _, err := os.Stat(workingDir); os.IsNotExist(err) {
		logger.Debugf("Making dir %s", workingDir)
		if err = os.MkdirAll(workingDir, 0755); err != nil {
			return fmt.Errorf("Error making dir %s: %s", workingDir, err.Error())
		}
	}

	// Override workingDir
	ctx := p.EnterContext()
	ctx.Set(
		"volumes", []string{
			fmt.Sprintf("%s:%s", logDir, logDir),
			fmt.Sprintf("%s:%s", logFile, logFile),
			fmt.Sprintf("%s:%s", workingDir, workingDir),
		},
	)
	return nil
}

func (d *dockerHook) postExec() error {
	// sh.Command(
	// 	"docker", "rm", "-f", d.Name(),
	// ).Run()
	name := d.Name()
	retry := 10
	for ; retry > 0; retry-- {
		out, err := sh.Command(
			"docker", "ps", "-a",
			"--filter", "name=^"+name+"$",
			"--format", "{{.Status}}",
		).Output()
		if err != nil {
			logger.Errorf("docker ps failed: %v", err)
			break
		}
		if len(out) == 0 {
			break
		}
		logger.Debugf("container %s still exists: '%s'", name, string(out))
		time.Sleep(1 * time.Second)
	}
	if retry == 0 {
		logger.Warningf("container %s not removed automatically, next sync may fail", name)
	}
	d.provider.ExitContext()
	return nil
}

// Volumes returns the configured volumes and
// runtime-needed volumes, including mirror dirs
// and log files
func (d *dockerHook) Volumes() []string {
	vols := make([]string, len(d.volumes))
	copy(vols, d.volumes)

	p := d.provider
	ctx := p.Context()
	if ivs, ok := ctx.Get("volumes"); ok {
		vs := ivs.([]string)
		vols = append(vols, vs...)
	}
	return vols
}

func (d *dockerHook) LogFile() string {
	p := d.provider
	ctx := p.Context()
	if iv, ok := ctx.Get(_LogFileKey + ":docker"); ok {
		v := iv.(string)
		return v
	}
	return p.LogFile()
}

func (d *dockerHook) Name() string {
	p := d.provider
	return "tunasync-job-" + p.Name()
}
