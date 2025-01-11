package manager

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/urfave/cli"
)

func TestConfig(t *testing.T) {
	var cfgBlob = `
	debug = true
	[server]
	addr = "0.0.0.0"
	port = 5000

	[files]
	status_file = "/tmp/tunasync.json"
	db_file = "/var/lib/tunasync/tunasync.db"
	`

	Convey("toml decoding should work", t, func() {

		var conf Config
		_, err := toml.Decode(cfgBlob, &conf)
		ShouldEqual(err, nil)
		ShouldEqual(conf.Server.Addr, "0.0.0.0")
		ShouldEqual(conf.Server.Port, 5000)
		ShouldEqual(conf.Files.StatusFile, "/tmp/tunasync.json")
		ShouldEqual(conf.Files.DBFile, "/var/lib/tunasync/tunasync.db")
	})

	Convey("load Config should work", t, func() {
		Convey("create config file & cli context", func() {
			tmpfile, err := os.CreateTemp("", "tunasync")
			So(err, ShouldEqual, nil)
			defer os.Remove(tmpfile.Name())

			err = os.WriteFile(tmpfile.Name(), []byte(cfgBlob), 0644)
			So(err, ShouldEqual, nil)
			defer tmpfile.Close()

			app := cli.NewApp()
			app.Flags = []cli.Flag{
				cli.StringFlag{
					Name: "config, c",
				},
				cli.StringFlag{
					Name: "addr",
				},
				cli.IntFlag{
					Name: "port",
				},
				cli.StringFlag{
					Name: "cert",
				},
				cli.StringFlag{
					Name: "key",
				},
				cli.StringFlag{
					Name: "status-file",
				},
				cli.StringFlag{
					Name: "db-file",
				},
			}
			Convey("when giving no config options", func() {
				app.Action = func(c *cli.Context) {
					cfgFile := c.String("config")
					cfg, err := LoadConfig(cfgFile, c)
					So(err, ShouldEqual, nil)
					So(cfg.Server.Addr, ShouldEqual, "127.0.0.1")
				}
				args := strings.Split("cmd", " ")
				app.Run(args)
			})
			Convey("when giving config options", func() {
				app.Action = func(c *cli.Context) {
					cfgFile := c.String("config")
					So(cfgFile, ShouldEqual, tmpfile.Name())
					conf, err := LoadConfig(cfgFile, c)
					So(err, ShouldEqual, nil)
					So(conf.Server.Addr, ShouldEqual, "0.0.0.0")
					So(conf.Server.Port, ShouldEqual, 5000)
					So(conf.Files.StatusFile, ShouldEqual, "/tmp/tunasync.json")
					So(conf.Files.DBFile, ShouldEqual, "/var/lib/tunasync/tunasync.db")

				}
				cmd := fmt.Sprintf("cmd -c %s", tmpfile.Name())
				args := strings.Split(cmd, " ")
				app.Run(args)
			})
			Convey("when giving cli options", func() {
				app.Action = func(c *cli.Context) {
					cfgFile := c.String("config")
					So(cfgFile, ShouldEqual, "")
					conf, err := LoadConfig(cfgFile, c)
					So(err, ShouldEqual, nil)
					So(conf.Server.Addr, ShouldEqual, "0.0.0.0")
					So(conf.Server.Port, ShouldEqual, 5001)
					So(conf.Server.SSLCert, ShouldEqual, "/ssl.cert")
					So(conf.Server.SSLKey, ShouldEqual, "/ssl.key")
					So(conf.Files.StatusFile, ShouldEqual, "/tunasync.json")
					So(conf.Files.DBFile, ShouldEqual, "/tunasync.db")

				}
				args := strings.Split(
					"cmd --addr=0.0.0.0 --port=5001 --cert=/ssl.cert --key /ssl.key --status-file=/tunasync.json --db-file=/tunasync.db",
					" ",
				)
				app.Run(args)
			})
			Convey("when giving both config and cli options", func() {
				app.Action = func(c *cli.Context) {
					cfgFile := c.String("config")
					So(cfgFile, ShouldEqual, tmpfile.Name())
					conf, err := LoadConfig(cfgFile, c)
					So(err, ShouldEqual, nil)
					So(conf.Server.Addr, ShouldEqual, "0.0.0.0")
					So(conf.Server.Port, ShouldEqual, 5000)
					So(conf.Server.SSLCert, ShouldEqual, "/ssl.cert")
					So(conf.Server.SSLKey, ShouldEqual, "/ssl.key")
					So(conf.Files.StatusFile, ShouldEqual, "/tunasync.json")
					So(conf.Files.DBFile, ShouldEqual, "/tunasync.db")

				}
				cmd := fmt.Sprintf(
					"cmd -c %s --cert=/ssl.cert --key /ssl.key --status-file=/tunasync.json --db-file=/tunasync.db",
					tmpfile.Name(),
				)
				args := strings.Split(cmd, " ")
				app.Run(args)
			})
		})
	})
}
