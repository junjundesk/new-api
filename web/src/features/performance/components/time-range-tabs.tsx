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

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { PERF_TIME_RANGES, type PerfTimeRangeHours } from '../lib/time-range'

export type TimeRangeTabsProps = {
  value: PerfTimeRangeHours
  onChange: (hours: PerfTimeRangeHours) => void
}

export function TimeRangeTabs(props: TimeRangeTabsProps) {
  const { t } = useTranslation()
  return (
    <Tabs
      value={String(props.value)}
      onValueChange={(value) => {
        const next = PERF_TIME_RANGES.find(
          (range) => String(range.hours) === value
        )
        if (next) props.onChange(next.hours)
      }}
    >
      <TabsList variant='line' aria-label={t('Time range')}>
        {PERF_TIME_RANGES.map((range) => (
          <TabsTrigger
            key={range.hours}
            value={String(range.hours)}
            aria-label={t(range.labelKey)}
            className='px-2 py-0.5 text-xs'
          >
            {range.label}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  )
}
