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

export const GROUP_FILTER_ALL_VALUE = '__all__'

export interface GroupFilterOption {
  value: string
  label: string
}

/**
 * Builds the options of the log page group filter. The "all" entry always comes
 * first so the filter can be cleared, followed by the model groups (the group
 * names of the pricing table, which is what log rows record).
 */
export function buildGroupFilterOptions(
  groups: readonly string[] | Record<string, unknown> | undefined,
  allLabel: string
): GroupFilterOption[] {
  const names = (Array.isArray(groups) ? [...groups] : Object.keys(groups ?? {}))
    .filter((name) => name.length > 0)
    .sort()
  return [
    { value: GROUP_FILTER_ALL_VALUE, label: allLabel },
    ...names.map((name) => ({ value: name, label: name })),
  ]
}
