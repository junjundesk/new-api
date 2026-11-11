package model

import (
	"strconv"

	"gorm.io/gorm"
)

// inviterCount is one row of a per-inviter aggregate.
type inviterCount struct {
	InviterId int
	Total     int
}

// registeredInviteCounts counts the users currently pointing at each inviter.
// The relationship column is the source of truth: users.aff_count is a legacy
// counter nothing maintains any more, and a manual rebind would desync it.
func registeredInviteCounts(inviterIds []int) (map[int]int, error) {
	counts := make(map[int]int, len(inviterIds))
	if len(inviterIds) == 0 {
		return counts, nil
	}
	var rows []inviterCount
	if err := DB.Model(&User{}).
		Select("inviter_id, COUNT(*) AS total").
		Where("inviter_id IN ?", inviterIds).
		Group("inviter_id").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.InviterId] = row.Total
	}
	return counts, nil
}

// validInviteCounts counts each inviter's valid invitees with the same rule the
// commission tier uses (see countValidInviteesTx), so every page shows the
// number the payout was actually computed from.
func validInviteCounts(inviterIds []int) (map[int]int, error) {
	counts := make(map[int]int, len(inviterIds))
	if len(inviterIds) == 0 {
		return counts, nil
	}
	var rows []ReferralCharge
	if err := DB.Model(&ReferralCharge{}).
		Distinct("inviter_id", "invitee_id").
		Where("inviter_id IN ? AND kind = ? AND counts_valid = ?", inviterIds, ReferralKindInviter, true).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.InviterId]++
	}
	return counts, nil
}

// AttachInviteCounts fills the live invite numbers on one page of users for
// the admin list: AffCount becomes the registered invitee count and
// ValidInviteCount the tier-counting one.
func AttachInviteCounts(users []*User) error {
	ids := make([]int, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.Id)
	}
	registered, err := registeredInviteCounts(ids)
	if err != nil {
		return err
	}
	valid, err := validInviteCounts(ids)
	if err != nil {
		return err
	}
	for _, user := range users {
		user.AffCount = registered[user.Id]
		user.ValidInviteCount = valid[user.Id]
	}
	return nil
}

// ReferralInvitee is one user invited by a given inviter.
type ReferralInvitee struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	CreatedAt   int64  `json:"created_at"`
	Valid       bool   `json:"valid"`        // produced a tier-counting charge for this inviter
	RewardQuota int64  `json:"reward_quota"` // commission this invitee generated for the inviter
}

// ListReferralInvitees returns one page of the users invited by inviterId,
// newest first, with what each one contributed to the inviter.
func ListReferralInvitees(inviterId int, offset int, limit int) ([]ReferralInvitee, int64, error) {
	query := DB.Model(&User{}).Where("inviter_id = ?", inviterId)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []User
	if err := query.Select("id", "username", "created_at").
		Order("id DESC").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	if len(users) == 0 {
		return []ReferralInvitee{}, total, nil
	}

	ids := make([]int, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.Id)
	}
	// One invitee has a handful of charges at most, so the page's charges are
	// folded in Go rather than with dialect-specific boolean aggregates.
	var charges []ReferralCharge
	if err := DB.Select("invitee_id", "counts_valid", "amount", "status").
		Where("inviter_id = ? AND kind = ? AND invitee_id IN ?", inviterId, ReferralKindInviter, ids).
		Find(&charges).Error; err != nil {
		return nil, 0, err
	}
	valid := make(map[int]bool, len(ids))
	reward := make(map[int]int64, len(ids))
	for _, charge := range charges {
		if charge.CountsValid {
			valid[charge.InviteeId] = true
		}
		if charge.Status != ReferralStatusCanceled {
			reward[charge.InviteeId] += charge.Amount
		}
	}

	invitees := make([]ReferralInvitee, 0, len(users))
	for _, user := range users {
		invitees = append(invitees, ReferralInvitee{
			Id:          user.Id,
			Username:    user.Username,
			CreatedAt:   user.CreatedAt,
			Valid:       valid[user.Id],
			RewardQuota: reward[user.Id],
		})
	}
	return invitees, total, nil
}

