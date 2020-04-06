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
	MetricCPUUTimeSeconds = "oasis_worker_cpu_utime_seconds"
	MetricCPUSTimeSeconds = "oasis_worker_cpu_stime_seconds"

	// getconf CLK_TCK
	ClockTicks = 100
)

var (
	utimeGauge prometheus.Gauge
	stimeGauge prometheus.Gauge

	cpuServiceOnce sync.Once
)

type cpuCollector struct {
	// TODO: Should we monitor memory of children PIDs as well?
	pid int
}

func (c *cpuCollector) Update() error {
	// Obtain process CPU info.
	proc, err := procfs.NewProc(c.pid)
	if err != nil {
		return fmt.Errorf("CPU metric: failed to obtain proc object for PID %d: %v", c.pid, err)
	}
	procStat, err := proc.Stat()
	if err != nil {
		return fmt.Errorf("CPU metric: failed to obtain procStat object %d: %v", c.pid, err)
	}

	utimeGauge.Set(float64(procStat.UTime) / float64(ClockTicks))
	stimeGauge.Set(float64(procStat.STime) / float64(ClockTicks))

	return nil
}

// NewCPUService constructs a new memory usage service.
//
// This service will read CPU spent time info from process Stat file every
// --metric.push.interval seconds.
func NewCPUService() ResourceCollector {
	cs := &cpuCollector{
		pid: os.Getpid(),
	}

	utimeGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: MetricCPUUTimeSeconds,
			Help: "CPU user time spent by worker",
		},
	)

	stimeGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: MetricCPUSTimeSeconds,
			Help: "CPU system time spent by worker",
		},
	)

	// CPU metrics are singletons per process. Ensure to register them only once.
	cpuServiceOnce.Do(func() {
		prometheus.MustRegister([]prometheus.Collector{utimeGauge, stimeGauge}...)
	})

	return cs
}
