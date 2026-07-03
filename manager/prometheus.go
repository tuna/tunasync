package manager

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	. "github.com/tuna/tunasync/internal"
)

// prometheusCollector implements prometheus.Collector interface for lazy metric collection
type prometheusCollector struct {
	registry *prometheus.Registry
	mu       sync.RWMutex
	manager  *Manager

	// Worker metrics
	workerRegistered     prometheus.Gauge
	workerLastOnlineTime *prometheus.GaugeVec

	// Mirror job metrics
	mirrorJobStatus      *prometheus.GaugeVec
	mirrorLastUpdate     *prometheus.GaugeVec
	mirrorLastStarted    *prometheus.GaugeVec
	mirrorLastEnded      *prometheus.GaugeVec
	mirrorNextSchedule   *prometheus.GaugeVec
	mirrorIsMaster       *prometheus.GaugeVec
	mirrorSizeBytes      *prometheus.GaugeVec
	mirrorSyncDuration   *prometheus.GaugeVec

	// Aggregated metrics
	totalMirrors    prometheus.Gauge
	mirrorsByStatus *prometheus.GaugeVec
}

// Status mapping for Prometheus gauge values
var statusValue = map[SyncStatus]float64{
	None:       0,
	Failed:     1,
	Success:    2,
	Syncing:    3,
	PreSyncing: 4,
	Paused:     5,
	Disabled:   6,
}

func newPrometheusCollector(manager *Manager, registry *prometheus.Registry) *prometheusCollector {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}

	c := &prometheusCollector{
		registry: registry,
		manager:  manager,

		// Worker metrics
		workerRegistered: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "tunasync",
			Subsystem: "worker",
			Name:      "registered",
			Help:      "Number of registered workers",
		}),
		workerLastOnlineTime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "tunasync",
			Subsystem: "worker",
			Name:      "last_online_timestamp",
			Help:      "Unix timestamp of when the worker was last online",
		}, []string{"worker_id"}),

		// Mirror job metrics
		mirrorJobStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "tunasync",
			Subsystem: "mirror",
			Name:      "job_status",
			Help:      "Current status of mirror jobs (0=none, 1=failed, 2=success, 3=syncing, 4=pre-syncing, 5=paused, 6=disabled)",
		}, []string{"mirror_name", "worker_id", "is_master"}),
		mirrorLastUpdate: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "tunasync",
			Subsystem: "mirror",
			Name:      "last_update_timestamp",
			Help:      "Unix timestamp of last successful update",
		}, []string{"mirror_name", "worker_id"}),
		mirrorLastStarted: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "tunasync",
			Subsystem: "mirror",
			Name:      "last_started_timestamp",
			Help:      "Unix timestamp of when the last sync started",
		}, []string{"mirror_name", "worker_id"}),
		mirrorLastEnded: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "tunasync",
			Subsystem: "mirror",
			Name:      "last_ended_timestamp",
			Help:      "Unix timestamp of when the last sync ended",
		}, []string{"mirror_name", "worker_id"}),
		mirrorNextSchedule: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "tunasync",
			Subsystem: "mirror",
			Name:      "next_schedule_timestamp",
			Help:      "Unix timestamp of next scheduled sync",
		}, []string{"mirror_name", "worker_id"}),
		mirrorIsMaster: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "tunasync",
			Subsystem: "mirror",
			Name:      "is_master",
			Help:      "Whether this mirror is a master (1) or slave (0)",
		}, []string{"mirror_name", "worker_id"}),
		mirrorSizeBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "tunasync",
			Subsystem: "mirror",
			Name:      "size_gb",
			Help:      "Size of the mirror in gigabytes (parsed from size string, 0 if unknown)",
		}, []string{"mirror_name", "worker_id"}),
		mirrorSyncDuration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "tunasync",
			Subsystem: "mirror",
			Name:      "last_sync_duration_seconds",
			Help:      "Duration of the last sync in seconds",
		}, []string{"mirror_name", "worker_id"}),

		// Aggregated metrics
		totalMirrors: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "tunasync",
			Name:      "total_mirrors",
			Help:      "Total number of mirrors managed",
		}),
		mirrorsByStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "tunasync",
			Name:      "mirrors_by_status",
			Help:      "Number of mirrors by status",
		}, []string{"status"}),
	}

	return c
}

