package worker

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type providerEnum uint8

const (
	provRsync providerEnum = iota
	provTwoStageRsync
	provCommand
)

func (p *providerEnum) UnmarshalText(text []byte) error {
	s := string(text)
	switch s {
	case `command`:
		*p = provCommand
	case `rsync`:
		*p = provRsync
	case `two-stage-rsync`:
		*p = provTwoStageRsync
	default:
		return errors.New("Invalid value to provierEnum")
	}
	return nil
}

// Config represents worker config options
type Config struct {
	Global  globalConfig   `toml:"global"`
	Manager managerConfig  `toml:"manager"`
	Server  serverConfig   `toml:"server"`
	Cgroup  cgroupConfig   `toml:"cgroup"`
	ZFS     zfsConfig      `toml:"zfs"`
	Docker  dockerConfig   `toml:"docker"`
	Include includeConfig  `toml:"include"`
	Mirrors []mirrorConfig `toml:"mirrors"`
}

type globalConfig struct {
	Name       string `toml:"name"`
	LogDir     string `toml:"log_dir"`
	MirrorDir  string `toml:"mirror_dir"`
	Concurrent int    `toml:"concurrent"`
	Interval   int    `toml:"interval"`

	ExecOnSuccess []string `toml:"exec_on_success"`
	ExecOnFailure []string `toml:"exec_on_failure"`
}

type managerConfig struct {
	APIBase string `toml:"api_base"`
	// this option overrides the APIBase
	APIList []string `toml:"api_base_list"`
	CACert  string   `toml:"ca_cert"`
	// Token   string `toml:"token"`
}

func (mc managerConfig) APIBaseList() []string {
	if len(mc.APIList) > 0 {
		return mc.APIList
	}
	return []string{mc.APIBase}
}

type serverConfig struct {
	Hostname string `toml:"hostname"`
	Addr     string `toml:"listen_addr"`
	Port     int    `toml:"listen_port"`
	SSLCert  string `toml:"ssl_cert"`
	SSLKey   string `toml:"ssl_key"`
}

type cgroupConfig struct {
	Enable    bool   `toml:"enable"`
	BasePath  string `toml:"base_path"`
	Group     string `toml:"group"`
	Subsystem string `toml:"subsystem"`
}

type dockerConfig struct {
	Enable  bool     `toml:"enable"`
	Volumes []string `toml:"volumes"`
	Options []string `toml:"options"`
}

type zfsConfig struct {
	Enable bool   `toml:"enable"`
	Zpool  string `toml:"zpool"`
}

type includeConfig struct {
	IncludeMirrors string `toml:"include_mirrors"`
}

type includedMirrorConfig struct {
	Mirrors []mirrorConfig `toml:"mirrors"`
}

type mirrorConfig struct {
	Name      string            `toml:"name"`
	Provider  providerEnum      `toml:"provider"`
	Upstream  string            `toml:"upstream"`
	Interval  int               `toml:"interval"`
	MirrorDir string            `toml:"mirror_dir"`
	LogDir    string            `toml:"log_dir"`
	Env       map[string]string `toml:"env"`
	Role      string            `toml:"role"`

	// These two options over-write the global options
	ExecOnSuccess []string `toml:"exec_on_success"`
	ExecOnFailure []string `toml:"exec_on_failure"`

	// These two options  the global options
	ExecOnSuccessExtra []string `toml:"exec_on_success_extra"`
	ExecOnFailureExtra []string `toml:"exec_on_failure_extra"`

	Command       string `toml:"command"`
	UseIPv6       bool   `toml:"use_ipv6"`
	UseIPv4       bool   `toml:"use_ipv4"`
	ExcludeFile   string `toml:"exclude_file"`
	Username      string `toml:"username"`
	Password      string `toml:"password"`
	Stage1Profile string `toml:"stage1_profile"`

	MemoryLimit string `toml:"memory_limit"`

	DockerImage   string   `toml:"docker_image"`
	DockerVolumes []string `toml:"docker_volumes"`
	DockerOptions []string `toml:"docker_options"`
}

// LoadConfig loads configuration
func LoadConfig(cfgFile string) (*Config, error) {
	if _, err := os.Stat(cfgFile); err != nil {
		return nil, err
	}

	cfg := new(Config)
	if _, err := toml.DecodeFile(cfgFile, cfg); err != nil {
		logger.Errorf(err.Error())
		return nil, err
	}

	if cfg.Include.IncludeMirrors != "" {
		includedFiles, err := filepath.Glob(cfg.Include.IncludeMirrors)
		if err != nil {
			logger.Errorf(err.Error())
			return nil, err
		}
		for _, f := range includedFiles {
			var incMirCfg includedMirrorConfig
			if _, err := toml.DecodeFile(f, &incMirCfg); err != nil {
				logger.Errorf(err.Error())
				return nil, err
			}
			cfg.Mirrors = append(cfg.Mirrors, incMirCfg.Mirrors...)
		}
	}

	return cfg, nil
}
