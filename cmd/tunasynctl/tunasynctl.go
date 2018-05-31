package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/op/go-logging.v1"
	"gopkg.in/urfave/cli.v1"

	tunasync "github.com/tuna/tunasync/internal"
)

var (
	buildstamp = ""
	githash    = "No githash provided"
)

const (
	listJobsPath      = "/jobs"
	listWorkersPath   = "/workers"
	flushDisabledPath = "/jobs/disabled"
	cmdPath           = "/cmd"

	systemCfgFile = "/etc/tunasync/ctl.conf"          // system-wide conf
	userCfgFile   = "$HOME/.config/tunasync/ctl.conf" // user-specific conf
)

var logger = logging.MustGetLogger("tunasynctl-cmd")

var baseURL string
var client *http.Client

func initializeWrapper(handler cli.ActionFunc) cli.ActionFunc {
	return func(c *cli.Context) error {
		err := initialize(c)
		if err != nil {
			return cli.NewExitError("", 1)
		}
		return handler(c)
	}
}

type config struct {
	ManagerAddr string `toml:"manager_addr"`
	ManagerPort int    `toml:"manager_port"`
	CACert      string `toml:"ca_cert"`
}

func loadConfig(cfgFile string, cfg *config) error {
	if cfgFile != "" {
		if _, err := toml.DecodeFile(cfgFile, cfg); err != nil {
			logger.Errorf(err.Error())
			return err
		}
	}

	return nil
}

func initialize(c *cli.Context) error {
	// init logger
	tunasync.InitLogger(c.Bool("verbose"), c.Bool("verbose"), false)

	cfg := new(config)

	// default configs
	cfg.ManagerAddr = "localhost"
	cfg.ManagerPort = 14242

	// find config file and load config
	if _, err := os.Stat(systemCfgFile); err == nil {
		loadConfig(systemCfgFile, cfg)
	}
	fmt.Println(os.ExpandEnv(userCfgFile))
	if _, err := os.Stat(os.ExpandEnv(userCfgFile)); err == nil {
		loadConfig(os.ExpandEnv(userCfgFile), cfg)
	}
	if c.String("config") != "" {
		loadConfig(c.String("config"), cfg)
	}

	// override config using the command-line arguments
	if c.String("manager") != "" {
		cfg.ManagerAddr = c.String("manager")
	}
	if c.Int("port") > 0 {
		cfg.ManagerPort = c.Int("port")
	}

	if c.String("ca-cert") != "" {
		cfg.CACert = c.String("ca-cert")
	}

	// parse base url of the manager server
	if cfg.CACert != "" {
		baseURL = fmt.Sprintf("https://%s:%d", cfg.ManagerAddr, cfg.ManagerPort)
	} else {
		baseURL = fmt.Sprintf("http://%s:%d", cfg.ManagerAddr, cfg.ManagerPort)
	}

	logger.Infof("Use manager address: %s", baseURL)

	// create HTTP client
	var err error
	client, err = tunasync.CreateHTTPClient(cfg.CACert)
	if err != nil {
		err = fmt.Errorf("Error initializing HTTP client: %s", err.Error())
		logger.Error(err.Error())
		return err

	}
	return nil
}

func listWorkers(c *cli.Context) error {
	var workers []tunasync.WorkerStatus
	_, err := tunasync.GetJSON(baseURL+listWorkersPath, &workers, client)
	if err != nil {
		return cli.NewExitError(
			fmt.Sprintf("Filed to correctly get informations from"+
				"manager server: %s", err.Error()), 1)
	}

	b, err := json.MarshalIndent(workers, "", "  ")
	if err != nil {
		return cli.NewExitError(
			fmt.Sprintf("Error printing out informations: %s",
				err.Error()),
			1)
	}
	fmt.Print(string(b))
	return nil
}

func listJobs(c *cli.Context) error {
	var jobs []tunasync.WebMirrorStatus
	if c.Bool("all") {
		_, err := tunasync.GetJSON(baseURL+listJobsPath, &jobs, client)
		if err != nil {
			return cli.NewExitError(
				fmt.Sprintf("Failed to correctly get information "+
					"of all jobs from manager server: %s", err.Error()),
				1)
		}

	} else {
		args := c.Args()
		if len(args) == 0 {
			return cli.NewExitError(
				fmt.Sprintf("Usage Error: jobs command need at"+
					" least one arguments or \"--all\" flag."), 1)
		}
		ans := make(chan []tunasync.WebMirrorStatus, len(args))
		for _, workerID := range args {
			go func(workerID string) {
				var workerJobs []tunasync.WebMirrorStatus
				_, err := tunasync.GetJSON(fmt.Sprintf("%s/workers/%s/jobs",
					baseURL, workerID), &workerJobs, client)
				if err != nil {
					logger.Errorf("Filed to correctly get jobs"+
						" for worker %s: %s", workerID, err.Error())
				}
				ans <- workerJobs
			}(workerID)
		}
		for range args {
			jobs = append(jobs, <-ans...)
		}
	}

	b, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return cli.NewExitError(
			fmt.Sprintf("Error printing out informations: %s", err.Error()),
			1)
	}
	fmt.Printf(string(b))
	return nil
}

