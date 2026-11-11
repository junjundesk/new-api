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
import type { ReferralTier } from '@/features/wallet/types'

export const REFERRAL_PAGE_SIZE = 10

/**
 * Render a basis-point rate as a percentage without losing the sub-percent
 * precision the backend stores (150 bps -> 1.5%).
 */
export function formatRate(rateBps: number): string {
  if (!rateBps) {
    return '0%'
  }
  if (rateBps % 100 === 0) {
    return `${rateBps / 100}%`
  }
  return `${(rateBps / 100).toFixed(2).replace(/0+$/, '').replace(/\.$/, '')}%`
}

/**
 * Number of pages needed for a total, never less than one so an empty list
 * still renders "page 1 of 1".
 */
export function pageCount(total: number, pageSize: number): number {
  if (total <= 0 || pageSize <= 0) {
    return 1
  }
  return Math.ceil(total / pageSize)
}

/**
 * Index of the tier that applies to a valid invitee count, or -1 when the
 * inviter has not reached the first tier yet. Tiers are sorted ascending.
 */
export function currentTierIndex(
  tiers: ReferralTier[],
  validInvitees: number
): number {
  let index = -1
  tiers.forEach((tier, i) => {
    if (validInvitees >= tier.min_invitees) {
      index = i
    }
  })
  return index
}
