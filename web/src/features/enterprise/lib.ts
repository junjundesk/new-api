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
/**
 * Enterprise identity helpers.
 *
 * An account is an enterprise account when the backend gate is enabled and its
 * cumulative paid recharge has reached the configured threshold. The flag is
 * computed server-side (`enterprise_unlocked`) and mirrored on the auth user.
 */

export const ENTERPRISE_PRESET = 'enterprise'

/**
 * localStorage key marking that the enterprise black-and-gold theme has already
 * been auto-applied for this browser, so re-entering the app does not fight the
 * user's explicit preset choice on every navigation.
 */
export const ENTERPRISE_AUTO_APPLIED_KEY = 'enterprise_theme_auto_applied'

export function isEnterpriseUnlocked(
  user: { enterprise_unlocked?: boolean } | null | undefined
): boolean {
  return user?.enterprise_unlocked === true
}
