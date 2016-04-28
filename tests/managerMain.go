package main

import "github.com/tuna/tunasync/manager"

var cfg = manager.Config{
	Debug: true,
	Server: manager.ServerConfig{
		Addr:    "127.0.0.1",
		Port:    12345,
		SSLCert: "manager.crt",
		SSLKey:  "manager.key",
	},
	Files: manager.FileConfig{
		DBType: "bolt",
		DBFile: "/tmp/tunasync/manager.db",
		CACert: "rootCA.crt",
	},
}

func main() {
	m := manager.GetTUNASyncManager(&cfg)
	m.Run()
}
