package worker

// put global variables and types here

import (
	"gopkg.in/op/go-logging.v1"
)

type empty struct{}

const defaultMaxRetry = 2

var logger = logging.MustGetLogger("tunasync")