// init registers the collector with the registry
func (c *prometheusCollector) init() {
	// Register only the collector - it will handle all metric collection
	c.registry.MustRegister(c)
}

// Describe sends all metric descriptors to Prometheus
func (c *prometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	// Send descriptors for all metrics
	c.workerRegistered.Describe(ch)
	c.workerLastOnlineTime.Describe(ch)
	c.mirrorJobStatus.Describe(ch)
	c.mirrorLastUpdate.Describe(ch)
	c.mirrorLastStarted.Describe(ch)
	c.mirrorLastEnded.Describe(ch)
	c.mirrorNextSchedule.Describe(ch)
	c.mirrorIsMaster.Describe(ch)
	c.mirrorSizeBytes.Describe(ch)
	c.mirrorSyncDuration.Describe(ch)
	c.totalMirrors.Describe(ch)
	c.mirrorsByStatus.Describe(ch)
}

// Collect fetches current state and sends metrics to Prometheus
// This is called on-demand when /metrics is scraped
func (c *prometheusCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.manager == nil {
		return
	}

	// Fetch current state from manager (don't hold manager lock during DB access)
	c.manager.rwmu.RLock()
	adapter := c.manager.adapter
	c.manager.rwmu.RUnlock()

	if adapter == nil {
		return
	}

	workers, err := adapter.ListWorkers()
	if err != nil {
		logger.Errorf("Failed to list workers for prometheus metrics: %s", err.Error())
		return
	}

	mirrors, err := adapter.ListAllMirrorStatus()
	if err != nil {
		logger.Errorf("Failed to list mirrors for prometheus metrics: %s", err.Error())
		return
	}

	// Reset counters for aggregation
	statusCount := make(map[SyncStatus]int)

	// Update worker metrics
	c.workerRegistered.Set(float64(len(workers)))
	c.workerLastOnlineTime.Reset()
	for _, w := range workers {
		c.workerLastOnlineTime.WithLabelValues(w.ID).Set(float64(w.LastOnline.Unix()))
	}

	// Update mirror metrics
	c.mirrorJobStatus.Reset()
	c.mirrorLastUpdate.Reset()
	c.mirrorLastStarted.Reset()
	c.mirrorLastEnded.Reset()
	c.mirrorNextSchedule.Reset()
	c.mirrorIsMaster.Reset()
	c.mirrorSizeBytes.Reset()
	c.mirrorSyncDuration.Reset()

	for _, m := range mirrors {
		isMasterStr := boolToString(m.IsMaster)

		// Status gauge
		c.mirrorJobStatus.WithLabelValues(m.Name, m.Worker, isMasterStr).Set(statusValue[m.Status])

		// Timestamps
		if !m.LastUpdate.IsZero() {
			c.mirrorLastUpdate.WithLabelValues(m.Name, m.Worker).Set(float64(m.LastUpdate.Unix()))
		}
		if !m.LastStarted.IsZero() {
			c.mirrorLastStarted.WithLabelValues(m.Name, m.Worker).Set(float64(m.LastStarted.Unix()))
		}
		if !m.LastEnded.IsZero() {
			c.mirrorLastEnded.WithLabelValues(m.Name, m.Worker).Set(float64(m.LastEnded.Unix()))
		}
		if !m.Scheduled.IsZero() {
			c.mirrorNextSchedule.WithLabelValues(m.Name, m.Worker).Set(float64(m.Scheduled.Unix()))
		}

		// Is master flag
		isMaster := 0.0
		if m.IsMaster {
			isMaster = 1.0
		}
		c.mirrorIsMaster.WithLabelValues(m.Name, m.Worker).Set(isMaster)

		// Size in GB
		sizeGB := parseSizeToGB(m.Size)
		c.mirrorSizeBytes.WithLabelValues(m.Name, m.Worker).Set(sizeGB)

		// Sync duration
		if !m.LastStarted.IsZero() && !m.LastEnded.IsZero() && m.LastEnded.After(m.LastStarted) {
			duration := m.LastEnded.Sub(m.LastStarted).Seconds()
			c.mirrorSyncDuration.WithLabelValues(m.Name, m.Worker).Set(duration)
		}

		// Count for aggregated status
		statusCount[m.Status]++
	}

	// Aggregated status counts
	c.mirrorsByStatus.Reset()
	for status, count := range statusCount {
		c.mirrorsByStatus.WithLabelValues(status.String()).Set(float64(count))
	}

	c.totalMirrors.Set(float64(len(mirrors)))

	// Collect all metrics
	c.workerRegistered.Collect(ch)
	c.workerLastOnlineTime.Collect(ch)
	c.mirrorJobStatus.Collect(ch)
	c.mirrorLastUpdate.Collect(ch)
	c.mirrorLastStarted.Collect(ch)
	c.mirrorLastEnded.Collect(ch)
	c.mirrorNextSchedule.Collect(ch)
	c.mirrorIsMaster.Collect(ch)
	c.mirrorSizeBytes.Collect(ch)
	c.mirrorSyncDuration.Collect(ch)
	c.totalMirrors.Collect(ch)
	c.mirrorsByStatus.Collect(ch)
}

