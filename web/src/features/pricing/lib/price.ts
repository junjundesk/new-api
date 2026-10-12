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
import { formatCurrencyFromUSD } from '@/lib/currency'

import { QUOTA_TYPE_VALUES, TOKEN_UNIT_DIVISORS } from '../constants'
import type { BillingRatio, PricingModel, TokenUnit, PriceType } from '../types'
import { getGroupBillingRatio, getDisplayBillingRatio } from './model-helpers'

// ----------------------------------------------------------------------------
// Price Calculation Utilities
// ----------------------------------------------------------------------------

/**
 * Strip trailing zeros from formatted price string while preserving currency symbols
 */
export function stripTrailingZeros(formatted: string): string {
  if (formatted.includes('–')) {
    return formatted.split('–').map(stripTrailingZeros).join('–')
  }
  // Match currency symbol at start, number, and potential 'k' suffix
  const match = formatted.match(/^([^\d-]*)([-\d,]+\.?\d*)(k?)$/)
  if (!match) return formatted

  const [, symbol, number, suffix] = match

  // Remove commas for processing
  const cleanNumber = number.replaceAll(',', '')

  // Convert to number and back to remove trailing zeros
  const parsed = Number.parseFloat(cleanNumber)
  if (Number.isNaN(parsed)) return formatted

  // Convert to string, which automatically removes trailing zeros
  let result = parsed.toString()

  // If the result is in scientific notation, format it properly
  if (result.includes('e')) {
    result = parsed.toFixed(20).replace(/\.?0+$/, '')
  }

  return `${symbol}${result}${suffix}`
}

/**
 * A price rendered together with its pre-ratio original.
 *
 * `original` stays undefined when the effective price already is the original
 * one, so callers render a single price instead of a strikethrough that crosses
 * out the same number it sits next to.
 */
export type PricePair = {
  effective: string
  original?: string
}

/**
 * Pair a ratio-applied price with its 1x original price.
 *
 * Ratios that do not change the rendered amount (for example 1.0000001x) count
 * as no discount, because the formatted prices are what users compare.
 */
export function withOriginalPrice(
  effective: string,
  original: string
): PricePair {
  return effective === original ? { effective } : { effective, original }
}

/**
 * Calculate token price in USD.
 *
 * Returns NaN when the required ratio field is missing/null so callers can
 * skip rendering that price type.
 */
function calculateTokenPrice(
  model: PricingModel,
  type: PriceType,
  ratio: number
): number {
  const base = model.model_ratio * 2 * ratio

  switch (type) {
    case 'input':
      return base
    case 'output':
      return base * model.completion_ratio
    case 'cache':
      return hasRatio(model.cache_ratio)
        ? base * Number(model.cache_ratio)
        : Number.NaN
    case 'create_cache':
      return hasRatio(model.create_cache_ratio)
        ? base * Number(model.create_cache_ratio)
        : Number.NaN
    case 'image':
      return hasRatio(model.image_ratio)
        ? base * Number(model.image_ratio)
        : Number.NaN
    case 'audio_input':
      return hasRatio(model.audio_ratio)
        ? base * Number(model.audio_ratio)
        : Number.NaN
    case 'audio_output':
      return hasRatio(model.audio_ratio) &&
        hasRatio(model.audio_completion_ratio)
        ? base *
            Number(model.audio_ratio) *
            Number(model.audio_completion_ratio)
        : Number.NaN
  }
}

function hasRatio(value: number | null | undefined): boolean {
  return value !== undefined && value !== null && Number.isFinite(Number(value))
}

/**
 * Apply recharge rate to price
 *
 * priceRate represents how much users need to recharge (in the display currency)
 * to get 1 USD credit. usdExchangeRate is the real exchange rate.
 *
 * The returned value will be formatted by formatCurrencyFromUSD, which will
 * multiply by the display currency's exchange rate.
 *
 * Examples:
 *
 * 1. Display currency = USD:
 *    - Model: 1 USD
 *    - priceRate = 0.5 (recharge $0.5 to get $1 credit)
 *    - usdExchangeRate = 1
 *    - Return: 1 × 0.5 / 1 = 0.5
 *    - formatCurrencyFromUSD(0.5) → $0.5 ✓
 *
 * 2. Display currency = CNY:
 *    - Model: 1 USD
 *    - priceRate = 4 (recharge ¥4 to get $1 credit)
 *    - usdExchangeRate = 7 (real rate: 1 USD = ¥7)
 *    - Return: 1 × 4 / 7 = 0.571
 *    - formatCurrencyFromUSD(0.571) → 0.571 × 7 = ¥4 ✓
 *    - Normal price: ¥7, Recharge price: ¥4 (cheaper!)
 */