func updateMirrorSize(c *cli.Context) error {
	args := c.Args()
	if len(args) != 2 {
		return cli.NewExitError("Usage: tunasynctl -w <worker-id> <mirror> <size>", 1)
	}
	workerID := c.String("worker")
	mirrorID := args.Get(0)
	mirrorSize := args.Get(1)

	msg := struct {
		Name string `json:"name"`
		Size string `json:"size"`
	}{
		Name: mirrorID,
		Size: mirrorSize,
	}

	url := fmt.Sprintf(
		"%s/workers/%s/jobs/%s/size", baseURL, workerID, mirrorID,
	)

	resp, err := tunasync.PostJSON(url, msg, client)
	if err != nil {
		return cli.NewExitError(
			fmt.Sprintf("Failed to send request to manager: %s",
				err.Error()),
			1)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return cli.NewExitError(
			fmt.Sprintf("Manager failed to update mirror size: %s", body), 1,
		)
	}

	var status tunasync.MirrorStatus
	json.Unmarshal(body, &status)
	if status.Size != mirrorSize {
		return cli.NewExitError(
			fmt.Sprintf(
				"Mirror size error, expecting %s, manager returned %s",
				mirrorSize, status.Size,
			), 1,
		)
	}

	logger.Infof("Successfully updated mirror size to %s", mirrorSize)
	return nil
}

func removeWorker(c *cli.Context) error {
	args := c.Args()
	if len(args) != 0 {
		return cli.NewExitError("Usage: tunasynctl -w <worker-id>", 1)
	}
	workerID := c.String("worker")
	if len(workerID) == 0 {
		return cli.NewExitError("Please specify the <worker-id>", 1)
	}
	url := fmt.Sprintf("%s/workers/%s", baseURL, workerID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		logger.Panicf("Invalid HTTP Request: %s", err.Error())
	}
	resp, err := client.Do(req)

	if err != nil {
		return cli.NewExitError(
			fmt.Sprintf("Failed to send request to manager: %s", err.Error()), 1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return cli.NewExitError(
				fmt.Sprintf("Failed to parse response: %s", err.Error()),
				1)
		}

		return cli.NewExitError(fmt.Sprintf("Failed to correctly send"+
			" command: HTTP status code is not 200: %s", body),
			1)
	}

	res := map[string]string{}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if res["message"] == "deleted" {
		logger.Info("Successfully removed the worker")
	} else {
		logger.Info("Failed to remove the worker")
	}
	return nil
}

func flushDisabledJobs(c *cli.Context) error {
	req, err := http.NewRequest("DELETE", baseURL+flushDisabledPath, nil)
	if err != nil {
		logger.Panicf("Invalid  HTTP Request: %s", err.Error())
	}
	resp, err := client.Do(req)

	if err != nil {
		return cli.NewExitError(
			fmt.Sprintf("Failed to send request to manager: %s",
				err.Error()),
			1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return cli.NewExitError(
				fmt.Sprintf("Failed to parse response: %s", err.Error()),
				1)
		}

		return cli.NewExitError(fmt.Sprintf("Failed to correctly send"+
			" command: HTTP status code is not 200: %s", body),
			1)
	}

	logger.Info("Successfully flushed disabled jobs")
	return nil
}

func cmdJob(cmd tunasync.CmdVerb) cli.ActionFunc {
	return func(c *cli.Context) error {
		var mirrorID string
		var argsList []string
		if len(c.Args()) == 1 {
			mirrorID = c.Args()[0]
		} else if len(c.Args()) == 2 {
			mirrorID = c.Args()[0]
			for _, arg := range strings.Split(c.Args()[1], ",") {
				argsList = append(argsList, strings.TrimSpace(arg))
			}
		} else {
			return cli.NewExitError("Usage Error: cmd command receive just "+
				"1 required positional argument MIRROR and 1 optional "+
				"argument WORKER", 1)
		}

		options := map[string]bool{}
		if c.Bool("force") {
			options["force"] = true
		}
		cmd := tunasync.ClientCmd{
			Cmd:      cmd,
			MirrorID: mirrorID,
			WorkerID: c.String("worker"),
			Args:     argsList,
			Options:  options,
		}
		resp, err := tunasync.PostJSON(baseURL+cmdPath, cmd, client)
		if err != nil {
			return cli.NewExitError(
				fmt.Sprintf("Failed to correctly send command: %s",
					err.Error()),
				1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return cli.NewExitError(
					fmt.Sprintf("Failed to parse response: %s", err.Error()),
					1)
			}

			return cli.NewExitError(fmt.Sprintf("Failed to correctly send"+
				" command: HTTP status code is not 200: %s", body),
				1)
		}
		logger.Info("Succesfully send command")

		return nil
	}
}

