// +build ignore

package main

import (
	"fmt"

	"github.com/tuna/tunasync/worker"
)

func main() {
	cfg, err := worker.LoadConfig("worker.conf")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	m := worker.GetTUNASyncWorker(cfg)
	m.Run()
}
