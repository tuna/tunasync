package internal

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestExtractSizeFromRsyncLog(t *testing.T) {
	realLogContent := `
Number of files: 998,470 (reg: 925,484, dir: 58,892, link: 14,094)
Number of created files: 1,049 (reg: 1,049)
Number of deleted files: 1,277 (reg: 1,277)
Number of regular files transferred: 5,694
Total file size: 1.33T bytes
Total transferred file size: 2.86G bytes
Literal data: 780.62M bytes
Matched data: 2.08G bytes
File list size: 37.55M
File list generation time: 7.845 seconds
File list transfer time: 0.000 seconds
Total bytes sent: 7.55M
Total bytes received: 823.25M

sent 7.55M bytes  received 823.25M bytes  5.11M bytes/sec
total size is 1.33T  speedup is 1,604.11
`
	Convey("Log parser should work", t, func() {
		tmpDir, err := os.MkdirTemp("", "tunasync")
		So(err, ShouldBeNil)
		defer os.RemoveAll(tmpDir)
		logFile := filepath.Join(tmpDir, "rs.log")
		err = os.WriteFile(logFile, []byte(realLogContent), 0755)
		So(err, ShouldBeNil)

		res := ExtractSizeFromRsyncLog(logFile)
		So(res, ShouldEqual, "1.33T")
	})
}
