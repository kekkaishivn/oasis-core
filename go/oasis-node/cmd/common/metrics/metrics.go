// Package metrics implements a prometheus metrics service.
package metrics

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/oasislabs/oasis-core/go/common/service"
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
)

var (
	// Flags has the flags used by the metrics service.
	Flags = flag.NewFlagSet("", flag.ContinueOnError)
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

type stubService struct {
	service.BaseBackgroundService

	rService *resourceService
}

func (s *stubService) Start() error {
	if err := s.rService.Start(); err != nil {
		return err
	}

	return nil
}

func (s *stubService) Stop() {}

func (s *stubService) Cleanup() {}

func newStubService() (service.BackgroundService, error) {
	svc := *service.NewBaseBackgroundService("metrics")

	return &stubService{
		BaseBackgroundService: svc,
		rService:              newResourceService(),
	}, nil
}

type pullService struct {
	service.BaseBackgroundService

	ln net.Listener
	s  *http.Server

	ctx   context.Context
	errCh chan error

	rService *resourceService
}

func (s *pullService) Start() error {
	if err := s.rService.Start(); err != nil {
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

	return &pullService{
		BaseBackgroundService: svc,
		ctx:                   ctx,
		ln:                    ln,
		s:                     &http.Server{Handler: promhttp.Handler()},
		errCh:                 make(chan error),
		rService:              newResourceService(),
	}, nil
}

type pushService struct {
	service.BaseBackgroundService

	pusher   *push.Pusher
	interval time.Duration

	rService *resourceService
}

func (s *pushService) Start() error {
	if err := s.rService.Start(); err != nil {
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

	return &pushService{
		BaseBackgroundService: svc,
		pusher:                pusher,
		interval:              interval,
		rService:              newResourceService(),
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
