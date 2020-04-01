// Package metrics implements a prometheus metrics service.
package metrics

import (
	"sync"
	"time"

	"github.com/spf13/viper"

	"github.com/oasislabs/oasis-core/go/common/service"
)

// ResourceMicroService is interface for monitoring resources (cpu, mem, disk, net).
type ResourceMicroService interface {
	// Update updates corresponding resource metrics.
	Update() error
}

// resourceService is a base implementation of ResourceMicroService.
type resourceService struct {
	service.BaseBackgroundService
	sync.Mutex

	interval         time.Duration
	resourceServices []ResourceMicroService
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
		resourceServices: []ResourceMicroService{
			NewDiskService(),
			NewMemService(),
			NewCPUService(),
			NewNetService(),
		},
	}

	return rs
}
