package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordActivityTokenUsage(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.ActivityGrant{}))
	grant := model.ActivityGrant{
		UserId: 9001, CodeId: 9001, Code: "usage-test", GroupName: "usage-group",
		Status: model.ActivityGrantStatusActive, TokenTotal: 100,
	}
	require.NoError(t, model.DB.Create(&grant).Error)
	t.Cleanup(func() {
		model.DB.Unscoped().Delete(&model.ActivityGrant{}, grant.Id)
	})

	relayInfo := &relaycommon.RelayInfo{
		PriceData: hosttypes.PriceData{
			GroupRatioInfo: hosttypes.GroupRatioInfo{ActivityGrantId: grant.Id},
		},
	}
	recordActivityTokenUsage(relayInfo, 25)
	recordActivityTokenUsage(relayInfo, 0)
	recordActivityTokenUsage(nil, 10)
	recordActivityTokenUsage(&relaycommon.RelayInfo{}, 10)

	reloaded := &model.ActivityGrant{}
	require.NoError(t, model.DB.First(reloaded, "id = ?", grant.Id).Error)
	assert.Equal(t, int64(25), reloaded.TokenUsed)
}
