package worker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/codeskyblue/go-sh"
	. "github.com/smartystreets/goconvey/convey"
)

func getDockerByName(name string) (string, error) {
	// docker ps -f 'name=$name' --format '{{.Names}}'
	out, err := sh.Command(
		"docker", "ps",
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
		tmpDir, err := ioutil.TempDir("", "tunasync")
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
sleep 10
`
		err = ioutil.WriteFile(cmdScript, []byte(cmdScriptContent), 0755)
		So(err, ShouldBeNil)

		provider, err := newCmdProvider(c)
		So(err, ShouldBeNil)

		d := &dockerHook{
			emptyHook: emptyHook{
				provider: provider,
			},
			image: "alpine",
			volumes: []string{
				fmt.Sprintf("%s:%s", cmdScript, "/bin/cmd.sh"),
			},
		}
		provider.AddHook(d)
		So(provider.Docker(), ShouldNotBeNil)

		err = d.preExec()
		So(err, ShouldBeNil)

		go func() {
			err = provider.Run()
			ctx.So(err, ShouldNotBeNil)
		}()

		// Wait for docker running
		time.Sleep(5 * time.Second)

		// assert container running
		names, err := getDockerByName(d.Name())
		So(err, ShouldBeNil)
		So(names, ShouldEqual, d.Name()+"\n")

		err = provider.Terminate()
		So(err, ShouldBeNil)

		// container should be terminated and removed
		names, err = getDockerByName(d.Name())
		So(err, ShouldBeNil)
		So(names, ShouldEqual, "")

		// check log content
		loggedContent, err := ioutil.ReadFile(provider.LogFile())
		So(err, ShouldBeNil)
		So(string(loggedContent), ShouldEqual, expectedOutput+"\n")

		d.postExec()
	})
}
