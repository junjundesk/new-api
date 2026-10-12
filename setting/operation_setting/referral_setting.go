package operation_setting

import (
	"sort"

	"github.com/QuantumNous/new-api/setting/config"
)

// ReferralTier is one inviter commission bracket. The tier with the largest
// MinInvitees that the inviter's valid invitee count reaches applies.
type ReferralTier struct {
	// MinInvitees is the number of valid invitees required for this tier.
	MinInvitees int `json:"min_invitees"`
	// RateBps is the commission rate in basis points, so 100 = 1%.
	RateBps int `json:"rate_bps"`
}

// ReferralSetting controls the invite-a-friend program. It has two reward
// tracks with different payout shapes:
//
//   - The invitee's first-top-up reward is granted as expiring bonus quota
//     (赠金) and is spendable only, never withdrawable.
//   - The inviter's commission accrues to the claimable referral balance
//     (users.aff_quota) backed by an expiring grant, and can be moved to the
//     main balance by the user.
type ReferralSetting struct {
	Enabled bool `json:"enabled"`

	// InviteeFirstTopUpEnabled toggles the invitee-side reward.
	InviteeFirstTopUpEnabled bool `json:"invitee_first_topup_enabled"`
	// InviteeFirstTopUpRateBps is the share of the invitee's first top-up paid
	// back to the invitee, independent of the inviter's commission.
	InviteeFirstTopUpRateBps int `json:"invitee_first_topup_rate_bps"`
	// InviteeFirstTopUpMaxQuota caps that reward in quota units; 0 = uncapped.
	InviteeFirstTopUpMaxQuota int64 `json:"invitee_first_topup_max_quota"`

	// InviterEnabled toggles the inviter-side commission.
	InviterEnabled bool `json:"inviter_enabled"`
	// InviterTiers lists inviter commission brackets. Order is irrelevant;
	// lookup sorts by MinInvitees.
	InviterTiers []ReferralTier `json:"inviter_tiers"`
	// InviterMaxQuota caps one commission payout in quota units; 0 = uncapped.
	InviterMaxQuota int64 `json:"inviter_max_quota"`

	// BonusDuration is how long reward bonus stays valid, parsed by
	// model.ParseBonusDuration (bare number = hours, so "720" = 30 days).
	// "0" (or blank) means the reward never expires.
	BonusDuration string `json:"bonus_duration"`

	// CountRedeemAsValid controls whether a redemption-code top-up counts
	// toward the inviter's valid invitee count. When false only an online
	// payment makes the invitee "valid", but redemption still pays rewards.
	CountRedeemAsValid bool `json:"count_redeem_as_valid"`
}

var referralSetting = ReferralSetting{
	Enabled:                   false,
	InviteeFirstTopUpEnabled:  true,
	InviteeFirstTopUpRateBps:  300,
	InviteeFirstTopUpMaxQuota: 0,
	InviterEnabled:            true,
	InviterTiers: []ReferralTier{
		{MinInvitees: 1, RateBps: 100},
		{MinInvitees: 3, RateBps: 300},
		{MinInvitees: 10, RateBps: 500},
		{MinInvitees: 30, RateBps: 800},
	},
	InviterMaxQuota:    0,
	BonusDuration:      "0",
	CountRedeemAsValid: true,
}

func init() {
	config.GlobalConfig.Register("referral_setting", &referralSetting)
}

func GetReferralSetting() *ReferralSetting {
	return &referralSetting
}

// RateBpsForInviteeCount returns the commission rate that applies to an
// inviter with the given number of valid invitees. A count below every tier
// floor yields 0, so a malformed or empty tier list disables payouts instead
// of paying an unconfigured rate.
func (s *ReferralSetting) RateBpsForInviteeCount(count int) int {
	rate := 0
	bestFloor := -1
	for _, tier := range s.InviterTiers {
		if tier.RateBps <= 0 || tier.MinInvitees <= 0 {
			continue
		}
		if count >= tier.MinInvitees && tier.MinInvitees > bestFloor {
			bestFloor = tier.MinInvitees
			rate = tier.RateBps
		}
	}
	return rate
}

// SortedInviterTiers returns the configured tiers ordered by MinInvitees, so
// the settings UI and the user-facing invite page show the same ladder.
func (s *ReferralSetting) SortedInviterTiers() []ReferralTier {
	tiers := make([]ReferralTier, len(s.InviterTiers))
	copy(tiers, s.InviterTiers)
	sort.SliceStable(tiers, func(i, j int) bool {
		return tiers[i].MinInvitees < tiers[j].MinInvitees
	})
	return tiers
}
