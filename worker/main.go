package worker

import (
	"bytes"
	"errors"
	"html/template"
	"path/filepath"
	"time"
)

// toplevel module for workers

func initProviders(c *Config) []mirrorProvider {

	formatLogDir := func(logDir string, m mirrorConfig) string {
		tmpl, err := template.New("logDirTmpl-" + m.Name).Parse(logDir)
		if err != nil {
			panic(err)
		}
		var formatedLogDir bytes.Buffer
		tmpl.Execute(&formatedLogDir, m)
		return formatedLogDir.String()
	}

	providers := []mirrorProvider{}

	for _, mirror := range c.Mirrors {
		logDir := mirror.LogDir
		mirrorDir := mirror.MirrorDir
		if logDir == "" {
			logDir = c.Global.LogDir
		}
		if mirrorDir == "" {
			mirrorDir = c.Global.MirrorDir
		}
		logDir = formatLogDir(logDir, mirror)
		switch mirror.Provider {
		case ProvCommand:
			pc := cmdConfig{
				name:        mirror.Name,
				upstreamURL: mirror.Upstream,
				command:     mirror.Command,
				workingDir:  filepath.Join(mirrorDir, mirror.Name),
				logDir:      logDir,
				logFile:     filepath.Join(logDir, "latest.log"),
				interval:    time.Duration(mirror.Interval) * time.Minute,
				env:         mirror.Env,
			}
			p, err := newCmdProvider(pc)
			if err != nil {
				panic(err)
			}
			providers = append(providers, p)
		case ProvRsync:
			rc := rsyncConfig{
				name:        mirror.Name,
				upstreamURL: mirror.Upstream,
				password:    mirror.Password,
				excludeFile: mirror.ExcludeFile,
				workingDir:  filepath.Join(mirrorDir, mirror.Name),
				logDir:      logDir,
				logFile:     filepath.Join(logDir, "latest.log"),
				useIPv6:     mirror.UseIPv6,
				interval:    time.Duration(mirror.Interval) * time.Minute,
			}
			p, err := newRsyncProvider(rc)
			if err != nil {
				panic(err)
			}
			providers = append(providers, p)
		case ProvTwoStageRsync:
			rc := twoStageRsyncConfig{
				name:          mirror.Name,
				stage1Profile: mirror.Stage1Profile,
				upstreamURL:   mirror.Upstream,
				password:      mirror.Password,
				excludeFile:   mirror.ExcludeFile,
				workingDir:    filepath.Join(mirrorDir, mirror.Name),
				logDir:        logDir,
				logFile:       filepath.Join(logDir, "latest.log"),
				useIPv6:       mirror.UseIPv6,
				interval:      time.Duration(mirror.Interval) * time.Minute,
			}
			p, err := newTwoStageRsyncProvider(rc)
			if err != nil {
				panic(err)
			}
			providers = append(providers, p)
		default:
			panic(errors.New("Invalid mirror provider"))

		}

	}
	return providers
}

func main() {

	for {
		// if time.Now().After() {
		//
		// }

		time.Sleep(1 * time.Second)
	}

}
