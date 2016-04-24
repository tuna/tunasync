package worker

import (
	"io/ioutil"
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestConfig(t *testing.T) {
	var cfgBlob = `
[global]
name = "test_worker"
token = "some_token"
log_dir = "/var/log/tunasync"
mirror_dir = "/data/mirrors"
concurrent = 10
interval = 240

[[mirrors]]
name = "AOSP"
provider = "command"
upstream = "https://aosp.google.com/"
interval = 720
mirror_dir = "/data/git/AOSP"
	[mirrors.env]
	REPO = "/usr/local/bin/aosp-repo"

[[mirrors]]
name = "debian"
provider = "two-stage-rsync"
stage1_profile = "debian"
upstream = "rsync://ftp.debian.org/debian/"
use_ipv6 = true


[[mirrors]]
name = "fedora"
provider = "rsync"
upstream = "rsync://ftp.fedoraproject.org/fedora/"
use_ipv6 = true
exclude_file = "/etc/tunasync.d/fedora-exclude.txt"
	`

	Convey("When giving invalid file", t, func() {
		cfg, err := loadConfig("/path/to/invalid/file")
		So(err, ShouldNotBeNil)
		So(cfg, ShouldBeNil)
	})

	Convey("Everything should work on valid config file", t, func() {
		tmpfile, err := ioutil.TempFile("", "tunasync")
		So(err, ShouldEqual, nil)
		defer os.Remove(tmpfile.Name())

		err = ioutil.WriteFile(tmpfile.Name(), []byte(cfgBlob), 0644)
		So(err, ShouldEqual, nil)
		defer tmpfile.Close()

		cfg, err := loadConfig(tmpfile.Name())
		So(err, ShouldBeNil)
		So(cfg.Global.Name, ShouldEqual, "test_worker")
		So(cfg.Global.Interval, ShouldEqual, 240)
		So(cfg.Global.MirrorDir, ShouldEqual, "/data/mirrors")

		m := cfg.Mirrors[0]
		So(m.Name, ShouldEqual, "AOSP")
		So(m.MirrorDir, ShouldEqual, "/data/git/AOSP")
		So(m.Provider, ShouldEqual, ProvCommand)
		So(m.Interval, ShouldEqual, 720)
		So(m.Env["REPO"], ShouldEqual, "/usr/local/bin/aosp-repo")

		m = cfg.Mirrors[1]
		So(m.Name, ShouldEqual, "debian")
		So(m.MirrorDir, ShouldEqual, "")
		So(m.Provider, ShouldEqual, ProvTwoStageRsync)

		m = cfg.Mirrors[2]
		So(m.Name, ShouldEqual, "fedora")
		So(m.MirrorDir, ShouldEqual, "")
		So(m.Provider, ShouldEqual, ProvRsync)
		So(m.ExcludeFile, ShouldEqual, "/etc/tunasync.d/fedora-exclude.txt")

		So(len(cfg.Mirrors), ShouldEqual, 3)

	})
}