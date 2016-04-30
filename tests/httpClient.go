// +build ignore

package main

import (
	"fmt"

	"github.com/tuna/tunasync/internal"
)

func main() {
	cfg, err := internal.GetTLSConfig("rootCA.crt")
	fmt.Println(err)
	var msg map[string]string
	resp, err := internal.GetJSON("https://localhost:5002/", &msg, cfg)
	fmt.Println(err)
	fmt.Println(resp)
}
