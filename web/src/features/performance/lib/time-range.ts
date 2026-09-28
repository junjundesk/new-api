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
export type PerfTimeRangeHours = 1 | 24 | 168

export const PERF_TIME_RANGES: {
  hours: PerfTimeRangeHours
  label: string
  labelKey: string
}[] = [
  { hours: 1, label: '1h', labelKey: 'Last hour' },
  { hours: 24, label: '24h', labelKey: 'Last 24 hours' },
  { hours: 168, label: '7d', labelKey: 'Last 7 days' },
]

export const DEFAULT_PERF_TIME_RANGE: PerfTimeRangeHours = 24

/** i18n key of the time window, used by the model breakdown dialog. */
export function getTimeRangeLabelKey(hours: number): string {
  const range = PERF_TIME_RANGES.find((item) => item.hours === hours)
  return range ? range.labelKey : 'Last 24 hours'
}
