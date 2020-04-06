// Package metrics implements a prometheus metrics service.
package metrics

import (
	"time"

	"github.com/spf13/viper"

	"github.com/oasislabs/oasis-core/go/common/service"
)

// ResourceCollector is interface for monitoring resources (cpu, mem, disk, net).
type ResourceCollector interface {
	// Update updates corresponding resource metrics.
	Update() error
}

// resourceService updates and collects number of resources.
type resourceService struct {
	service.BaseBackgroundService

	interval         time.Duration
	resourceServices []ResourceCollector
}

func (brs *resourceService) Start() error {
	go brs.worker()
	return nil
}

func (brs *resourceService) worker() {
	t := time.NewTicker(brs.interval)
	defer t.Stop()

	for {
		select {
		case <-brs.Quit():
			break
		case <-t.C:
		}
		for _, rs := range brs.resourceServices {
			if err := rs.Update(); err != nil {
				brs.Logger.Warn(err.Error())
			}
		}
	}
}

func newResourceService() *resourceService {
	rs := &resourceService{
		BaseBackgroundService: *service.NewBaseBackgroundService("resources_watcher"),
		interval:              viper.GetDuration(CfgMetricsPushInterval),
		resourceServices: []ResourceCollector{
			NewDiskService(),
			NewMemService(),
			NewCPUService(),
			NewNetService(),
		},
	}

	return rs
}
