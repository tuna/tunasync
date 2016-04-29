package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/codegangsta/cli"
	"gopkg.in/op/go-logging.v1"

	tunasync "github.com/tuna/tunasync/internal"
)

const (
	listJobsPath    = "/jobs"
	listWorkersPath = "/workers"
	cmdPath         = "/cmd"
)

var logger = logging.MustGetLogger("tunasynctl-cmd")

var baseURL string
var client *http.Client

func initializeWrapper(handler func(*cli.Context)) func(*cli.Context) {
	return func(c *cli.Context) {
		err := initialize(c)
		if err != nil {
			os.Exit(1)
		}
		handler(c)
	}
}

func initialize(c *cli.Context) error {
	// init logger
	tunasync.InitLogger(c.Bool("verbose"), c.Bool("verbose"), false)

	// parse manager server address
	baseURL = c.String("manager")
	if baseURL == "" {
		baseURL = "localhost"
	}
	managerPort := c.String("port")
	if managerPort != "" {
		baseURL += ":" + managerPort
	}
	if c.Bool("no-ssl") {
		baseURL = "http://" + baseURL
	} else {
		baseURL = "https://" + baseURL
	}
	logger.Infof("Use manager address: %s", baseURL)

	// create HTTP client
	var err error
	client, err = tunasync.CreateHTTPClient(c.String("ca-cert"))
	if err != nil {
		err = fmt.Errorf("Error initializing HTTP client: %s", err.Error())
		logger.Error(err.Error())
		return err

	}
	return nil
}

func listWorkers(c *cli.Context) {
	var workers []tunasync.WorkerStatus
	_, err := tunasync.GetJSON(baseURL+listWorkersPath, &workers, client)
	if err != nil {
		logger.Errorf("Filed to correctly get informations from manager server: %s", err.Error())
		os.Exit(1)
	}

	b, err := json.MarshalIndent(workers, "", "  ")
	if err != nil {
		logger.Errorf("Error printing out informations: %s", err.Error())
	}
	fmt.Print(string(b))
}

func listJobs(c *cli.Context) {
	// FIXME: there should be an API on manager server side that return MirrorStatus list to tunasynctl
	var jobs []tunasync.MirrorStatus
	if c.Bool("all") {
		_, err := tunasync.GetJSON(baseURL+listJobsPath, &jobs, client)
		if err != nil {
			logger.Errorf("Filed to correctly get information of all jobs from manager server: %s", err.Error())
			os.Exit(1)
		}

	} else {
		args := c.Args()
		if len(args) == 0 {
			logger.Error("Usage Error: jobs command need at least one arguments or \"--all\" flag.")
			os.Exit(1)
		}
		ans := make(chan []tunasync.MirrorStatus, len(args))
		for _, workerID := range args {
			go func(workerID string) {
				var workerJobs []tunasync.MirrorStatus
				_, err := tunasync.GetJSON(fmt.Sprintf("%s/workers/%s/jobs", baseURL, workerID), &workerJobs, client)
				if err != nil {
					logger.Errorf("Filed to correctly get jobs for worker %s: %s", workerID, err.Error())
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
		logger.Errorf("Error printing out informations: %s", err.Error())
	}
	fmt.Printf(string(b))
}

func cmdJob(cmd tunasync.CmdVerb) func(*cli.Context) {
	return func(c *cli.Context) {
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
			logger.Error("Usage Error: cmd command receive just 1 required positional argument MIRROR and 1 optional ")
			os.Exit(1)
		}

		cmd := tunasync.ClientCmd{
			Cmd:      cmd,
			MirrorID: mirrorID,
			WorkerID: c.String("worker"),
			Args:     argsList,
		}
		resp, err := tunasync.PostJSON(baseURL+cmdPath, cmd, client)
		if err != nil {
			logger.Errorf("Failed to correctly send command: %s", err.Error())
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				logger.Errorf("Failed to parse response: %s", err.Error())
			}

			logger.Errorf("Failed to correctly send command: HTTP status code is not 200: %s", body)
		} else {
			logger.Info("Succesfully send command")
		}
	}
}

func main() {
	app := cli.NewApp()
	app.EnableBashCompletion = true
	app.Version = "0.1"

	commonFlags := []cli.Flag{
		cli.StringFlag{
			Name:  "manager, m",
			Usage: "The manager server address",
		},
		cli.StringFlag{
			Name:  "port, p",
			Usage: "The manager server port",
		},
		cli.StringFlag{
			Name:  "ca-cert, c",
			Usage: "Trust CA cert `CERT`",
		},

		cli.BoolFlag{
			Name:  "no-ssl",
			Usage: "Use http rather than https",
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
			Name:   "workers",
			Usage:  "List workers",
			Flags:  commonFlags,
			Action: initializeWrapper(listWorkers),
		},
		{
			Name:   "start",
			Usage:  "Start a job",
			Flags:  append(commonFlags, cmdFlags...),
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
			Name:   "ping",
			Flags:  append(commonFlags, cmdFlags...),
			Action: initializeWrapper(cmdJob(tunasync.CmdPing)),
		},
	}
	app.Run(os.Args)
}
