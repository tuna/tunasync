package worker

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	cgv1 "github.com/containerd/cgroups"
	cgv2 "github.com/containerd/cgroups/v2"
	units "github.com/docker/go-units"
	"github.com/imdario/mergo"
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
	Global        globalConfig        `toml:"global"`
	Manager       managerConfig       `toml:"manager"`
	Server        serverConfig        `toml:"server"`
	Cgroup        cgroupConfig        `toml:"cgroup"`
	ZFS           zfsConfig           `toml:"zfs"`
	BtrfsSnapshot btrfsSnapshotConfig `toml:"btrfs_snapshot"`
	Docker        dockerConfig        `toml:"docker"`
	Include       includeConfig       `toml:"include"`
	MirrorsConf   []mirrorConfig      `toml:"mirrors"`
	Mirrors       []mirrorConfig
}

type globalConfig struct {
	Name       string `toml:"name"`
	LogDir     string `toml:"log_dir"`
	MirrorDir  string `toml:"mirror_dir"`
	Concurrent int    `toml:"concurrent"`
	Interval   int    `toml:"interval"`
	Retry      int    `toml:"retry"`
	Timeout    int    `toml:"timeout"`

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
	isUnified bool
	cgMgrV1   cgv1.Cgroup
	cgMgrV2   *cgv2.Manager
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

type btrfsSnapshotConfig struct {
	Enable       bool   `toml:"enable"`
	SnapshotPath string `toml:"snapshot_path"`
}

type includeConfig struct {
	IncludeMirrors string `toml:"include_mirrors"`
}

type includedMirrorConfig struct {
	Mirrors []mirrorConfig `toml:"mirrors"`
}

type MemBytes int64

// Set sets the value of the MemBytes by passing a string
func (m *MemBytes) Set(value string) error {
	val, err := units.RAMInBytes(value)
	*m = MemBytes(val)
	return err
}

// Type returns the type
func (m *MemBytes) Type() string {
	return "bytes"
}

// Value returns the value in int64
func (m *MemBytes) Value() int64 {
	return int64(*m)
}

// UnmarshalJSON is the customized unmarshaler for MemBytes
func (m *MemBytes) UnmarshalText(s []byte) error {
	val, err := units.RAMInBytes(string(s))
	*m = MemBytes(val)
	return err
}

type mirrorConfig struct {
	Name         string            `toml:"name"`
	Provider     providerEnum      `toml:"provider"`
	Upstream     string            `toml:"upstream"`
	Interval     int               `toml:"interval"`
	Retry        int               `toml:"retry"`
	Timeout      int               `toml:"timeout"`
	MirrorDir    string            `toml:"mirror_dir"`
	MirrorSubDir string            `toml:"mirror_subdir"`
	LogDir       string            `toml:"log_dir"`
	Env          map[string]string `toml:"env"`
	Role         string            `toml:"role"`

	// These two options over-write the global options
	ExecOnSuccess []string `toml:"exec_on_success"`
	ExecOnFailure []string `toml:"exec_on_failure"`

	// These two options  the global options
	ExecOnSuccessExtra []string `toml:"exec_on_success_extra"`
	ExecOnFailureExtra []string `toml:"exec_on_failure_extra"`

	Command       string   `toml:"command"`
	FailOnMatch   string   `toml:"fail_on_match"`
	SizePattern   string   `toml:"size_pattern"`
	UseIPv6       bool     `toml:"use_ipv6"`
	UseIPv4       bool     `toml:"use_ipv4"`
	ExcludeFile   string   `toml:"exclude_file"`
	Username      string   `toml:"username"`
	Password      string   `toml:"password"`
	RsyncNoTimeo  bool     `toml:"rsync_no_timeout"`
	RsyncTimeout  int      `toml:"rsync_timeout"`
	RsyncOptions  []string `toml:"rsync_options"`
	RsyncOverride []string `toml:"rsync_override"`
	Stage1Profile string   `toml:"stage1_profile"`

	MemoryLimit MemBytes `toml:"memory_limit"`

	DockerImage   string   `toml:"docker_image"`
	DockerVolumes []string `toml:"docker_volumes"`
	DockerOptions []string `toml:"docker_options"`

	SnapshotPath string `toml:"snapshot_path"`

	ChildMirrors []mirrorConfig `toml:"mirrors"`
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
			cfg.MirrorsConf = append(cfg.MirrorsConf, incMirCfg.Mirrors...)
		}
	}

	for _, m := range cfg.MirrorsConf {
		if err := recursiveMirrors(cfg, nil, m); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func recursiveMirrors(cfg *Config, parent *mirrorConfig, mirror mirrorConfig) error {
	var curMir mirrorConfig
	if parent != nil {
		curMir = *parent
	}
	curMir.ChildMirrors = nil
	if err := mergo.Merge(&curMir, mirror, mergo.WithOverride); err != nil {
		return err
	}
	if mirror.ChildMirrors == nil {
		cfg.Mirrors = append(cfg.Mirrors, curMir)
	} else {
		for _, m := range mirror.ChildMirrors {
			if err := recursiveMirrors(cfg, &curMir, m); err != nil {
				return err
			}
		}
	}
	return nil
}
