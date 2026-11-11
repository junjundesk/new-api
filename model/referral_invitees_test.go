package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedInviterCharge(t *testing.T, refKey string, inviterId int, inviteeId int, countsValid bool, amount int64, status int) {
	t.Helper()
	require.NoError(t, DB.Create(&ReferralCharge{
		UserId:      inviterId,
		InviterId:   inviterId,
		InviteeId:   inviteeId,
		Kind:        ReferralKindInviter,
		RefKey:      refKey,
		RefKind:     ReferralRefTopUp,
		CountsValid: countsValid,
		Amount:      amount,
		Status:      status,
	}).Error)
}

// seedInviteGraph builds two inviters. Inviter 100 has three invitees: 101 paid
// twice (valid), 102 only has a canceled non-counting charge, 103 never paid.
// Inviter 200 has one valid invitee, 201.
func seedInviteGraph(t *testing.T) {
	t.Helper()
	referralSettleTestReset(t)
	seedBindInviterUser(t, 100, "alpha-inviter", 0)
	seedBindInviterUser(t, 101, "alpha-paid", 100)
	seedBindInviterUser(t, 102, "alpha-canceled", 100)
	seedBindInviterUser(t, 103, "alpha-idle", 100)
	seedBindInviterUser(t, 200, "beta-inviter", 0)
	seedBindInviterUser(t, 201, "beta-paid", 200)

	seedInviterCharge(t, "topup:1", 100, 101, true, 300, ReferralStatusSettled)
	seedInviterCharge(t, "topup:2", 100, 101, false, 200, ReferralStatusPending)
	seedInviterCharge(t, "topup:3", 100, 102, false, 900, ReferralStatusCanceled)
	seedInviterCharge(t, "topup:4", 200, 201, true, 50, ReferralStatusSettled)
}

// The admin list must count invitees from the live relationship: aff_count is
// a legacy column nothing writes, so it used to show 0 for every inviter.
func TestAttachInviteCountsUsesLiveRelationship(t *testing.T) {
	seedInviteGraph(t)

	var users []*User
	require.NoError(t, DB.Where("id IN ?", []int{100, 101, 200}).Order("id").Find(&users).Error)
	require.NoError(t, AttachInviteCounts(users))

	byId := indexUsersById(users)
	assert.Equal(t, 3, byId[100].AffCount)
	assert.Equal(t, 1, byId[100].ValidInviteCount, "a repeat payer counts once and a non-counting charge not at all")
	assert.Equal(t, 0, byId[101].AffCount)
	assert.Equal(t, 0, byId[101].ValidInviteCount)
	assert.Equal(t, 1, byId[200].AffCount)
	assert.Equal(t, 1, byId[200].ValidInviteCount)
}

func TestListReferralInviteesReportsValidityAndReward(t *testing.T) {
	seedInviteGraph(t)

	invitees, total, err := ListReferralInvitees(100, 0, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, invitees, 3)

	assert.Equal(t, ReferralInvitee{Id: 103, Username: "alpha-idle", CreatedAt: invitees[0].CreatedAt}, invitees[0])
	assert.Equal(t, ReferralInvitee{Id: 102, Username: "alpha-canceled", CreatedAt: invitees[1].CreatedAt}, invitees[1],
		"a canceled charge is neither a valid invite nor a reward")
	assert.Equal(t, ReferralInvitee{Id: 101, Username: "alpha-paid", CreatedAt: invitees[2].CreatedAt, Valid: true, RewardQuota: 500}, invitees[2])
}

func TestListReferralInviteesPaginatesAndScopesToInviter(t *testing.T) {
	seedInviteGraph(t)

	page, total, err := ListReferralInvitees(100, 1, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, page, 1)
	assert.Equal(t, 102, page[0].Id)

	none, total, err := ListReferralInvitees(101, 0, 20)
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.Empty(t, none)
}

func TestListReferralInvitersOrdersByInviteCount(t *testing.T) {
	seedInviteGraph(t)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 100).Update("aff_history", 300).Error)

	inviters, total, err := ListReferralInviters("", 0, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Equal(t, []ReferralInviter{
		{Id: 100, Username: "alpha-inviter", RegisteredCount: 3, ValidInvitees: 1, HistoryQuota: 300},
		{Id: 200, Username: "beta-inviter", RegisteredCount: 1, ValidInvitees: 1},
	}, inviters)
}

func TestListReferralInvitersFiltersByKeyword(t *testing.T) {
	seedInviteGraph(t)

	tests := []struct {
		name    string
		keyword string
		wantIds []int
	}{
		{name: "username fragment", keyword: "beta", wantIds: []int{200}},
		{name: "numeric id", keyword: "100", wantIds: []int{100}},
		{name: "invitee name is not an inviter", keyword: "alpha-paid", wantIds: []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inviters, total, err := ListReferralInviters(tt.keyword, 0, 20)
			require.NoError(t, err)
			ids := make([]int, 0, len(inviters))
			for _, inviter := range inviters {
				ids = append(ids, inviter.Id)
			}
			assert.Equal(t, tt.wantIds, ids)
			assert.Equal(t, int64(len(tt.wantIds)), total)
		})
	}
}

func TestGetReferralOverviewAggregatesSiteTotals(t *testing.T) {
	seedInviteGraph(t)

	overview, err := GetReferralOverview()
	require.NoError(t, err)
	assert.Equal(t, ReferralOverview{Inviters: 2, Invitees: 4, ValidInvitees: 2, RewardQuota: 550}, *overview)
}

func TestGetReferralOverviewIsZeroWithoutInvites(t *testing.T) {
	referralSettleTestReset(t)
	seedBindInviterUser(t, 300, "lonely-user", 0)

	overview, err := GetReferralOverview()
	require.NoError(t, err)
	assert.Equal(t, ReferralOverview{}, *overview)
}
