// Package metrics implements a prometheus metrics service.
package metrics

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/prometheus/procfs"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/oasislabs/oasis-core/go/common/service"
	"github.com/oasislabs/oasis-core/go/oasis-node/cmd/common"
)

const (
	CfgMetricsMode         = "metrics.mode"
	CfgMetricsAddr         = "metrics.address"
	CfgMetricsPushJobName  = "metrics.push.job_name"
	CfgMetricsPushLabels   = "metrics.push.labels"
	CfgMetricsPushInterval = "metrics.push.interval"

	MetricsModeNone = "none"
	MetricsModePull = "pull"
	MetricsModePush = "push"

	// getconf CLK_TCK
	ClockTicks = 100
)

var (
	// Flags has the flags used by the metrics service.
	Flags = flag.NewFlagSet("", flag.ContinueOnError)

	diskServiceOnce sync.Once
	memServiceOnce  sync.Once
	cpuServiceOnce  sync.Once
)

// ParseMetricPushLabes is a drop-in replacement for viper.GetStringMapString due to https://github.com/spf13/viper/issues/608.
func ParseMetricPushLabels(val string) map[string]string {
	// viper.GetString() wraps the string inside [] parenthesis, unwrap it.
	val = val[1 : len(val)-1]

	labels := map[string]string{}
	for _, lPair := range strings.Split(val, ",") {
		kv := strings.Split(lPair, "=")
		if len(kv) != 2 || kv[0] == "" {
			continue
		}
		labels[kv[0]] = kv[1]
	}
	return labels
}

// ServiceConfig contains the configuration parameters for metrics.
type ServiceConfig struct {
	// Mode is the service mode ("none", "pull", "push").
	Mode string
	// Address is the address of the push server.
	Address string
	// JobName is the name of the job for which metrics are collected.
	JobName string
	// Labels are the key-value labels of the job being collected for.
	Labels map[string]string
	// Interval defined the push interval for metrics collection.
	Interval time.Duration
}

// GetServiceConfig gets the metrics configuration parameter struct.
func GetServiceConfig() *ServiceConfig {
	return &ServiceConfig{
		Mode:     viper.GetString(CfgMetricsMode),
		Address:  viper.GetString(CfgMetricsAddr),
		JobName:  viper.GetString(CfgMetricsPushJobName),
		Labels:   ParseMetricPushLabels(viper.GetString(CfgMetricsPushLabels)),
		Interval: viper.GetDuration(CfgMetricsPushInterval),
	}
}

type stubService struct {
	service.BaseBackgroundService

	dService *diskService
	mService *memService
	cService *cpuService
}

func (s *stubService) Start() error {
	if err := s.dService.Start(); err != nil {
		return err
	}

	return nil
}

func (s *stubService) Stop() {}

func (s *stubService) Cleanup() {}

func newStubService() (service.BackgroundService, error) {
	svc := *service.NewBaseBackgroundService("metrics")

	d, err := NewDiskService()
	if err != nil {
		return nil, err
	}

	m, err := NewMemService()
	if err != nil {
		return nil, err
	}

	c, err := NewCPUService()
	if err != nil {
		return nil, err
	}

	return &stubService{
		BaseBackgroundService: svc,
		dService:              d.(*diskService),
		mService:              m.(*memService),
		cService:              c.(*cpuService),
	}, nil
}

type pullService struct {
	service.BaseBackgroundService

	ln net.Listener
	s  *http.Server

	ctx   context.Context
	errCh chan error

	dService *diskService
	mService *memService
	cService *cpuService
}

func (s *pullService) Start() error {
	if err := s.dService.Start(); err != nil {
		return err
	}
	if err := s.mService.Start(); err != nil {
		return err
	}
	if err := s.cService.Start(); err != nil {
		return err
	}

	go func() {
		if err := s.s.Serve(s.ln); err != nil {
			s.BaseBackgroundService.Stop()
			s.errCh <- err
		}
	}()
	return nil
}

func (s *pullService) Stop() {
	if s.s != nil {
		select {
		case err := <-s.errCh:
			if err != nil {
				s.Logger.Error("metrics terminated uncleanly",
					"err", err,
				)
			}
		default:
			_ = s.s.Shutdown(s.ctx)
		}
		s.s = nil
	}
}

