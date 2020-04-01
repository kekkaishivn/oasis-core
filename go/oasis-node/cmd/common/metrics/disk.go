// Package metrics implements a prometheus metrics service.
package metrics

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/procfs"
	"github.com/spf13/viper"

	"github.com/oasislabs/oasis-core/go/oasis-node/cmd/common"
)

const (
	MetricDiskUsageBytes   = "oasis_worker_disk_usage_bytes"
	MetricDiskReadBytes    = "oasis_worker_disk_read_bytes"
	MetricDiskWrittenBytes = "oasis_worker_disk_written_bytes"
)

var (
	diskUsageGauge          prometheus.Gauge
	diskIOReadBytesGauge    prometheus.Gauge
	diskIOWrittenBytesGauge prometheus.Gauge

	diskServiceOnce sync.Once
)

type diskService struct {
	ResourceMicroService
	sync.Mutex

	dataDir string
	// TODO: Should we monitor I/O of children PIDs as well?
	pid int
}

func (d *diskService) Update() error {
	d.Lock()
	defer d.Unlock()

	// Compute disk usage of datadir.
	var duBytes int64
	err := filepath.Walk(d.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("disk usage metric: failed to access file %s: %v", path, err)
		}
		duBytes += info.Size()
		return nil
	})
	if err != nil {
		return fmt.Errorf("disk usage metric: failed to walk directory %s: %v", d.dataDir, err)
	}
	diskUsageGauge.Set(float64(duBytes))

	// Obtain process I/O info.
	proc, err := procfs.NewProc(d.pid)
	if err != nil {
		return fmt.Errorf("disk I/O metric: failed to obtain proc object for PID %d: %v", d.pid, err)
	}
	procIO, err := proc.IO()
	if err != nil {
		return fmt.Errorf("disk I/O metric: failed to obtain procIO object %d: %v", d.pid, err)
	}

	diskIOWrittenBytesGauge.Set(float64(procIO.ReadBytes))
	diskIOReadBytesGauge.Set(float64(procIO.WriteBytes))

	return nil
}

// NewDiskService constructs a new disk usage and I/O service.
//
// This service will compute the size of datadir folder and read I/O info of the process every --metric.push.interval
// seconds.
func NewDiskService() ResourceMicroService {
	ds := &diskService{
		dataDir: viper.GetString(common.CfgDataDir),
		pid:     os.Getpid(),
	}

	diskUsageGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: MetricDiskUsageBytes,
			Help: "Size of datadir of the worker",
		},
	)

	diskIOReadBytesGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: MetricDiskReadBytes,
			Help: "Read bytes by the worker",
		},
	)

	diskIOWrittenBytesGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: MetricDiskWrittenBytes,
			Help: "Written bytes by the worker",
		},
	)

	// Disk metrics are singletons per process. Ensure to register them only once.
	diskServiceOnce.Do(func() {
		prometheus.MustRegister([]prometheus.Collector{diskUsageGauge, diskIOReadBytesGauge, diskIOWrittenBytesGauge}...)
	})

	return ds
}
