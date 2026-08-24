package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	vipTierMaintenanceTickInterval = 1 * time.Minute
	vipTierMaintenanceBatchSize    = 300
)

var (
	vipTierMaintenanceOnce    sync.Once
	vipTierMaintenanceRunning atomic.Bool
)

func StartVipTierMaintenanceTask() {
	vipTierMaintenanceOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("vip tier maintenance task started: tick=%s", vipTierMaintenanceTickInterval))
			ticker := time.NewTicker(vipTierMaintenanceTickInterval)
			defer ticker.Stop()

			runVipTierMaintenanceOnce()
			for range ticker.C {
				runVipTierMaintenanceOnce()
			}
		})
	})
}

func runVipTierMaintenanceOnce() {
	if !vipTierMaintenanceRunning.CompareAndSwap(false, true) {
		return
	}
	defer vipTierMaintenanceRunning.Store(false)

	ctx := context.Background()
	expired := 0
	for {
		n, err := model.ExpireDueVipTiers(vipTierMaintenanceBatchSize)
		expired += n
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("vip tier expiry task failed: %v", err))
			return
		}
		if n < vipTierMaintenanceBatchSize {
			break
		}
	}

	// Backstop for promotions the payment paths missed; normally a no-op
	// because each credited payment promotes inline.
	promoted, err := model.AutoPromoteDueVipTiers(vipTierMaintenanceBatchSize)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("vip tier promotion sweep failed: %v", err))
	}
	if expired > 0 || promoted > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("vip tier maintenance: expired_count=%d, promoted_count=%d", expired, promoted))
	}
}