function applyRechargeRate(
  price: number,
  showWithRecharge: boolean,
  priceRate: number,
  usdExchangeRate: number
): number {
  if (!showWithRecharge) return price
  return (price * priceRate) / usdExchangeRate
}

function formatEffectivePrice(
  basePrice: number,
  ratio: BillingRatio,
  showWithRecharge: boolean,
  priceRate: number,
  usdExchangeRate: number,
  digitsSmall: number
): string {
  const prices = [ratio.min, ratio.max].map((multiplier) =>
    formatCurrencyFromUSD(
      applyRechargeRate(
        basePrice * multiplier,
        showWithRecharge,
        priceRate,
        usdExchangeRate
      ),
      {
        digitsLarge: 4,
        digitsSmall,
        abbreviate: false,
      }
    )
  )
  return ratio.min === ratio.max ? prices[0] : prices.join('–')
}

/**
 * Format token-based price for display
 */
export function formatPrice(
  model: PricingModel,
  type: PriceType,
  tokenUnit: TokenUnit,
  showWithRecharge = false,
  priceRate = 1,
  usdExchangeRate = 1,
  selectedGroup?: string
): string {
  if (model.quota_type === QUOTA_TYPE_VALUES.REQUEST) {
    return '-'
  }

  return formatEffectivePrice(
    calculateTokenPrice(model, type, 1) / TOKEN_UNIT_DIVISORS[tokenUnit],
    getDisplayBillingRatio(model, selectedGroup),
    showWithRecharge,
    priceRate,
    usdExchangeRate,
    6
  )
}

/**
 * Format price for a specific group (token-based)
 */
export function formatGroupPrice(
  model: PricingModel,
  group: string,
  type: PriceType,
  tokenUnit: TokenUnit,
  showWithRecharge = false,
  priceRate = 1,
  usdExchangeRate = 1,
  groupRatio: Record<string, number>
): string {
  if (model.quota_type === QUOTA_TYPE_VALUES.REQUEST) {
    return '-'
  }

  return formatEffectivePrice(
    calculateTokenPrice(model, type, 1) / TOKEN_UNIT_DIVISORS[tokenUnit],
    getGroupBillingRatio(model, group, groupRatio),
    showWithRecharge,
    priceRate,
    usdExchangeRate,
    6
  )
}

/**
 * Format fixed price for pay-per-request models (with specific group)
 */
export function formatFixedPrice(
  model: PricingModel,
  group: string,
  showWithRecharge = false,
  priceRate = 1,
  usdExchangeRate = 1,
  groupRatio: Record<string, number>
): string {
  if (model.quota_type !== QUOTA_TYPE_VALUES.REQUEST) {
    return '-'
  }

  return formatEffectivePrice(
    model.model_price || 0,
    getGroupBillingRatio(model, group, groupRatio),
    showWithRecharge,
    priceRate,
    usdExchangeRate,
    4
  )
}

/**
 * Format fixed price for pay-per-request models (minimum price from all groups)
 */
export function formatRequestPrice(
  model: PricingModel,
  showWithRecharge = false,
  priceRate = 1,
  usdExchangeRate = 1,
  selectedGroup?: string
): string {
  if (model.quota_type !== QUOTA_TYPE_VALUES.REQUEST) {
    return '-'
  }

  return formatEffectivePrice(
    model.model_price || 0,
    getDisplayBillingRatio(model, selectedGroup),
    showWithRecharge,
    priceRate,
    usdExchangeRate,
    4
  )
}

/**
 * Format token-based price at ratio=1 (base price before any group/channel multiplier).
 */
export function formatBasePrice(
  model: PricingModel,
  type: PriceType,
  tokenUnit: TokenUnit,
  showWithRecharge = false,
  priceRate = 1,
  usdExchangeRate = 1
): string {
  if (model.quota_type === QUOTA_TYPE_VALUES.REQUEST) {
    return '-'
  }
  return formatEffectivePrice(
    calculateTokenPrice(model, type, 1) / TOKEN_UNIT_DIVISORS[tokenUnit],
    { min: 1, max: 1, source: 'group' } as BillingRatio,
    showWithRecharge,
    priceRate,
    usdExchangeRate,
    6
  )
}

/**
 * Format per-request price at ratio=1 (base price before any group/channel multiplier).
 */
export function formatBaseRequestPrice(
  model: PricingModel,
  showWithRecharge = false,
  priceRate = 1,
  usdExchangeRate = 1
): string {
  if (model.quota_type !== QUOTA_TYPE_VALUES.REQUEST) {
    return '-'
  }
  return formatEffectivePrice(
    model.model_price || 0,
    { min: 1, max: 1, source: 'group' } as BillingRatio,
    showWithRecharge,
    priceRate,
    usdExchangeRate,
    4
  )
}
