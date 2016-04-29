package worker

import (
	"errors"
	"os"

	"github.com/BurntSushi/toml"
)

type ProviderEnum uint8

const (
	ProvRsync ProviderEnum = iota
	ProvTwoStageRsync
	ProvCommand
)

func (p *ProviderEnum) UnmarshalText(text []byte) error {
	s := string(text)
	switch s {
	case `command`:
		*p = ProvCommand
	case `rsync`:
		*p = ProvRsync
	case `two-stage-rsync`:
		*p = ProvTwoStageRsync
	default:
		return errors.New("Invalid value to provierEnum")
	}
	return nil

}

type Config struct {
	Global  globalConfig   `toml:"global"`
	Manager managerConfig  `toml:"manager"`
	Server  serverConfig   `toml:"server"`
	Cgroup  cgroupConfig   `toml:"cgroup"`
	Mirrors []mirrorConfig `toml:"mirrors"`
}

type globalConfig struct {
	Name       string `toml:"name"`
	LogDir     string `toml:"log_dir"`
	MirrorDir  string `toml:"mirror_dir"`
	Concurrent int    `toml:"concurrent"`
	Interval   int    `toml:"interval"`
}

type managerConfig struct {
	APIBase string `toml:"api_base"`
	CACert  string `toml:"ca_cert"`
	Token   string `toml:"token"`
}

type serverConfig struct {
	Hostname string `toml:"hostname"`
	Addr     string `toml:"listen_addr"`
	Port     int    `toml:"listen_port"`
	SSLCert  string `toml:"ssl_cert"`
	SSLKey   string `toml:"ssl_key"`
}

type cgroupConfig struct {
	Enable   bool   `toml:"enable"`
	BasePath string `toml:"base_path"`
	Group    string `toml:"group"`
}

type mirrorConfig struct {
	Name      string            `toml:"name"`
	Provider  ProviderEnum      `toml:"provider"`
	Upstream  string            `toml:"upstream"`
	Interval  int               `toml:"interval"`
	MirrorDir string            `toml:"mirror_dir"`
	LogDir    string            `toml:"log_dir"`
	Env       map[string]string `toml:"env"`

	Command       string `toml:"command"`
	UseIPv6       bool   `toml:"use_ipv6"`
	ExcludeFile   string `toml:"exclude_file"`
	Password      string `toml:"password"`
	Stage1Profile string `toml:"stage1_profile"`
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
	return cfg, nil
}
