package manager

import (
	"github.com/BurntSushi/toml"
	"github.com/urfave/cli"
)

// A Config is the top-level toml-serializaible config struct
type Config struct {
	Debug  bool         `toml:"debug"`
	Server ServerConfig `toml:"server"`
	Files  FileConfig   `toml:"files"`
}

// A ServerConfig represents the configuration for HTTP server
type ServerConfig struct {
	Addr    string `toml:"addr"`
	Port    int    `toml:"port"`
	SSLCert string `toml:"ssl_cert"`
	SSLKey  string `toml:"ssl_key"`
}

// A FileConfig contains paths to special files
type FileConfig struct {
	StatusFile string `toml:"status_file"`
	DBFile     string `toml:"db_file"`
	DBType     string `toml:"db_type"`
	// used to connect to worker
	CACert string `toml:"ca_cert"`
}

// LoadConfig loads config from specified file
func LoadConfig(cfgFile string, c *cli.Context) (*Config, error) {

	cfg := new(Config)
	cfg.Server.Addr = "127.0.0.1"
	cfg.Server.Port = 14242
	cfg.Debug = false
	cfg.Files.StatusFile = "/var/lib/tunasync/tunasync.json"
	cfg.Files.DBFile = "/var/lib/tunasync/tunasync.db"
	cfg.Files.DBType = "bolt"

	if cfgFile != "" {
		if _, err := toml.DecodeFile(cfgFile, cfg); err != nil {
			logger.Errorf(err.Error())
			return nil, err
		}
	}

	if c == nil {
		return cfg, nil
	}

	if c.String("addr") != "" {
		cfg.Server.Addr = c.String("addr")
	}
	if c.Int("port") > 0 {
		cfg.Server.Port = c.Int("port")
	}
	if c.String("cert") != "" && c.String("key") != "" {
		cfg.Server.SSLCert = c.String("cert")
		cfg.Server.SSLKey = c.String("key")
	}
	if c.String("status-file") != "" {
		cfg.Files.StatusFile = c.String("status-file")
	}
	if c.String("db-file") != "" {
		cfg.Files.DBFile = c.String("db-file")
	}
	if c.String("db-type") != "" {
		cfg.Files.DBFile = c.String("db-type")
	}

	return cfg, nil
}
