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
/** Supported values of the perf-metrics lang query parameter. */
const PERF_METRICS_LANGUAGES = new Set(['zh', 'en'])

/**
 * Maps an i18n language tag onto the lang value the perf-metrics API expects.
 * Every zh variant (zh-CN / zh-TW / zh-Hant* / zhtw) resolves to zh, a known
 * primary subtag to itself, and anything unknown to en.
 */
export function resolvePerfMetricsLang(language: string | undefined): string {
  if (!language) return 'en'
  const normalized = language.trim().replaceAll('_', '-').toLowerCase()
  if (normalized.startsWith('zh')) return 'zh'
  const primary = normalized.split('-')[0]
  if (PERF_METRICS_LANGUAGES.has(primary)) return primary
  if (PERF_METRICS_LANGUAGES.has(normalized)) return normalized
  return 'en'
}
