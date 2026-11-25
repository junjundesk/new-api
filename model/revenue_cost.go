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
	"regexp"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm/clause"
)

// These are reporting assumptions, not GroupRatio/ModelRatio billing settings.
const revenueCostOption = "RevenueReportCostGroups"
const MaxRevenueSettingsBytes = 60 << 10 // Fits MySQL TEXT after normalization.

var revenueRatePattern = regexp.MustCompile(`^(?:[0-9]{1,4}(?:[.][0-9]{1,4})?|[.][0-9]{1,4})$`)

type RevenueModelRate struct {
	Model     string `json:"model"`
	CostRatio string `json:"cost_ratio"`
}
type RevenueGroupRate struct {
	Group     string             `json:"group"`
	CostRatio string             `json:"cost_ratio"`
	Models    []RevenueModelRate `json:"models"`
}
type RevenueCostSettings struct {
	Groups []RevenueGroupRate `json:"groups"`
}
type revenueKey struct{ group, model string }

func parseRevenueRate(raw string) (decimal.Decimal, error) {
	if revenueRatePattern.MatchString(raw) {
		d, err := decimal.NewFromString(raw)
		if err == nil && d.LessThanOrEqual(decimal.NewFromInt(1000)) {
			return d, nil
		}
	}
	return decimal.Zero, errors.New("cost multiplier must be between 0 and 1000 with up to four decimal places")
}

// An empty model key is a group default. Map membership (not value != 0)
// distinguishes an explicit zero-cost override from an unconfigured model.
func (s RevenueCostSettings) rates() (map[revenueKey]decimal.Decimal, error) {
	if s.Groups == nil || len(s.Groups) > 128 {
		return nil, errors.New("expected a list of at most 128 cost groups")
	}
	rates := make(map[revenueKey]decimal.Decimal)
	for i := range s.Groups {
		g := &s.Groups[i]
		g.Group = strings.TrimSpace(g.Group)
		key := revenueKey{group: g.Group}
		if _, ok := rates[key]; ok || g.Group == "" || len(g.Group) > 255 || strings.ContainsFunc(g.Group, unicode.IsControl) {
			return nil, errors.New("invalid or duplicate cost group")
		}
		ratio, err := parseRevenueRate(g.CostRatio)
		if err != nil {
			return nil, err
		}
		rates[key], g.CostRatio = ratio, ratio.String()
		if len(g.Models) > 256 {
			return nil, errors.New("too many model cost overrides")
		}
		if g.Models == nil {
			g.Models = []RevenueModelRate{}
		}
		for j := range g.Models {
			m := &g.Models[j]
			m.Model = strings.TrimSpace(m.Model)
			key := revenueKey{g.Group, m.Model}
			if _, ok := rates[key]; ok || m.Model == "" || len(m.Model) > 255 || strings.ContainsFunc(m.Model, unicode.IsControl) {
				return nil, errors.New("invalid or duplicate model in cost group")
			}
			ratio, err := parseRevenueRate(m.CostRatio)
			if err != nil {
				return nil, err
			}
			rates[key], m.CostRatio = ratio, ratio.String()
		}
	}
	return rates, nil
}

func GetRevenueCostSettings(ctx context.Context) (RevenueCostSettings, error) {
	var opt Option
	settings := RevenueCostSettings{Groups: []RevenueGroupRate{}}
	result := DB.WithContext(ctx).Where(&Option{Key: revenueCostOption}).Limit(1).Find(&opt)
	if result.Error != nil || result.RowsAffected == 0 {
		return settings, result.Error
	}
	err := common.UnmarshalJsonStr(opt.Value, &settings)
	return settings, err
}

func SaveRevenueCostSettings(ctx context.Context, settings RevenueCostSettings) error {
	if _, err := settings.rates(); err != nil {
		return err
	}
	raw, err := common.Marshal(settings)
	if err != nil {
		return err
	}
	if len(raw) > MaxRevenueSettingsBytes {
		return errors.New("cost group settings are too large")
	}
	return DB.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"value"})}).Create(&Option{Key: revenueCostOption, Value: string(raw)}).Error
}
