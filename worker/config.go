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

func (p ProviderEnum) MarshalText() ([]byte, error) {

	switch p {
	case ProvCommand:
		return []byte("command"), nil
	case ProvRsync:
		return []byte("rsync"), nil
	case ProvTwoStageRsync:
		return []byte("two-stage-rsync"), nil
	default:
		return []byte{}, errors.New("Invalid ProviderEnum value")
	}

}

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
	Mirrors []mirrorConfig `toml:"mirrors"`
}

type globalConfig struct {
	Name       string `toml:"name"`
	Token      string `toml:"token"`
	LogDir     string `toml:"log_dir"`
	MirrorDir  string `toml:"mirror_dir"`
	Concurrent int    `toml:"concurrent"`
	Interval   int    `toml:"interval"`
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

func loadConfig(cfgFile string) (*Config, error) {
	if _, err := os.Stat(cfgFile); err != nil {
		return nil, err
	}

	cfg := new(Config)
	if _, err := toml.DecodeFile(cfgFile, cfg); err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return cfg, nil
}
