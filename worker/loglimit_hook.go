package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// limit

type logLimiter struct {
	emptyHook
}

func newLogLimiter(provider mirrorProvider) *logLimiter {
	return &logLimiter{
		emptyHook: emptyHook{
			provider: provider,
		},
	}
}

type fileSlice []os.FileInfo

func (f fileSlice) Len() int           { return len(f) }
func (f fileSlice) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f fileSlice) Less(i, j int) bool { return f[i].ModTime().Before(f[j].ModTime()) }

func (l *logLimiter) preExec() error {
	logger.Debugf("executing log limitter for %s", l.provider.Name())

	p := l.provider
	if p.LogFile() == "/dev/null" {
		return nil
	}

	logDir := p.LogDir()
	files, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(logDir, 0755)
		} else {
			return err
		}
	}
	matchedFiles := []os.FileInfo{}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), p.Name()) {
			info, _ := f.Info()
			matchedFiles = append(matchedFiles, info)
		}
	}

	// sort the filelist in time order
	// earlier modified files are sorted as larger
	sort.Sort(
		sort.Reverse(
			fileSlice(matchedFiles),
		),
	)
	// remove old files
	if len(matchedFiles) > 9 {
		for _, f := range matchedFiles[9:] {
			// logger.Debug(f.Name())
			os.Remove(filepath.Join(logDir, f.Name()))
		}
	}

	logFileName := fmt.Sprintf(
		"%s_%s.log",
		p.Name(),
		time.Now().Format("2006-01-02_15_04"),
	)
	logFilePath := filepath.Join(
		logDir, logFileName,
	)

	logLink := filepath.Join(logDir, "latest")

	if _, err = os.Lstat(logLink); err == nil {
		os.Remove(logLink)
	}
	os.Symlink(logFileName, logLink)

	ctx := p.EnterContext()
	ctx.Set(_LogFileKey, logFilePath)
	return nil
}

func (l *logLimiter) postSuccess() error {
	l.provider.ExitContext()
	return nil
}

func (l *logLimiter) postFail() error {
	logFile := l.provider.LogFile()
	logFileFail := logFile + ".fail"
	logDir := l.provider.LogDir()
	logLink := filepath.Join(logDir, "latest")
	os.Rename(logFile, logFileFail)
	os.Remove(logLink)
	logFileName := filepath.Base(logFileFail)
	os.Symlink(logFileName, logLink)

	l.provider.ExitContext()
	return nil
}
