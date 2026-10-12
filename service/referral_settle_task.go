package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	referralSettleTickInterval = 1 * time.Minute
	// referralSettleMaxBatches bounds how much of a backlog one tick drains, so
	// a large backlog cannot monopolize a tick and starve other maintenance.
	referralSettleMaxBatches = 5
)

var referralSettleOnce sync.Once

// StartReferralSettlementTask pays out pending referral rewards. Charges are
// recorded inside the payment transaction (so a credited top-up always has its
// reward on file) and are converted to bonus grants here, off the payment
// callback path.
func StartReferralSettlementTask() {
	referralSettleOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("referral settlement task started: tick=%s", referralSettleTickInterval))
			ticker := time.NewTicker(referralSettleTickInterval)
			defer ticker.Stop()

			runReferralSettlementOnce()
			for range ticker.C {
				runReferralSettlementOnce()
			}
		})
	})
}

func runReferralSettlementOnce() {
	ctx := context.Background()
	for i := 0; i < referralSettleMaxBatches; i++ {
		settled, err := model.SettlePendingReferralCharges()
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("referral settlement failed: %v", err))
			return
		}
		// A short batch means the backlog is drained.
		if settled == 0 {
			return
		}
	}
}
