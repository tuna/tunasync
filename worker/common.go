package worker

// put global viables and types here

import (
	"gopkg.in/op/go-logging.v1"
)

type empty struct{}

const maxRetry = 2

var logger = logging.MustGetLogger("tunasync")
