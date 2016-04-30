package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/gin-gonic/gin"
	"gopkg.in/op/go-logging.v1"

	tunasync "github.com/tuna/tunasync/internal"
	"github.com/tuna/tunasync/manager"
	"github.com/tuna/tunasync/worker"
)

var logger = logging.MustGetLogger("tunasync-cmd")

func startManager(c *cli.Context) {
	tunasync.InitLogger(c.Bool("verbose"), c.Bool("debug"), c.Bool("with-systemd"))

	cfg, err := manager.LoadConfig(c.String("config"), c)
	if err != nil {
		logger.Errorf("Error loading config: %s", err.Error())
		os.Exit(1)
	}
	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	m := manager.GetTUNASyncManager(cfg)
	if m == nil {
		logger.Errorf("Error intializing TUNA sync worker.")
		os.Exit(1)
	}

	logger.Info("Run tunasync manager server.")
	m.Run()
}

func startWorker(c *cli.Context) {
	tunasync.InitLogger(c.Bool("verbose"), c.Bool("debug"), c.Bool("with-systemd"))
	if !c.Bool("debug") {
		gin.SetMode(gin.ReleaseMode)
	}

	cfg, err := worker.LoadConfig(c.String("config"))
	if err != nil {
		logger.Errorf("Error loading config: %s", err.Error())
		os.Exit(1)
	}

	w := worker.GetTUNASyncWorker(cfg)
	if w == nil {
		logger.Errorf("Error intializing TUNA sync worker.")
		os.Exit(1)
	}

	logger.Info("Run tunasync worker.")
	w.Run()
}

func main() {
	app := cli.NewApp()
	app.EnableBashCompletion = true
	app.Version = "0.1"
	app.Commands = []cli.Command{
		{
			Name:    "manager",
			Aliases: []string{"m"},
			Usage:   "start the tunasync manager",
			Action:  startManager,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "config, c",
					Usage: "Load manager configurations from `FILE`",
				},

				cli.StringFlag{
					Name:  "addr",
					Usage: "The manager will listen on `ADDR`",
				},
				cli.StringFlag{
					Name:  "port",
					Usage: "The manager will bind to `PORT`",
				},
				cli.StringFlag{
					Name:  "cert",
					Usage: "Use SSL certificate from `FILE`",
				},
				cli.StringFlag{
					Name:  "key",
					Usage: "Use SSL key from `FILE`",
				},
				cli.StringFlag{
					Name:  "status-file",
					Usage: "Write status file to `FILE`",
				},
				cli.StringFlag{
					Name:  "db-file",
					Usage: "Use `FILE` as the database file",
				},
				cli.StringFlag{
					Name:  "db-type",
					Usage: "Use database type `TYPE`",
				},

				cli.BoolFlag{
					Name:  "verbose, v",
					Usage: "Enable verbose logging",
				},
				cli.BoolFlag{
					Name:  "debug",
					Usage: "Run manager in debug mode",
				},
				cli.BoolFlag{
					Name:  "with-systemd",
					Usage: "Enable systemd-compatible logging",
				},

				cli.StringFlag{
					Name:  "pidfile",
					Value: "/run/tunasync/tunasync.manager.pid",
					Usage: "The pid file of the manager process",
				},
			},
		},
		{
			Name:    "worker",
			Aliases: []string{"w"},
			Usage:   "start the tunasync worker",
			Action:  startWorker,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "config, c",
					Usage: "Load worker configurations from `FILE`",
				},

				cli.BoolFlag{
					Name:  "verbose, v",
					Usage: "Enable verbose logging",
				},
				cli.BoolFlag{
					Name:  "debug",
					Usage: "Run manager in debug mode",
				},
				cli.BoolFlag{
					Name:  "with-systemd",
					Usage: "Enable systemd-compatible logging",
				},

				cli.StringFlag{
					Name:  "pidfile",
					Value: "/run/tunasync/tunasync.worker.pid",
					Usage: "The pid file of the worker process",
				},
			},
		},
	}
	app.Run(os.Args)
}
