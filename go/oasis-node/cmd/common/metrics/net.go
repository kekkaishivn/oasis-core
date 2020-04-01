// Package metrics implements a prometheus metrics service.
package metrics

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/procfs"
)

const (
	MetricNetReceiveBytesTotal    = "oasis_worker_net_receive_bytes_total"
	MetricNetReceivePacketsTotal  = "oasis_worker_net_receive_packets_total"
	MetricNetTransmitBytesTotal   = "oasis_worker_net_transmit_bytes_total"
	MetricNetTransmitPacketsTotal = "oasis_worker_net_transmit_packets_total"
)

var (
	receiveBytesGauge    *prometheus.GaugeVec
	receivePacketsGauge  *prometheus.GaugeVec
	transmitBytesGauge   *prometheus.GaugeVec
	transmitPacketsGauge *prometheus.GaugeVec

	netServiceOnce sync.Once
)

type netService struct {
	sync.Mutex
}

func (n *netService) Update() error {
	n.Lock()
	defer n.Unlock()

	// Obtain process Network info.
	proc, err := procfs.NewDefaultFS()
	if err != nil {
		return fmt.Errorf("network metric: failed to obtain proc object: %v", err)
	}
	netDevs, err := proc.NetDev()
	if err != nil {
		return fmt.Errorf("network metric: failed to obtain netDevs object: %v", err)
	}

	for _, netDev := range netDevs {
		receiveBytesGauge.WithLabelValues(netDev.Name).Set(float64(netDev.RxBytes))
		receivePacketsGauge.WithLabelValues(netDev.Name).Set(float64(netDev.RxPackets))
		transmitBytesGauge.WithLabelValues(netDev.Name).Set(float64(netDev.TxBytes))
		transmitPacketsGauge.WithLabelValues(netDev.Name).Set(float64(netDev.TxPackets))
	}

	return nil
}

// NewNetService constructs a new network statistics service.
//
// This service will read info from /proc/net/dev file every --metric.push.interval
// seconds.
func NewNetService() ResourceMicroService {
	ns := &netService{}

	receiveBytesGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: MetricNetReceiveBytesTotal,
			Help: "Number of received bytes",
		},
		[]string{
			// Interface name, e.g. eth0.
			"device",
		})

	receivePacketsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: MetricNetReceivePacketsTotal,
			Help: "Number of received packets",
		},
		[]string{
			// Interface name, e.g. eth0.
			"device",
		})

	transmitBytesGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: MetricNetTransmitBytesTotal,
			Help: "Number of transmitted bytes",
		},
		[]string{
			// Interface name, e.g. eth0.
			"device",
		})

	transmitPacketsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: MetricNetTransmitPacketsTotal,
			Help: "Number of transmitted packets",
		},
		[]string{
			// Interface name, e.g. eth0.
			"device",
		})

	netServiceOnce.Do(func() {
		prometheus.MustRegister([]prometheus.Collector{receiveBytesGauge, receivePacketsGauge, transmitBytesGauge, transmitPacketsGauge}...)
	})

	return ns
}