// ReferralInviter is one row of the admin inviter overview.
type ReferralInviter struct {
	Id              int    `json:"id"`
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	RegisteredCount int    `json:"registered_count"`
	ValidInvitees   int    `json:"valid_invitees"`
	HistoryQuota    int    `json:"history_quota"` // lifetime commission paid out
}

// inviteeOwners scopes a query to users that currently have an inviter,
// optionally narrowed to inviters matching an ID or username keyword.
func inviteeOwners(keyword string) *gorm.DB {
	query := DB.Model(&User{}).Where("inviter_id > ?", 0)
	if keyword == "" {
		return query
	}
	matched := DB.Unscoped().Model(&User{}).Select("id")
	like := "%" + keyword + "%"
	if id, err := strconv.Atoi(keyword); err == nil {
		matched = matched.Where("id = ? OR username LIKE ?", id, like)
	} else {
		matched = matched.Where("username LIKE ?", like)
	}
	return query.Where("inviter_id IN (?)", matched)
}

// ListReferralInviters returns one page of users that have invited at least
// one user, busiest inviter first.
func ListReferralInviters(keyword string, offset int, limit int) ([]ReferralInviter, int64, error) {
	var total int64
	if err := inviteeOwners(keyword).Distinct("inviter_id").Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var counts []inviterCount
	if err := inviteeOwners(keyword).
		Select("inviter_id, COUNT(*) AS total").
		Group("inviter_id").
		Order("total DESC, inviter_id ASC").
		Offset(offset).Limit(limit).
		Find(&counts).Error; err != nil {
		return nil, 0, err
	}
	if len(counts) == 0 {
		return []ReferralInviter{}, total, nil
	}

	ids := make([]int, 0, len(counts))
	for _, row := range counts {
		ids = append(ids, row.InviterId)
	}
	// Unscoped: a deleted inviter still owns its invitees until they are rebound.
	var users []User
	if err := DB.Unscoped().Select("id", "username", "display_name", "aff_history").
		Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	byId := make(map[int]User, len(users))
	for _, user := range users {
		byId[user.Id] = user
	}
	valid, err := validInviteCounts(ids)
	if err != nil {
		return nil, 0, err
	}

	inviters := make([]ReferralInviter, 0, len(counts))
	for _, row := range counts {
		user := byId[row.InviterId]
		inviters = append(inviters, ReferralInviter{
			Id:              row.InviterId,
			Username:        user.Username,
			DisplayName:     user.DisplayName,
			RegisteredCount: row.Total,
			ValidInvitees:   valid[row.InviterId],
			HistoryQuota:    user.AffHistoryQuota,
		})
	}
	return inviters, total, nil
}

// ReferralOverview is the site-wide header of the admin inviter page.
type ReferralOverview struct {
	Inviters      int64 `json:"inviters"`       // users with at least one invitee
	Invitees      int64 `json:"invitees"`       // users that have an inviter
	ValidInvitees int64 `json:"valid_invitees"` // invitee/inviter pairs that counted for a tier
	RewardQuota   int64 `json:"reward_quota"`   // inviter commission recorded and not canceled
}

// GetReferralOverview aggregates the site-wide invite numbers.
func GetReferralOverview() (*ReferralOverview, error) {
	overview := &ReferralOverview{}
	if err := inviteeOwners("").Count(&overview.Invitees).Error; err != nil {
		return nil, err
	}
	if err := inviteeOwners("").Distinct("inviter_id").Count(&overview.Inviters).Error; err != nil {
		return nil, err
	}
	pairs := DB.Model(&ReferralCharge{}).
		Distinct("inviter_id", "invitee_id").
		Where("kind = ? AND counts_valid = ?", ReferralKindInviter, true)
	if err := DB.Table("(?) AS valid_pairs", pairs).Count(&overview.ValidInvitees).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&ReferralCharge{}).
		Where("kind = ? AND status <> ?", ReferralKindInviter, ReferralStatusCanceled).
		Select("COALESCE(SUM(amount), 0)").Scan(&overview.RewardQuota).Error; err != nil {
		return nil, err
	}
	return overview, nil
}