func (s *pullService) Cleanup() {
	if s.ln != nil {
		_ = s.ln.Close()
		s.ln = nil
	}
}

func newPullService(ctx context.Context) (service.BackgroundService, error) {
	addr := viper.GetString(CfgMetricsAddr)

	svc := *service.NewBaseBackgroundService("metrics")

	svc.Logger.Debug("Metrics Server Params",
		"mode", MetricsModePull,
		"addr", addr,
	)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	d, err := NewDiskService()
	if err != nil {
		return nil, err
	}

	m, err := NewMemService()
	if err != nil {
		return nil, err
	}

	c, err := NewCPUService()
	if err != nil {
		return nil, err
	}

	return &pullService{
		BaseBackgroundService: svc,
		ctx:                   ctx,
		ln:                    ln,
		s:                     &http.Server{Handler: promhttp.Handler()},
		errCh:                 make(chan error),
		dService:              d.(*diskService),
		mService:              m.(*memService),
		cService:              c.(*cpuService),
	}, nil
}

type pushService struct {
	service.BaseBackgroundService

	pusher   *push.Pusher
	interval time.Duration

	dService *diskService
	mService *memService
	cService *cpuService
}

func (s *pushService) Start() error {
	if err := s.dService.Start(); err != nil {
		return err
	}
	if err := s.mService.Start(); err != nil {
		return err
	}
	if err := s.cService.Start(); err != nil {
		return err
	}

	s.pusher = s.pusher.Gatherer(prometheus.DefaultGatherer)

	go s.worker()
	return nil
}

func (s *pushService) worker() {
	t := time.NewTicker(s.interval)
	defer t.Stop()

	for {
		select {
		case <-s.Quit():
			break
		case <-t.C:
		}

		if err := s.pusher.Push(); err != nil {
			s.Logger.Warn("Push: failed",
				"err", err,
			)
		}
	}
}

func newPushService() (service.BackgroundService, error) {
	addr := viper.GetString(CfgMetricsAddr)
	jobName := viper.GetString(CfgMetricsPushJobName)
	labels := ParseMetricPushLabels(viper.GetString(CfgMetricsPushLabels))
	interval := viper.GetDuration(CfgMetricsPushInterval)

	if jobName == "" {
		return nil, fmt.Errorf("metrics: metrics.push.job_name required for push mode")
	}
	if labels["instance"] == "" {
		return nil, fmt.Errorf("metrics: at least 'instance' key should be set for metrics.push.labels. Provided labels: %v, viper raw: %v", labels, viper.GetString(CfgMetricsPushLabels))
	}

	svc := *service.NewBaseBackgroundService("metrics")

	svc.Logger.Debug("Metrics Server Params",
		"mode", MetricsModePush,
		"addr", addr,
		"job_name", jobName,
		"labels", labels,
		"push_interval", interval,
	)

	pusher := push.New(addr, jobName)
	for k, v := range labels {
		pusher = pusher.Grouping(k, v)
	}

	d, err := NewDiskService()
	if err != nil {
		return nil, err
	}

	m, err := NewMemService()
	if err != nil {
		return nil, err
	}

	c, err := NewCPUService()
	if err != nil {
		return nil, err
	}

	return &pushService{
		BaseBackgroundService: svc,
		pusher:                pusher,
		interval:              interval,
		dService:              d.(*diskService),
		mService:              m.(*memService),
		cService:              c.(*cpuService),
	}, nil
}

// New constructs a new metrics service.
func New(ctx context.Context) (service.BackgroundService, error) {
	mode := viper.GetString(CfgMetricsMode)
	switch strings.ToLower(mode) {
	case MetricsModeNone:
		return newStubService()
	case MetricsModePull:
		return newPullService(ctx)
	case MetricsModePush:
		return newPushService()
	default:
		return nil, fmt.Errorf("metrics: unsupported mode: '%v'", mode)
	}
}

type diskService struct {
	service.BaseBackgroundService
	sync.Mutex

	dataDir string
	// TODO: Should we monitor I/O of children PIDs as well?
	pid      int
	interval time.Duration

	diskUsageGauge          prometheus.Gauge
	diskIOReadBytesGauge    prometheus.Gauge
	diskIOWrittenBytesGauge prometheus.Gauge
}

func (d *diskService) Start() error {
	go d.worker()
	return nil
}

