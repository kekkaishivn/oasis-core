// Package metrics implements a prometheus metrics service.
package metrics

import (
	"fmt"
	"os"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/procfs"
)

const (
	MetricMemVmSizeBytes   = "oasis_worker_mem_VmSize_bytes" // nolint: golint
	MetricMemRssAnonBytes  = "oasis_worker_mem_RssAnon_bytes"
	MetricMemRssFileBytes  = "oasis_worker_mem_RssFile_bytes"
	MetricMemRssShmemBytes = "oasis_worker_mem_RssShmem_bytes"
)

var (
	vmSizeGauge   prometheus.Gauge
	rssAnonGauge  prometheus.Gauge
	rssFileGauge  prometheus.Gauge
	rssShmemGauge prometheus.Gauge

	memServiceOnce sync.Once
)

type memCollector struct {
	// TODO: Should we monitor memory of children PIDs as well?
	pid int
}

func (m *memCollector) Update() error {
	// Obtain process Memory info.
	proc, err := procfs.NewProc(m.pid)
	if err != nil {
		return fmt.Errorf("memory metric: failed to obtain proc object for PID %d: %v", m.pid, err)
	}
	procStatus, err := proc.NewStatus()
	if err != nil {
		return fmt.Errorf("memory metric: failed to obtain procStatus object %d: %v", m.pid, err)
	}

	vmSizeGauge.Set(float64(procStatus.VmSize))
	rssAnonGauge.Set(float64(procStatus.RssAnon))
	rssFileGauge.Set(float64(procStatus.RssFile))
	rssShmemGauge.Set(float64(procStatus.RssShmem))

	return nil
}

// NewMemService constructs a new memory usage service.
//
// This service will read memory info from process Status file every --metric.push.interval
// seconds.
func NewMemService() ResourceCollector {
	ms := &memCollector{
		pid: os.Getpid(),
	}

	vmSizeGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: MetricMemVmSizeBytes,
			Help: "Virtual memory size of worker",
		},
	)

	rssAnonGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: MetricMemRssAnonBytes,
			Help: "Size of resident anonymous memory of worker",
		},
	)

	rssFileGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: MetricMemRssFileBytes,
			Help: "Size of resident file mappings of worker",
		},
	)

	rssShmemGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: MetricMemRssShmemBytes,
			Help: "Size of resident shared memory of worker",
		},
	)

	// Memory metrics are singletons per process. Ensure to register them only once.
	memServiceOnce.Do(func() {
		prometheus.MustRegister([]prometheus.Collector{vmSizeGauge, rssAnonGauge, rssFileGauge, rssShmemGauge}...)
	})

	return ms
}
