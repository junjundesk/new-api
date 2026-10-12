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
import { EXCLUDED_GROUPS, FILTER_ALL, QUOTA_TYPE_VALUES } from '../constants'
import type { BillingRatio, PricingModel } from '../types'

// ----------------------------------------------------------------------------
// Model Helper Utilities
// ----------------------------------------------------------------------------

/**
 * Get available groups for a model
 */
export function getAvailableGroups(
  model: PricingModel,
  usableGroup: Record<string, { desc: string; ratio: number }>
): string[] {
  const modelEnableGroups = Array.isArray(model.enable_groups)
    ? model.enable_groups
    : []

  return Object.keys(usableGroup)
    .filter((g) => !EXCLUDED_GROUPS.includes(g))
    .filter((g) => modelEnableGroups.includes(g))
}

/**
 * Read a configured group ratio while preserving valid zero ratios.
 */
export function getConfiguredGroupRatio(
  groupRatio: Record<string, number>,
  group: string
): number {
  const ratio = groupRatio[group]
  return typeof ratio === 'number' && Number.isFinite(ratio) ? ratio : 1
}

/**
 * Resolve the group ratio used by model square summary prices.
 *
 * When no specific group is selected, the model square shows the best price
 * available to the viewer. When a group filter is active, it shows that
 * group's price instead.
 */
export function getGroupBillingRatio(
  model: PricingModel,
  group: string,
  groupRatio: Record<string, number> = model.group_ratio ?? {}
): BillingRatio {
  const configured = model.billing_ratios?.[group]
  if (
    configured &&
    Number.isFinite(configured.min) &&
    Number.isFinite(configured.max) &&
    configured.min >= 0 &&
    configured.max >= configured.min
  ) {
    return configured
  }
  const ratio = getConfiguredGroupRatio(groupRatio, group)
  return { min: ratio, max: ratio, source: 'group' }
}

export function getDisplayBillingRatio(
  model: PricingModel,
  selectedGroup?: string
): BillingRatio {
  if (
    selectedGroup &&
    selectedGroup !== FILTER_ALL &&
    model.enable_groups?.includes(selectedGroup)
  ) {
    return getGroupBillingRatio(model, selectedGroup)
  }
  // The all-groups view advertises the lowest available effective price, not
  // a made-up single group or the cheapest default group multiplier.
  const available = (model.enable_groups ?? []).filter(
    (group) =>
      Object.hasOwn(model.billing_ratios ?? {}, group) ||
      Object.hasOwn(model.group_ratio ?? {}, group)
  )
  let minimum = Number.POSITIVE_INFINITY
  for (const group of available) {
    minimum = Math.min(minimum, getGroupBillingRatio(model, group).min)
  }
  const ratio = Number.isFinite(minimum) ? minimum : 1
  return { min: ratio, max: ratio, source: 'group' }
}

export function getDisplayGroupRatio(
  model: PricingModel,
  selectedGroup?: string
): number {
  return getDisplayBillingRatio(model, selectedGroup).min
}

/**
 * Whether the effective billing ratio differs from 1×, meaning the displayed
 * price is discounted or surcharged relative to the raw model price.
 */
export function hasBillingDiscount(
  model: PricingModel,
  selectedGroup?: string
): boolean {
  const ratio = getDisplayBillingRatio(model, selectedGroup)
  return ratio.min !== 1 || ratio.max !== 1
}

/**
 * Whether a specific group's billing ratio differs from 1×.
 */
export function hasGroupBillingDiscount(
  model: PricingModel,
  group: string,
  groupRatio: Record<string, number> = model.group_ratio ?? {}
): boolean {
  const ratio = getGroupBillingRatio(model, group, groupRatio)
  return ratio.min !== 1 || ratio.max !== 1
}

export function formatBillingRatio(
  ratio: Pick<BillingRatio, 'min' | 'max'>
): string {
  if (ratio.min === ratio.max) return `${ratio.min}×`
  return `${ratio.min}–${ratio.max}×`
}

/**
 * Replace model placeholder in endpoint path
 */
export function replaceModelInPath(path: string, modelName: string): string {
  return path.replaceAll('{model}', modelName)
}

/**
 * Check if model is token-based pricing
 */
export function isTokenBasedModel(model: PricingModel): boolean {
  return model.quota_type === QUOTA_TYPE_VALUES.TOKEN
}
