//go:build ignore
// +build ignore

package main

import (
	"fmt"

	"github.com/tuna/tunasync/manager"
)

func main() {
	cfg, err := manager.LoadConfig("manager.conf", nil)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	m := manager.GetTUNASyncManager(cfg)
	m.Run()
}