// updateDiskUsage walks through dataDir and updates diskUsageGauge attribute.
func (d *diskService) updateDiskUsage() error {
	d.Lock()
	defer d.Unlock()

	// Compute disk usage of datadir.
	var duBytes int64
	err := filepath.Walk(d.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("disk usage metric: failed to access file %s", path))
		}
		duBytes += info.Size()
		return nil
	})
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("disk usage metric: failed to walk directory %s", d.dataDir))
	}
	d.diskUsageGauge.Set(float64(duBytes))

	return nil
}

// updateDiskUsage reads process info and updates diskIO{Read|Written}BytesGauge attributes.
func (d *diskService) updateIO() error {
	d.Lock()
	defer d.Unlock()

	// Obtain process I/O info.
	proc, err := procfs.NewProc(d.pid)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("disk I/O metric: failed to obtain proc object for PID %d", d.pid))
	}
	procIO, err := proc.IO()
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("disk I/O metric: failed to obtain procIO object %d", d.pid))
	}

	d.diskIOWrittenBytesGauge.Set(float64(procIO.ReadBytes))
	d.diskIOReadBytesGauge.Set(float64(procIO.WriteBytes))

	return nil
}

func (d *diskService) worker() {
	t := time.NewTicker(d.interval)
	defer t.Stop()

	for {
		select {
		case <-d.Quit():
			break
		case <-t.C:
		}

		if err := d.updateDiskUsage(); err != nil {
			d.Logger.Warn(err.Error())
		}

		if err := d.updateIO(); err != nil {
			d.Logger.Warn(err.Error())
		}
	}
}

// NewDiskService constructs a new disk usage and I/O service.
//
// This service will compute the size of datadir folder and read I/O info of the process every --metric.push.interval
// seconds.
func NewDiskService() (service.BackgroundService, error) {
	ds := &diskService{
		BaseBackgroundService: *service.NewBaseBackgroundService("disk"),
		dataDir:               viper.GetString(common.CfgDataDir),
		pid:                   os.Getpid(),
		interval:              viper.GetDuration(CfgMetricsPushInterval),

		diskUsageGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "oasis_worker_disk_usage_bytes",
				Help: "Size of datadir of the worker",
			},
		),

		diskIOReadBytesGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "oasis_worker_disk_read_bytes",
				Help: "Read bytes by the worker",
			},
		),

		diskIOWrittenBytesGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "oasis_worker_disk_written_bytes",
				Help: "Written bytes by the worker",
			},
		),
	}

	diskCollectors := []prometheus.Collector{
		ds.diskUsageGauge,
		ds.diskIOReadBytesGauge,
		ds.diskIOWrittenBytesGauge,
	}

	// Disk metrics are singletons per process. Ensure to register them only once.
	diskServiceOnce.Do(func() {
		prometheus.MustRegister(diskCollectors...)
	})

	return ds, nil
}

type memService struct {
	service.BaseBackgroundService
	sync.Mutex

	// TODO: Should we monitor memory of children PIDs as well?
	pid      int
	interval time.Duration

	VmSizeGauge   prometheus.Gauge // nolint: golint
	RssAnonGauge  prometheus.Gauge
	RssFileGauge  prometheus.Gauge
	RssShmemGauge prometheus.Gauge
}

func (m *memService) Start() error {
	go m.worker()
	return nil
}

// updateMemory updates current memory usage attributes.
func (m *memService) updateMemory() error {
	m.Lock()
	defer m.Unlock()

	/// Obtain process Memory info.
	proc, err := procfs.NewProc(m.pid)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("memory metric: failed to obtain proc object for PID %d", m.pid))
	}
	procStatus, err := proc.NewStatus()
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("memory metric: failed to obtain procStatus object %d", m.pid))
	}

	m.VmSizeGauge.Set(float64(procStatus.VmSize))
	m.RssAnonGauge.Set(float64(procStatus.RssAnon))
	m.RssFileGauge.Set(float64(procStatus.RssFile))
	m.RssShmemGauge.Set(float64(procStatus.RssShmem))

	return nil
}

func (m *memService) worker() {
	t := time.NewTicker(m.interval)
	defer t.Stop()

	for {
		select {
		case <-m.Quit():
			break
		case <-t.C:
		}

		if err := m.updateMemory(); err != nil {
			m.Logger.Warn(err.Error())
		}
	}
}

