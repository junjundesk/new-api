/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package model

import (
	"context"
	"errors"
	"slices"

	"gorm.io/gorm"
)

var ErrWalletTierSelection = errors.New("invalid wallet tier selection")

// Deliberately read categories and rules only. The wallet must never receive
// issued redemption keys, operator records, stock or purchaser information.
type WalletCardTier struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Quota        int    `json:"quota"`
	BonusQuota   int    `json:"bonus_quota" gorm:"-"`
	BonusRateBps int    `json:"-"`
}

func GetWalletCardTiers(ctx context.Context) ([]WalletCardTier, error) {
	tiers := []WalletCardTier{}
	err := DB.WithContext(ctx).Table("redemption_categories AS c").
		Select("c.id, c.name, c.quota, COALESCE(b.bonus_rate_bps, 0) AS bonus_rate_bps").
		Joins("LEFT JOIN redemption_bonus_rules AS b ON b.category_id = c.id").
		Where("c.wallet_visible = ?", true).Order("c.quota ASC, c.id ASC").Scan(&tiers).Error
	if err != nil {
		return nil, err
	}
	for i := range tiers {
		rule := RedemptionBonusRule{BonusRateBps: tiers[i].BonusRateBps}
		bonus, err := rule.bonusQuota(tiers[i].Quota)
		if err != nil {
			return nil, err
		}
		tiers[i].BonusQuota = bonus
	}
	return tiers, nil
}

// UpdateWalletTierVisibility changes only the named categories, leaving unrelated
// selections, face values, issued codes and bonus rules untouched.
func UpdateWalletTierVisibility(ctx context.Context, show, hide []int) error {
	ids := slices.Concat(show, hide)
	if len(ids) == 0 {
		return ErrWalletTierSelection
	}
	slices.Sort(ids)
	for i, id := range ids {
		if id <= 0 || (i > 0 && id == ids[i-1]) {
			return ErrWalletTierSelection
		}
	}
	const batchSize = 500 // Stay below SQLite's bind-parameter limit.
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock in ID order and reject stale selections before changing any row.
		for batch := range slices.Chunk(ids, batchSize) {
			var found []int
			if err := lockForUpdate(tx).Model(&RedemptionCategory{}).Where("id IN ?", batch).Order("id").Pluck("id", &found).Error; err != nil {
				return err
			}
			if len(found) != len(batch) {
				return ErrRedemptionCategory
			}
		}
		for _, group := range []struct {
			ids     []int
			visible bool
		}{{show, true}, {hide, false}} {
			for batch := range slices.Chunk(group.ids, batchSize) {
				if err := tx.Model(&RedemptionCategory{}).Where("id IN ?", batch).Update("wallet_visible", group.visible).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
