package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moby/moby/pkg/reexec"
	"github.com/pkg/profile"
	"github.com/urfave/cli"
	"gopkg.in/op/go-logging.v1"

	tunasync "github.com/tuna/tunasync/internal"
	"github.com/tuna/tunasync/manager"
	"github.com/tuna/tunasync/worker"
)

var (
	buildstamp = ""
	githash    = "No githash provided"
)

var logger = logging.MustGetLogger("tunasync")

func startManager(c *cli.Context) error {
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
		logger.Errorf("Error intializing TUNA sync manager.")
		os.Exit(1)
	}

	logger.Info("Run tunasync manager server.")
	m.Run()
	return nil
}

func startWorker(c *cli.Context) error {
	tunasync.InitLogger(c.Bool("verbose"), c.Bool("debug"), c.Bool("with-systemd"))
	if !c.Bool("debug") {
		gin.SetMode(gin.ReleaseMode)
	}

	cfg, err := worker.LoadConfig(c.String("config"))
	if err != nil {
		logger.Errorf("Error loading config: %s", err.Error())
		os.Exit(1)
	}

	w := worker.NewTUNASyncWorker(cfg)
	if w == nil {
		logger.Errorf("Error intializing TUNA sync worker.")
		os.Exit(1)
	}

	if profPath := c.String("prof-path"); profPath != "" {
		valid := false
		if fi, err := os.Stat(profPath); err == nil {
			if fi.IsDir() {
				valid = true
				defer profile.Start(profile.ProfilePath(profPath)).Stop()
			}
		}
		if !valid {
			logger.Errorf("Invalid profiling path: %s", profPath)
			os.Exit(1)
		}
	}

	go func() {
		time.Sleep(1 * time.Second)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGHUP)
		signal.Notify(sigChan, syscall.SIGINT)
		signal.Notify(sigChan, syscall.SIGTERM)
		for s := range sigChan {
			switch s {
			case syscall.SIGHUP:
				logger.Info("Received reload signal")
				newCfg, err := worker.LoadConfig(c.String("config"))
				if err != nil {
					logger.Errorf("Error loading config: %s", err.Error())
				} else {
					w.ReloadMirrorConfig(newCfg.Mirrors)
				}
			case syscall.SIGINT, syscall.SIGTERM:
				w.Halt()
			}
		}
	}()

	logger.Info("Run tunasync worker.")
	w.Run()
	return nil
}

func main() {

	if reexec.Init() {
		return
	}

	cli.VersionPrinter = func(c *cli.Context) {
		var builddate string
		if buildstamp == "" {
			builddate = "No build date provided"
		} else {
			ts, err := strconv.Atoi(buildstamp)
			if err != nil {
				builddate = "No build date provided"
			} else {
				t := time.Unix(int64(ts), 0)
				builddate = t.String()
			}
		}
		fmt.Printf(
			"Version: %s\n"+
				"Git Hash: %s\n"+
				"Build Date: %s\n",
			c.App.Version, githash, builddate,
		)
	}

	app := cli.NewApp()
	app.Name = "tunasync"
	app.Usage = "tunasync mirror job management tool"
	app.EnableBashCompletion = true
	app.Version = tunasync.Version
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
					Usage: "Run worker in debug mode",
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
				cli.StringFlag{
					Name:  "prof-path",
					Value: "",
					Usage: "Go profiling file path",
				},
			},
		},
	}
	app.Run(os.Args)
}