func cmdWorker(cmd tunasync.CmdVerb) cli.ActionFunc {
	return func(c *cli.Context) error {
		cmd := tunasync.ClientCmd{
			Cmd:      cmd,
			WorkerID: c.String("worker"),
		}
		resp, err := tunasync.PostJSON(baseURL+cmdPath, cmd, client)
		if err != nil {
			return cli.NewExitError(
				fmt.Sprintf("Failed to correctly send command: %s",
					err.Error()),
				1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return cli.NewExitError(
					fmt.Sprintf("Failed to parse response: %s", err.Error()),
					1)
			}

			return cli.NewExitError(fmt.Sprintf("Failed to correctly send"+
				" command: HTTP status code is not 200: %s", body),
				1)
		}
		logger.Info("Succesfully send command")

		return nil
	}
}

func main() {
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
	app.EnableBashCompletion = true
	app.Version = tunasync.Version
	app.Name = "tunasynctl"
	app.Usage = "control client for tunasync manager"

	commonFlags := []cli.Flag{
		cli.StringFlag{
			Name: "config, c",
			Usage: "Read configuration from `FILE` rather than" +
				" ~/.config/tunasync/ctl.conf and /etc/tunasync/ctl.conf",
		},
		cli.StringFlag{
			Name:  "manager, m",
			Usage: "The manager server address",
		},
		cli.StringFlag{
			Name:  "port, p",
			Usage: "The manager server port",
		},
		cli.StringFlag{
			Name:  "ca-cert",
			Usage: "Trust root CA cert file `CERT`",
		},

		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "Enable verbosely logging",
		},
	}
	cmdFlags := []cli.Flag{
		cli.StringFlag{
			Name:  "worker, w",
			Usage: "Send the command to `WORKER`",
		},
	}

	forceStartFlag := cli.BoolFlag{
		Name:  "force, f",
		Usage: "Override the concurrent limit",
	}

	app.Commands = []cli.Command{
		{
			Name:  "list",
			Usage: "List jobs of workers",
			Flags: append(commonFlags,
				[]cli.Flag{
					cli.BoolFlag{
						Name:  "all, a",
						Usage: "List all jobs of all workers",
					},
				}...),
			Action: initializeWrapper(listJobs),
		},
		{
			Name:   "flush",
			Usage:  "Flush disabled jobs",
			Flags:  commonFlags,
			Action: initializeWrapper(flushDisabledJobs),
		},
		{
			Name:   "workers",
			Usage:  "List workers",
			Flags:  commonFlags,
			Action: initializeWrapper(listWorkers),
		},
		{
			Name:  "rm-worker",
			Usage: "Remove a worker",
			Flags: append(
				commonFlags,
				cli.StringFlag{
					Name:  "worker, w",
					Usage: "worker-id of the worker to be removed",
				},
			),
			Action: initializeWrapper(removeWorker),
		},
		{
			Name:  "set-size",
			Usage: "Set mirror size",
			Flags: append(
				commonFlags,
				cli.StringFlag{
					Name:  "worker, w",
					Usage: "specify worker-id of the mirror job",
				},
			),
			Action: initializeWrapper(updateMirrorSize),
		},
		{
			Name:   "start",
			Usage:  "Start a job",
			Flags:  append(append(commonFlags, cmdFlags...), forceStartFlag),
			Action: initializeWrapper(cmdJob(tunasync.CmdStart)),
		},
		{
			Name:   "stop",
			Usage:  "Stop a job",
			Flags:  append(commonFlags, cmdFlags...),
			Action: initializeWrapper(cmdJob(tunasync.CmdStop)),
		},
		{
			Name:   "disable",
			Usage:  "Disable a job",
			Flags:  append(commonFlags, cmdFlags...),
			Action: initializeWrapper(cmdJob(tunasync.CmdDisable)),
		},
		{
			Name:   "restart",
			Usage:  "Restart a job",
			Flags:  append(commonFlags, cmdFlags...),
			Action: initializeWrapper(cmdJob(tunasync.CmdRestart)),
		},
		{
			Name:   "reload",
			Usage:  "Tell worker to reload configurations",
			Flags:  append(commonFlags, cmdFlags...),
			Action: initializeWrapper(cmdWorker(tunasync.CmdReload)),
		},
		{
			Name:   "ping",
			Flags:  append(commonFlags, cmdFlags...),
			Action: initializeWrapper(cmdJob(tunasync.CmdPing)),
		},
	}
	app.Run(os.Args)
}
