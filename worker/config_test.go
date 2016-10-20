package worker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestConfig(t *testing.T) {
	var cfgBlob = `
[global]
name = "test_worker"
log_dir = "/var/log/tunasync/{{.Name}}"
mirror_dir = "/data/mirrors"
concurrent = 10
interval = 240

[manager]
api_base = "https://127.0.0.1:5000"
token = "some_token"

[server]
hostname = "worker1.example.com"
listen_addr = "127.0.0.1"
listen_port = 6000
ssl_cert = "/etc/tunasync.d/worker1.cert"
ssl_key = "/etc/tunasync.d/worker1.key"

[[mirrors]]
name = "AOSP"
provider = "command"
upstream = "https://aosp.google.com/"
interval = 720
mirror_dir = "/data/git/AOSP"
exec_on_success = [
	"bash -c 'echo ${TUNASYNC_JOB_EXIT_STATUS} > ${TUNASYNC_WORKING_DIR}/exit_status'"
]
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
exec_on_failure = [
	"bash -c 'echo ${TUNASYNC_JOB_EXIT_STATUS} > ${TUNASYNC_WORKING_DIR}/exit_status'"
]
	`

	Convey("When giving invalid file", t, func() {
		cfg, err := LoadConfig("/path/to/invalid/file")
		So(err, ShouldNotBeNil)
		So(cfg, ShouldBeNil)
	})

	Convey("Everything should work on valid config file", t, func() {
		tmpfile, err := ioutil.TempFile("", "tunasync")
		So(err, ShouldEqual, nil)
		defer os.Remove(tmpfile.Name())

		tmpDir, err := ioutil.TempDir("", "tunasync")
		So(err, ShouldBeNil)
		defer os.RemoveAll(tmpDir)

		incSection := fmt.Sprintf(
			"\n[include]\n"+
				"include_mirrors = \"%s/*.conf\"",
			tmpDir,
		)

		cfgBlob = cfgBlob + incSection

		err = ioutil.WriteFile(tmpfile.Name(), []byte(cfgBlob), 0644)
		So(err, ShouldEqual, nil)
		defer tmpfile.Close()

		incBlob1 := `
[[mirrors]]
name = "debian-cd"
provider = "two-stage-rsync"
stage1_profile = "debian"
use_ipv6 = true

[[mirrors]]
name = "debian-security"
provider = "two-stage-rsync"
stage1_profile = "debian"
use_ipv6 = true
		`
		incBlob2 := `
[[mirrors]]
name = "ubuntu"
provider = "two-stage-rsync"
stage1_profile = "debian"
use_ipv6 = true
		`
		err = ioutil.WriteFile(filepath.Join(tmpDir, "debian.conf"), []byte(incBlob1), 0644)
		So(err, ShouldEqual, nil)
		err = ioutil.WriteFile(filepath.Join(tmpDir, "ubuntu.conf"), []byte(incBlob2), 0644)
		So(err, ShouldEqual, nil)

		cfg, err := LoadConfig(tmpfile.Name())
		So(err, ShouldBeNil)
		So(cfg.Global.Name, ShouldEqual, "test_worker")
		So(cfg.Global.Interval, ShouldEqual, 240)
		So(cfg.Global.MirrorDir, ShouldEqual, "/data/mirrors")

		So(cfg.Manager.APIBase, ShouldEqual, "https://127.0.0.1:5000")
		So(cfg.Server.Hostname, ShouldEqual, "worker1.example.com")

		m := cfg.Mirrors[0]
		So(m.Name, ShouldEqual, "AOSP")
		So(m.MirrorDir, ShouldEqual, "/data/git/AOSP")
		So(m.Provider, ShouldEqual, provCommand)
		So(m.Interval, ShouldEqual, 720)
		So(m.Env["REPO"], ShouldEqual, "/usr/local/bin/aosp-repo")

		m = cfg.Mirrors[1]
		So(m.Name, ShouldEqual, "debian")
		So(m.MirrorDir, ShouldEqual, "")
		So(m.Provider, ShouldEqual, provTwoStageRsync)

		m = cfg.Mirrors[2]
		So(m.Name, ShouldEqual, "fedora")
		So(m.MirrorDir, ShouldEqual, "")
		So(m.Provider, ShouldEqual, provRsync)
		So(m.ExcludeFile, ShouldEqual, "/etc/tunasync.d/fedora-exclude.txt")

		m = cfg.Mirrors[3]
		So(m.Name, ShouldEqual, "debian-cd")
		So(m.MirrorDir, ShouldEqual, "")
		So(m.Provider, ShouldEqual, provTwoStageRsync)

		m = cfg.Mirrors[4]
		So(m.Name, ShouldEqual, "debian-security")

		m = cfg.Mirrors[5]
		So(m.Name, ShouldEqual, "ubuntu")

		So(len(cfg.Mirrors), ShouldEqual, 6)
	})

	Convey("Providers can be inited from a valid config file", t, func() {
		tmpfile, err := ioutil.TempFile("", "tunasync")
		So(err, ShouldEqual, nil)
		defer os.Remove(tmpfile.Name())

		err = ioutil.WriteFile(tmpfile.Name(), []byte(cfgBlob), 0644)
		So(err, ShouldEqual, nil)
		defer tmpfile.Close()

		cfg, err := LoadConfig(tmpfile.Name())
		So(err, ShouldBeNil)

		providers := map[string]mirrorProvider{}
		for _, m := range cfg.Mirrors {
			p := newMirrorProvider(m, cfg)
			providers[p.Name()] = p
		}

		p := providers["AOSP"]
		So(p.Name(), ShouldEqual, "AOSP")
		So(p.LogDir(), ShouldEqual, "/var/log/tunasync/AOSP")
		So(p.LogFile(), ShouldEqual, "/var/log/tunasync/AOSP/latest.log")
		_, ok := p.(*cmdProvider)
		So(ok, ShouldBeTrue)
		for _, hook := range p.Hooks() {
			switch h := hook.(type) {
			case *execPostHook:
				So(h.command, ShouldResemble, []string{"bash", "-c", `echo ${TUNASYNC_JOB_EXIT_STATUS} > ${TUNASYNC_WORKING_DIR}/exit_status`})
			}
		}

		p = providers["debian"]
		So(p.Name(), ShouldEqual, "debian")
		So(p.LogDir(), ShouldEqual, "/var/log/tunasync/debian")
		So(p.LogFile(), ShouldEqual, "/var/log/tunasync/debian/latest.log")
		r2p, ok := p.(*twoStageRsyncProvider)
		So(ok, ShouldBeTrue)
		So(r2p.stage1Profile, ShouldEqual, "debian")
		So(r2p.WorkingDir(), ShouldEqual, "/data/mirrors/debian")

		p = providers["fedora"]
		So(p.Name(), ShouldEqual, "fedora")
		So(p.LogDir(), ShouldEqual, "/var/log/tunasync/fedora")
		So(p.LogFile(), ShouldEqual, "/var/log/tunasync/fedora/latest.log")
		rp, ok := p.(*rsyncProvider)
		So(ok, ShouldBeTrue)
		So(rp.WorkingDir(), ShouldEqual, "/data/mirrors/fedora")
		So(rp.excludeFile, ShouldEqual, "/etc/tunasync.d/fedora-exclude.txt")

	})
}
