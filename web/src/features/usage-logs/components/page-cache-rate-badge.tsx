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
import { useTranslation } from 'react-i18next'

import { StatBadge } from './stat-badge'

/**
 * Cache hit rate computed from the rows on the current log page. A null rate
 * means the page carries no input tokens, so the value is unknown rather than
 * zero.
 */
export function PageCacheRateBadge(props: { rate: number | null }) {
  const { t } = useTranslation()

  return (
    <StatBadge
      label={t('Page Cache Rate')}
      value={props.rate === null ? '—' : `${props.rate.toFixed(1)}%`}
      accent='bg-emerald-500/70'
    />
  )
}
