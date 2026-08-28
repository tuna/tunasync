package manager

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/tuna/tunasync/internal"
)

func TestMetricsEndpoint(t *testing.T) {
	var listenPort = 15000

	Convey("metrics endpoint works with EnableMetrics=true", t, func(ctx C) {
		listenPort++
		port := listenPort
		
		internal.InitLogger(true, true, false)

		cfg := &Config{
			Debug: true,
			Server: ServerConfig{
				Addr:          "127.0.0.1",
				Port:          port,
				EnableMetrics: true,
			},
			Files: FileConfig{
				DBFile: fmt.Sprintf("/tmp/tunasync_test_metrics_%d.db", port),
				DBType: "bolt",
			},
		}
		defer os.Remove(cfg.Files.DBFile)
		defer func() { manager = nil }()

		s := GetTUNASyncManager(cfg)
		So(s, ShouldNotBeNil)
		go s.Run()
		time.Sleep(100 * time.Millisecond)

		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", port))
		So(err, ShouldBeNil)
		So(resp.StatusCode, ShouldEqual, http.StatusOK)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		So(err, ShouldBeNil)
		bodyStr := string(body)

		// Verify Prometheus format
		So(bodyStr, ShouldContainSubstring, "# HELP")
		So(bodyStr, ShouldContainSubstring, "# TYPE")
		So(bodyStr, ShouldContainSubstring, "tunasync_worker_registered")
	})

	Convey("metrics endpoint returns 404 when default", t, func(ctx C) {
		listenPort++
		port := listenPort
		
		internal.InitLogger(true, true, false)

		cfg := &Config{
			Debug: true,
			Server: ServerConfig{
				Addr: "127.0.0.1",
				Port: port,
			},
			Files: FileConfig{
				DBFile: fmt.Sprintf("/tmp/tunasync_test_no_metrics_%d.db", port),
				DBType: "bolt",
			},
		}
		defer os.Remove(cfg.Files.DBFile)
		defer func() { manager = nil }()

		s := GetTUNASyncManager(cfg)
		So(s, ShouldNotBeNil)
		go s.Run()
		time.Sleep(100 * time.Millisecond)

		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", port))
		So(err, ShouldBeNil)
		So(resp.StatusCode, ShouldEqual, http.StatusNotFound)
		resp.Body.Close()
	})

	Convey("metrics endpoint reports worker and mirror metrics correctly", t, func(ctx C) {
		listenPort++
		port := listenPort
		
		internal.InitLogger(true, true, false)

		cfg := &Config{
			Debug: true,
			Server: ServerConfig{
				Addr:          "127.0.0.1",
				Port:          port,
				EnableMetrics: true,
			},
			Files: FileConfig{
				DBFile: fmt.Sprintf("/tmp/tunasync_test_metrics_full_%d.db", port),
				DBType: "bolt",
			},
		}
		defer os.Remove(cfg.Files.DBFile)
		defer func() { manager = nil }()

		s := GetTUNASyncManager(cfg)
		So(s, ShouldNotBeNil)

		// Set up mock DB adapter with test data
		mockAdapter := &mockDBAdapter{
			workerStore: map[string]internal.WorkerStatus{
				"test_worker_1": {
					ID:           "test_worker_1",
					URL:          "http://localhost:6001",
					Token:        "token123",
					LastOnline:   time.Now().Add(-1 * time.Hour),
					LastRegister: time.Now().Add(-24 * time.Hour),
				},
				"test_worker_2": {
					ID:           "test_worker_2",
					URL:          "http://localhost:6002",
					Token:        "token456",
					LastOnline:   time.Now().Add(-30 * time.Minute),
					LastRegister: time.Now().Add(-12 * time.Hour),
				},
			},
			statusStore: map[string]internal.MirrorStatus{
				"archlinux/test_worker_1": {
					Name:      "archlinux",
					Worker:    "test_worker_1",
					Status:    internal.Success,
					IsMaster:  true,
					Size:      "50G",
					Scheduled: time.Now().Add(1 * time.Hour),
				},
				"debian/test_worker_1": {
					Name:     "debian",
					Worker:   "test_worker_1",
					Status:   internal.Syncing,
					IsMaster: false,
					Size:     "100G",
				},
				"ubuntu/test_worker_2": {
					Name:     "ubuntu",
					Worker:   "test_worker_2",
					Status:   internal.Failed,
					IsMaster: true,
					Size:     "80G",
				},
			},
		}
		s.setDBAdapter(mockAdapter)

		go s.Run()
		time.Sleep(100 * time.Millisecond)

		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", port))
		So(err, ShouldBeNil)
		So(resp.StatusCode, ShouldEqual, http.StatusOK)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		So(err, ShouldBeNil)
		bodyStr := string(body)

		// Verify worker metrics
		So(bodyStr, ShouldContainSubstring, `tunasync_worker_registered 2`)
		So(bodyStr, ShouldContainSubstring, `tunasync_worker_last_online_timestamp{worker_id="test_worker_1"}`)
		So(bodyStr, ShouldContainSubstring, `tunasync_worker_last_online_timestamp{worker_id="test_worker_2"}`)

		// Verify mirror job status metrics
		// Status values: 0=none, 1=failed, 2=success, 3=syncing, 4=pre-syncing, 5=paused, 6=disabled
		So(bodyStr, ShouldContainSubstring, `tunasync_mirror_job_status{is_master="true",mirror_name="archlinux",worker_id="test_worker_1"} 2`)
		So(bodyStr, ShouldContainSubstring, `tunasync_mirror_job_status{is_master="false",mirror_name="debian",worker_id="test_worker_1"} 3`)
		So(bodyStr, ShouldContainSubstring, `tunasync_mirror_job_status{is_master="true",mirror_name="ubuntu",worker_id="test_worker_2"} 1`)

		// Verify size metrics (in GB)
		So(bodyStr, ShouldContainSubstring, `tunasync_mirror_size_gb{mirror_name="archlinux",worker_id="test_worker_1"} 50`)
		So(bodyStr, ShouldContainSubstring, `tunasync_mirror_size_gb{mirror_name="debian",worker_id="test_worker_1"} 100`)
		So(bodyStr, ShouldContainSubstring, `tunasync_mirror_size_gb{mirror_name="ubuntu",worker_id="test_worker_2"} 80`)

		// Verify aggregated status counts
		So(bodyStr, ShouldContainSubstring, `tunasync_mirrors_by_status{status="success"} 1`)
		So(bodyStr, ShouldContainSubstring, `tunasync_mirrors_by_status{status="syncing"} 1`)
		So(bodyStr, ShouldContainSubstring, `tunasync_mirrors_by_status{status="failed"} 1`)
		So(bodyStr, ShouldContainSubstring, `tunasync_total_mirrors 3`)
	})
}

// containsMetricLine checks if a specific metric line exists in the output
// Useful for verifying exact metric values with labels
func containsMetricLine(metrics, metricName string, labels map[string]string, expectedValue string) bool {
	labelParts := make([]string, 0, len(labels))
	for k, v := range labels {
		labelParts = append(labelParts, k+"=\""+v+"\"")
	}

	var sb strings.Builder
	sb.WriteString(metricName)
	if len(labelParts) > 0 {
		sb.WriteString("{")
		for i, part := range labelParts {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(part)
		}
		sb.WriteString("}")
	}
	sb.WriteString(" ")
	sb.WriteString(expectedValue)

	return strings.Contains(metrics, sb.String())
}
