package internal

import (
	"os"

	"gopkg.in/op/go-logging.v1"
)

// InitLogger initilizes logging format and level
func InitLogger(verbose, debug, withSystemd bool) {
	var fmtString string
	if withSystemd {
		fmtString = "[%{level:.6s}] %{message}"
	} else {
		fmtString = "%{color}[%{time:06-01-02 15:04:05}][%{level:.6s}][%{shortfile}]%{color:reset} %{message}"
	}
	format := logging.MustStringFormatter(fmtString)
	logging.SetFormatter(format)
	logging.SetBackend(logging.NewLogBackend(os.Stdout, "", 0))

	if debug {
		logging.SetLevel(logging.DEBUG, "tunasync")
	} else if verbose {
		logging.SetLevel(logging.INFO, "tunasync")
	} else {
		logging.SetLevel(logging.NOTICE, "tunasync")
	}
}
