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
 * Popup width strategies for the group combobox.
 *
 * "anchored" matches the trigger width and is right for full-width fields.
 * "contentAdaptive" keeps at least the trigger width but grows with the option
 * rows (group name plus ratio badge) and never exceeds the available viewport
 * width, so narrow compact triggers do not clip the group names.
 */
export const GROUP_COMBOBOX_POPUP_WIDTH_CLASSES = {
  anchored: 'w-[var(--anchor-width)]',
  contentAdaptive:
    'w-max min-w-[var(--anchor-width)] max-w-[var(--available-width)]',
} as const
