package worker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	units "github.com/docker/go-units"

	"github.com/codeskyblue/go-sh"
	. "github.com/smartystreets/goconvey/convey"
)

func cmdRun(p string, args []string) {
	cmd := exec.Command(p, args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Debugf("cmdRun failed %s", err)
		return
	}
	logger.Debugf("cmdRun: ", string(out))
}

func getDockerByName(name string) (string, error) {
	// docker ps -f 'name=$name' --format '{{.Names}}'
	out, err := sh.Command(
		"docker", "ps", "-a",
		"--filter", "name="+name,
		"--format", "{{.Names}}",
	).Output()
	if err == nil {
		logger.Debugf("docker ps: '%s'", string(out))
	}
	return string(out), err
}

func TestDocker(t *testing.T) {
	Convey("Docker Should Work", t, func(ctx C) {
		tmpDir, err := os.MkdirTemp("", "tunasync")
		defer os.RemoveAll(tmpDir)
		So(err, ShouldBeNil)
		cmdScript := filepath.Join(tmpDir, "cmd.sh")
		tmpFile := filepath.Join(tmpDir, "log_file")
		expectedOutput := "HELLO_WORLD"

		c := cmdConfig{
			name:        "tuna-docker",
			upstreamURL: "http://mirrors.tuna.moe/",
			command:     "/bin/cmd.sh",
			workingDir:  tmpDir,
			logDir:      tmpDir,
			logFile:     tmpFile,
			interval:    600 * time.Second,
			env: map[string]string{
				"TEST_CONTENT": expectedOutput,
			},
		}

		cmdScriptContent := `#!/bin/sh
echo ${TEST_CONTENT}
sleep 20
`
		err = os.WriteFile(cmdScript, []byte(cmdScriptContent), 0755)
		So(err, ShouldBeNil)

		provider, err := newCmdProvider(c)
		So(err, ShouldBeNil)

		d := &dockerHook{
			emptyHook: emptyHook{
				provider: provider,
			},
			image: "alpine:3.8",
			volumes: []string{
				fmt.Sprintf("%s:%s", cmdScript, "/bin/cmd.sh"),
			},
			memoryLimit: 512 * units.MiB,
		}
		provider.AddHook(d)
		So(provider.Docker(), ShouldNotBeNil)

		err = d.preExec()
		So(err, ShouldBeNil)

		cmdRun("docker", []string{"images"})
		exitedErr := make(chan error, 1)
		go func() {
			err = provider.Run(make(chan empty, 1))
			logger.Debugf("provider.Run() exited")
			if err != nil {
				logger.Errorf("provider.Run() failed: %v", err)
			}
			exitedErr <- err
		}()

		// Wait for docker running
		for wait := 0; wait < 8; wait++ {
			names, err := getDockerByName(d.Name())
			So(err, ShouldBeNil)
			if names != "" {
				break
			}
			time.Sleep(1 * time.Second)
		}
		// cmdRun("ps", []string{"aux"})

		// assert container running
		names, err := getDockerByName(d.Name())
		So(err, ShouldBeNil)
		So(names, ShouldEqual, d.Name()+"\n")

		err = provider.Terminate()
		So(err, ShouldBeNil)

		// cmdRun("ps", []string{"aux"})
		<-exitedErr

		// container should be terminated and removed
		names, err = getDockerByName(d.Name())
		So(err, ShouldBeNil)
		So(names, ShouldEqual, "")

		// check log content
		loggedContent, err := os.ReadFile(provider.LogFile())
		So(err, ShouldBeNil)
		So(string(loggedContent), ShouldEqual, expectedOutput+"\n")

		d.postExec()
	})
}
