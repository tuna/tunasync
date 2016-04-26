package manager

import (
	"bytes"
	"encoding/json"
	"net/http"
)

func postJSON(url string, obj interface{}) (*http.Response, error) {
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(obj)
	return http.Post(url, "application/json; charset=utf-8", b)
}