// Registry returns the Prometheus registry for HTTP handler registration
func (c *prometheusCollector) Registry() *prometheus.Registry {
	return c.registry
}

// Helper functions

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// parseSizeToGB parses human-readable size strings like "1.5G", "500M", "12.32T" and returns gigabytes
func parseSizeToGB(sizeStr string) float64 {
	if sizeStr == "" || sizeStr == "unknown" {
		return 0
	}

	// Simple parser for common size suffixes
	var multiplier float64 = 1 // Default: already in GB
	var numStr string

	switch {
	case len(sizeStr) > 0 && sizeStr[len(sizeStr)-1] == 'B':
		// Check for KB, MB, GB, TB
		if len(sizeStr) > 1 {
			switch sizeStr[len(sizeStr)-2] {
			case 'K', 'k':
				multiplier = 1.0 / 1024 / 1024 // KB to GB
				numStr = sizeStr[:len(sizeStr)-2]
			case 'M', 'm':
				multiplier = 1.0 / 1024 // MB to GB
				numStr = sizeStr[:len(sizeStr)-2]
			case 'G', 'g':
				multiplier = 1 // GB
				numStr = sizeStr[:len(sizeStr)-2]
			case 'T', 't':
				multiplier = 1024 // TB to GB
				numStr = sizeStr[:len(sizeStr)-2]
			default:
				// Bytes to GB
				multiplier = 1.0 / 1024 / 1024 / 1024
				numStr = sizeStr[:len(sizeStr)-1]
			}
		}
	case len(sizeStr) > 0 && (sizeStr[len(sizeStr)-1] == 'G' || sizeStr[len(sizeStr)-1] == 'M' || sizeStr[len(sizeStr)-1] == 'K' || sizeStr[len(sizeStr)-1] == 'T'):
		switch sizeStr[len(sizeStr)-1] {
		case 'K', 'k':
			multiplier = 1.0 / 1024 / 1024 // KB to GB
			numStr = sizeStr[:len(sizeStr)-1]
		case 'M', 'm':
			multiplier = 1.0 / 1024 // MB to GB
			numStr = sizeStr[:len(sizeStr)-1]
		case 'G', 'g':
			multiplier = 1 // GB
			numStr = sizeStr[:len(sizeStr)-1]
		case 'T', 't':
			multiplier = 1024 // TB to GB
			numStr = sizeStr[:len(sizeStr)-1]
		}
	}

	if numStr == "" {
		numStr = sizeStr
	}

	// Parse the numeric part
	var result float64
	_, err := fmt.Sscanf(numStr, "%f", &result)
	if err != nil {
		return 0
	}

	return result * multiplier
}
