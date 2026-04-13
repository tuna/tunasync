package worker

// put global variables and types here

import (
	tunasync "github.com/tuna/tunasync/internal"
)

type empty struct{}

const defaultMaxRetry = 2

var logger = tunasync.MustGetLogger("tunasync")
