package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	units "github.com/docker/go-units"

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
retry = 3
timeout = 86400

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
retry = 2
timeout = 3600
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
memory_limit = "256MiB"

[[mirrors]]
name = "fedora"
provider = "rsync"
upstream = "rsync://ftp.fedoraproject.org/fedora/"
use_ipv6 = true
memory_limit = "128M"

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
		tmpfile, err := os.CreateTemp("", "tunasync")
		So(err, ShouldEqual, nil)
		defer os.Remove(tmpfile.Name())

		tmpDir, err := os.MkdirTemp("", "tunasync")
		So(err, ShouldBeNil)
		defer os.RemoveAll(tmpDir)

		incSection := fmt.Sprintf(
			"\n[include]\n"+
				"include_mirrors = \"%s/*.conf\"",
			tmpDir,
		)

		curCfgBlob := cfgBlob + incSection

		err = os.WriteFile(tmpfile.Name(), []byte(curCfgBlob), 0644)
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
		err = os.WriteFile(filepath.Join(tmpDir, "debian.conf"), []byte(incBlob1), 0644)
		So(err, ShouldEqual, nil)
		err = os.WriteFile(filepath.Join(tmpDir, "ubuntu.conf"), []byte(incBlob2), 0644)
		So(err, ShouldEqual, nil)

		cfg, err := LoadConfig(tmpfile.Name())
		So(err, ShouldBeNil)
		So(cfg.Global.Name, ShouldEqual, "test_worker")
		So(cfg.Global.Interval, ShouldEqual, 240)
		So(cfg.Global.Retry, ShouldEqual, 3)
		So(cfg.Global.Timeout, ShouldEqual, 86400)
		So(cfg.Global.MirrorDir, ShouldEqual, "/data/mirrors")

		So(cfg.Manager.APIBase, ShouldEqual, "https://127.0.0.1:5000")
		So(cfg.Server.Hostname, ShouldEqual, "worker1.example.com")

		m := cfg.Mirrors[0]
		So(m.Name, ShouldEqual, "AOSP")
		So(m.MirrorDir, ShouldEqual, "/data/git/AOSP")
		So(m.Provider, ShouldEqual, provCommand)
		So(m.Interval, ShouldEqual, 720)
		So(m.Retry, ShouldEqual, 2)
		So(m.Timeout, ShouldEqual, 3600)
		So(m.Env["REPO"], ShouldEqual, "/usr/local/bin/aosp-repo")

		m = cfg.Mirrors[1]
		So(m.Name, ShouldEqual, "debian")
		So(m.MirrorDir, ShouldEqual, "")
		So(m.Provider, ShouldEqual, provTwoStageRsync)
		So(m.MemoryLimit.Value(), ShouldEqual, 256*units.MiB)

		m = cfg.Mirrors[2]
		So(m.Name, ShouldEqual, "fedora")
		So(m.MirrorDir, ShouldEqual, "")
		So(m.Provider, ShouldEqual, provRsync)
		So(m.ExcludeFile, ShouldEqual, "/etc/tunasync.d/fedora-exclude.txt")
		So(m.MemoryLimit.Value(), ShouldEqual, 128*units.MiB)

		m = cfg.Mirrors[3]
		So(m.Name, ShouldEqual, "debian-cd")
		So(m.MirrorDir, ShouldEqual, "")
		So(m.Provider, ShouldEqual, provTwoStageRsync)
		So(m.MemoryLimit.Value(), ShouldEqual, 0)

		m = cfg.Mirrors[4]
		So(m.Name, ShouldEqual, "debian-security")

		m = cfg.Mirrors[5]
		So(m.Name, ShouldEqual, "ubuntu")

		So(len(cfg.Mirrors), ShouldEqual, 6)
	})

	Convey("Everything should work on nested config file", t, func() {
		tmpfile, err := os.CreateTemp("", "tunasync")
		So(err, ShouldEqual, nil)
		defer os.Remove(tmpfile.Name())

		tmpDir, err := os.MkdirTemp("", "tunasync")
		So(err, ShouldBeNil)
		defer os.RemoveAll(tmpDir)

		incSection := fmt.Sprintf(
			"\n[include]\n"+
				"include_mirrors = \"%s/*.conf\"",
			tmpDir,
		)

		curCfgBlob := cfgBlob + incSection

		err = os.WriteFile(tmpfile.Name(), []byte(curCfgBlob), 0644)
		So(err, ShouldEqual, nil)
		defer tmpfile.Close()

		incBlob1 := `
[[mirrors]]
name = "ipv6s"
use_ipv6 = true
	[[mirrors.mirrors]]
	name = "debians"
	mirror_subdir = "debian"
	provider = "two-stage-rsync"
	stage1_profile = "debian"

		[[mirrors.mirrors.mirrors]]
		name = "debian-security"
		upstream = "rsync://test.host/debian-security/"
		[[mirrors.mirrors.mirrors]]
		name = "ubuntu"
		stage1_profile = "ubuntu"
		upstream = "rsync://test.host2/ubuntu/"
	[[mirrors.mirrors]]
	name = "debian-cd"
	provider = "rsync"
	upstream = "rsync://test.host3/debian-cd/"
		`
		err = os.WriteFile(filepath.Join(tmpDir, "nest.conf"), []byte(incBlob1), 0644)
		So(err, ShouldEqual, nil)

		cfg, err := LoadConfig(tmpfile.Name())
		So(err, ShouldBeNil)
		So(cfg.Global.Name, ShouldEqual, "test_worker")
		So(cfg.Global.Interval, ShouldEqual, 240)
		So(cfg.Global.Retry, ShouldEqual, 3)
		So(cfg.Global.MirrorDir, ShouldEqual, "/data/mirrors")

		So(cfg.Manager.APIBase, ShouldEqual, "https://127.0.0.1:5000")
		So(cfg.Server.Hostname, ShouldEqual, "worker1.example.com")

		m := cfg.Mirrors[0]
		So(m.Name, ShouldEqual, "AOSP")
		So(m.MirrorDir, ShouldEqual, "/data/git/AOSP")
		So(m.Provider, ShouldEqual, provCommand)
		So(m.Interval, ShouldEqual, 720)
		So(m.Retry, ShouldEqual, 2)
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
		So(m.Name, ShouldEqual, "debian-security")
		So(m.MirrorDir, ShouldEqual, "")
		So(m.Provider, ShouldEqual, provTwoStageRsync)
		So(m.UseIPv6, ShouldEqual, true)
		So(m.Stage1Profile, ShouldEqual, "debian")

		m = cfg.Mirrors[4]
		So(m.Name, ShouldEqual, "ubuntu")
		So(m.MirrorDir, ShouldEqual, "")
		So(m.Provider, ShouldEqual, provTwoStageRsync)
		So(m.UseIPv6, ShouldEqual, true)
		So(m.Stage1Profile, ShouldEqual, "ubuntu")

		m = cfg.Mirrors[5]
		So(m.Name, ShouldEqual, "debian-cd")
		So(m.UseIPv6, ShouldEqual, true)
		So(m.Provider, ShouldEqual, provRsync)

		So(len(cfg.Mirrors), ShouldEqual, 6)
	})
	Convey("Providers can be inited from a valid config file", t, func() {
		tmpfile, err := os.CreateTemp("", "tunasync")
		So(err, ShouldEqual, nil)
		defer os.Remove(tmpfile.Name())

		err = os.WriteFile(tmpfile.Name(), []byte(cfgBlob), 0644)
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

	Convey("MirrorSubdir should work", t, func() {
		tmpfile, err := os.CreateTemp("", "tunasync")
		So(err, ShouldEqual, nil)
		defer os.Remove(tmpfile.Name())

		cfgBlob1 := `
[global]
name = "test_worker"
log_dir = "/var/log/tunasync/{{.Name}}"
mirror_dir = "/data/mirrors"
concurrent = 10
interval = 240
timeout = 86400
retry = 3

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
name = "ipv6s"
use_ipv6 = true
	[[mirrors.mirrors]]
	name = "debians"
	mirror_subdir = "debian"
	provider = "two-stage-rsync"
	stage1_profile = "debian"

		[[mirrors.mirrors.mirrors]]
		name = "debian-security"
		upstream = "rsync://test.host/debian-security/"
		[[mirrors.mirrors.mirrors]]
		name = "ubuntu"
		stage1_profile = "ubuntu"
		upstream = "rsync://test.host2/ubuntu/"
	[[mirrors.mirrors]]
	name = "debian-cd"
	provider = "rsync"
	upstream = "rsync://test.host3/debian-cd/"
		`
		err = os.WriteFile(tmpfile.Name(), []byte(cfgBlob1), 0644)
		So(err, ShouldEqual, nil)
		defer tmpfile.Close()

		cfg, err := LoadConfig(tmpfile.Name())
		So(err, ShouldBeNil)

		providers := map[string]mirrorProvider{}
		for _, m := range cfg.Mirrors {
			p := newMirrorProvider(m, cfg)
			providers[p.Name()] = p
		}

		p := providers["debian-security"]
		So(p.Name(), ShouldEqual, "debian-security")
		So(p.LogDir(), ShouldEqual, "/var/log/tunasync/debian-security")
		So(p.LogFile(), ShouldEqual, "/var/log/tunasync/debian-security/latest.log")
		r2p, ok := p.(*twoStageRsyncProvider)
		So(ok, ShouldBeTrue)
		So(r2p.stage1Profile, ShouldEqual, "debian")
		So(r2p.WorkingDir(), ShouldEqual, "/data/mirrors/debian/debian-security")

		p = providers["ubuntu"]
		So(p.Name(), ShouldEqual, "ubuntu")
		So(p.LogDir(), ShouldEqual, "/var/log/tunasync/ubuntu")
		So(p.LogFile(), ShouldEqual, "/var/log/tunasync/ubuntu/latest.log")
		r2p, ok = p.(*twoStageRsyncProvider)
		So(ok, ShouldBeTrue)
		So(r2p.stage1Profile, ShouldEqual, "ubuntu")
		So(r2p.WorkingDir(), ShouldEqual, "/data/mirrors/debian/ubuntu")

		p = providers["debian-cd"]
		So(p.Name(), ShouldEqual, "debian-cd")
		So(p.LogDir(), ShouldEqual, "/var/log/tunasync/debian-cd")
		So(p.LogFile(), ShouldEqual, "/var/log/tunasync/debian-cd/latest.log")
		rp, ok := p.(*rsyncProvider)
		So(ok, ShouldBeTrue)
		So(rp.WorkingDir(), ShouldEqual, "/data/mirrors/debian-cd")
		So(p.Timeout(), ShouldEqual, 86400*time.Second)
	})

	Convey("rsync_override_only should work", t, func() {
		tmpfile, err := os.CreateTemp("", "tunasync")
		So(err, ShouldEqual, nil)
		defer os.Remove(tmpfile.Name())

		cfgBlob1 := `
[global]
name = "test_worker"
log_dir = "/var/log/tunasync/{{.Name}}"
mirror_dir = "/data/mirrors"
concurrent = 10
interval = 240
retry = 3
timeout = 86400

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
name = "foo"
provider = "rsync"
upstream = "rsync://foo.bar/"
interval = 720
retry = 2
timeout = 3600
mirror_dir = "/data/foo"
rsync_override = ["--bar", "baz"]
rsync_override_only = true
`

		err = os.WriteFile(tmpfile.Name(), []byte(cfgBlob1), 0644)
		So(err, ShouldEqual, nil)
		defer tmpfile.Close()

		cfg, err := LoadConfig(tmpfile.Name())
		So(err, ShouldBeNil)

		providers := map[string]mirrorProvider{}
		for _, m := range cfg.Mirrors {
			p := newMirrorProvider(m, cfg)
			providers[p.Name()] = p
		}

		p, ok := providers["foo"].(*rsyncProvider)
		So(ok, ShouldBeTrue)
		So(p.options, ShouldResemble, []string{"--bar", "baz"})
	})

	Convey("rsync global options should work", t, func() {
		tmpfile, err := os.CreateTemp("", "tunasync")
		So(err, ShouldEqual, nil)
		defer os.Remove(tmpfile.Name())

		cfgBlob1 := `
[global]
name = "test_worker"
log_dir = "/var/log/tunasync/{{.Name}}"
mirror_dir = "/data/mirrors"
concurrent = 10
interval = 240
retry = 3
timeout = 86400
rsync_options = ["--global"]

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
name = "foo"
provider = "rsync"
upstream = "rsync://foo.bar/"
interval = 720
retry = 2
timeout = 3600
mirror_dir = "/data/foo"
rsync_override = ["--override"]
rsync_options = ["--local"]
`

		err = os.WriteFile(tmpfile.Name(), []byte(cfgBlob1), 0644)
		So(err, ShouldEqual, nil)
		defer tmpfile.Close()

		cfg, err := LoadConfig(tmpfile.Name())
		So(err, ShouldBeNil)

		providers := map[string]mirrorProvider{}
		for _, m := range cfg.Mirrors {
			p := newMirrorProvider(m, cfg)
			providers[p.Name()] = p
		}

		p, ok := providers["foo"].(*rsyncProvider)
		So(ok, ShouldBeTrue)
		So(p.options, ShouldResemble, []string{
			"--override",    // from mirror.rsync_override
			"--timeout=120", // generated by newRsyncProvider
			"--global",      // from global.rsync_options
			"--local",       // from mirror.rsync_options
		})
	})
}
