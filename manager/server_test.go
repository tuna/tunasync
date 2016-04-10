package manager

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestHTTPServer(t *testing.T) {
	Convey("HTTP server should work", t, func() {
		s := makeHTTPServer(false)
		So(s, ShouldNotBeNil)
		port := rand.Intn(10000) + 20000
		go func() {
			s.Run(fmt.Sprintf("127.0.0.1:%d", port))
		}()
		time.Sleep(50 * time.Microsecond)
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/ping", port))
		So(err, ShouldBeNil)
		So(resp.StatusCode, ShouldEqual, http.StatusOK)
		So(resp.Header.Get("Content-Type"), ShouldEqual, "application/json; charset=utf-8")
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		So(err, ShouldBeNil)
		var p map[string]string
		err = json.Unmarshal(body, &p)
		So(err, ShouldBeNil)
		So(p["msg"], ShouldEqual, "pong")
	})

}