// NewMemService constructs a new memory usage service.
//
// This service will read memory info from process Status file every --metric.push.interval
// seconds.
func NewMemService() (service.BackgroundService, error) {
	ms := &memService{
		BaseBackgroundService: *service.NewBaseBackgroundService("mem"),
		pid:                   os.Getpid(),
		interval:              viper.GetDuration(CfgMetricsPushInterval),

		VmSizeGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "oasis_worker_mem_VmSize_bytes",
				Help: "Virtual memory size of worker",
			},
		),

		RssAnonGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "oasis_worker_mem_RssAnon_bytes",
				Help: "Size of resident anonymous memory of worker",
			},
		),

		RssFileGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "oasis_worker_mem_RssFile_bytes",
				Help: "Size of resident file mappings of worker",
			},
		),

		RssShmemGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "oasis_worker_mem_RssShmem_bytes",
				Help: "Size of resident shared memory of worker",
			},
		),
	}

	memCollectors := []prometheus.Collector{
		ms.VmSizeGauge,
		ms.RssAnonGauge,
		ms.RssFileGauge,
		ms.RssShmemGauge,
	}

	// Memory metrics are singletons per process. Ensure to register them only once.
	memServiceOnce.Do(func() {
		prometheus.MustRegister(memCollectors...)
	})

	return ms, nil
}

type cpuService struct {
	service.BaseBackgroundService
	sync.Mutex

	// TODO: Should we monitor memory of children PIDs as well?
	pid      int
	interval time.Duration

	utimeGauge prometheus.Gauge
	stimeGauge prometheus.Gauge
}

func (c *cpuService) Start() error {
	go c.worker()
	return nil
}

// updateCPU updates current memory usage attributes.
func (c *cpuService) updateCPU() error {
	c.Lock()
	defer c.Unlock()

	/// Obtain process CPU info.
	proc, err := procfs.NewProc(c.pid)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("CPU metric: failed to obtain proc object for PID %d", c.pid))
	}
	procStat, err := proc.Stat()
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("CPU metric: failed to obtain procStat object %d", c.pid))
	}

	c.utimeGauge.Set(float64(procStat.UTime) / float64(ClockTicks))
	c.stimeGauge.Set(float64(procStat.STime) / float64(ClockTicks))

	return nil
}

func (c *cpuService) worker() {
	t := time.NewTicker(c.interval)
	defer t.Stop()

	for {
		select {
		case <-c.Quit():
			break
		case <-t.C:
		}

		if err := c.updateCPU(); err != nil {
			c.Logger.Warn(err.Error())
		}
	}
}

// NewCPUService constructs a new memory usage service.
//
// This service will read CPU spent time info from process Stat file every
// --metric.push.interval seconds.
func NewCPUService() (service.BackgroundService, error) {
	cs := &cpuService{
		BaseBackgroundService: *service.NewBaseBackgroundService("cpu"),
		pid:                   os.Getpid(),
		interval:              viper.GetDuration(CfgMetricsPushInterval),

		utimeGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "oasis_worker_cpu_utime_seconds",
				Help: "CPU user time spent by worker",
			},
		),

		stimeGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "oasis_worker_cpu_stime_seconds",
				Help: "CPU system time spent by worker",
			},
		),
	}

	cpuCollectors := []prometheus.Collector{
		cs.utimeGauge,
		cs.stimeGauge,
	}

	// CPU metrics are singletons per process. Ensure to register them only once.
	cpuServiceOnce.Do(func() {
		prometheus.MustRegister(cpuCollectors...)
	})

	return cs, nil
}

// EscapeLabelCharacters replaces invalid prometheus label name characters with "_".
func EscapeLabelCharacters(l string) string {
	return strings.Replace(l, ".", "_", -1)
}

func init() {
	Flags.String(CfgMetricsMode, MetricsModeNone, "metrics mode: none, pull, push")
	Flags.String(CfgMetricsAddr, "127.0.0.1:3000", "metrics pull/push address")
	Flags.String(CfgMetricsPushJobName, "", "metrics push job name")
	Flags.StringToString(CfgMetricsPushLabels, map[string]string{}, "metrics push instance label")
	Flags.Duration(CfgMetricsPushInterval, 5*time.Second, "metrics push interval")

	_ = viper.BindPFlags(Flags)
}
