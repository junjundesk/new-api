package model

import (
	"maps"
	"slices"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// loadPricingChannels takes one DB snapshot of enabled abilities and enabled
// channels, including every priority that can be reached by retry. It deliberately
// does not load credentials or infer additional wildcard models/groups.
func loadPricingChannels() ([]AbilityWithChannel, map[int]*Channel, error) {
	var rows []struct {
		AbilityWithChannel
		Setting       *string
		OtherSettings string
	}
	err := DB.Table("abilities").
		Select("abilities.*, channels.type as channel_type, channels.setting, channels.settings as other_settings").
		Joins("JOIN channels ON abilities.channel_id = channels.id").
		Where("abilities.enabled = ? AND channels.status = ?", true, common.ChannelStatusEnabled).
		Scan(&rows).Error
	if err != nil {
		return nil, nil, err
	}

	abilities := make([]AbilityWithChannel, 0, len(rows))
	channels := make(map[int]*Channel)
	for _, row := range rows {
		abilities = append(abilities, row.AbilityWithChannel)
		if _, exists := channels[row.ChannelId]; !exists {
			channels[row.ChannelId] = &Channel{
				Id:            row.ChannelId,
				Type:          row.ChannelType,
				Setting:       row.Setting,
				OtherSettings: row.OtherSettings,
			}
		}
	}
	return abilities, channels, nil
}

// clone returns an independently mutable public price, without the private
// channel candidates. Pointer-valued model ratios also belong to this copy.
func (p Pricing) clone() Pricing {
	p.EnableGroup = slices.Clone(p.EnableGroup)
	p.SupportedEndpointTypes = slices.Clone(p.SupportedEndpointTypes)
	p.BillingRatios = maps.Clone(p.BillingRatios)
	for _, ratio := range []**float64{
		&p.CacheRatio, &p.CreateCacheRatio, &p.ImageRatio,
		&p.AudioRatio, &p.AudioCompletionRatio,
	} {
		if *ratio != nil {
			*ratio = common.GetPointer(**ratio)
		}
	}
	p.billingCandidates = nil
	return p
}

// GetPricingForUser resolves current group settings after copying the static
// snapshot. No per-user price or visible-group selection is written to the cache.
func GetPricingForUser(userGroup string, usableGroups map[string]string) []Pricing {
	snapshot := getPricingSnapshot()
	pricing := make([]Pricing, 0, len(snapshot))
	if len(usableGroups) == 0 {
		return pricing
	}
	for _, cached := range snapshot {
		item := cached.clone()
		item.EnableGroup = make([]string, 0, len(cached.EnableGroup))
		for _, group := range cached.EnableGroup {
			if _, visible := usableGroups[group]; visible || group == "all" {
				item.EnableGroup = append(item.EnableGroup, group)
			}
		}
		if len(item.EnableGroup) == 0 {
			continue
		}

		item.BillingRatios = make(map[string]BillingRatioRange)
		for group, candidates := range cached.billingCandidates {
			if _, visible := usableGroups[group]; !visible {
				continue
			}
			// "all" is a legacy pricing visibility marker, not a routing
			// fallback. Its candidates must not be applied to other groups.
			for i, override := range candidates {
				var overrides map[string]*float64
				if override != nil {
					overrides = map[string]*float64{item.ModelName: override}
				}
				resolved := ratio_setting.ResolveModelGroupRatio(userGroup, group, item.ModelName, overrides)
				if i == 0 {
					item.BillingRatios[group] = BillingRatioRange{
						Min: resolved.GroupRatio, Max: resolved.GroupRatio, Source: resolved.Source,
					}
					continue
				}
				rangeInfo := item.BillingRatios[group]
				rangeInfo.Min = min(rangeInfo.Min, resolved.GroupRatio)
				rangeInfo.Max = max(rangeInfo.Max, resolved.GroupRatio)
				if rangeInfo.Source != resolved.Source {
					rangeInfo.Source = "mixed"
				}
				item.BillingRatios[group] = rangeInfo
			}
		}
		pricing = append(pricing, item)
	}
	return pricing
}
