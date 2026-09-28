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
import dayjs from 'dayjs'
import { RefreshCw } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import { CacheMetricsPopover } from './components/cache-metrics-popover'
import { GroupCardGrid } from './components/group-card-grid'
import { ModelBreakdownDialog } from './components/model-breakdown-dialog'
import { ReferenceCacheRatePopover } from './components/reference-cache-rate-popover'
import { SortSelect } from './components/sort-select'
import { StatusFilterChips } from './components/status-filter-chips'
import { TimeRangeTabs } from './components/time-range-tabs'
import { usePerformancePage } from './hooks/use-performance-page'

const CARD_SKELETON_KEYS = ['one', 'two', 'three', 'four', 'five', 'six']

export function Performance() {
  const { t } = useTranslation()
  const page = usePerformancePage()
  const query = page.query

  let body: ReactNode
  if (query.isLoading) {
    body = (
      <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
        {CARD_SKELETON_KEYS.map((key) => (
          <Skeleton key={key} className='h-80 rounded-xl' />
        ))}
      </div>
    )
  } else if (query.isError) {
    body = (
      <div className='flex flex-col items-center gap-3 rounded-xl border border-dashed p-10 text-center'>
        <p className='text-muted-foreground text-sm'>
          {t('Failed to load performance data.')}
        </p>
        <Button variant='outline' size='sm' onClick={page.refresh}>
          {t('Retry')}
        </Button>
      </div>
    )
  } else if (page.sortedGroups.length === 0) {
    body = (
      <div className='text-muted-foreground rounded-xl border border-dashed p-10 text-center text-sm'>
        {t('You have no usable groups yet.')}
      </div>
    )
  } else {
    body = (
      <div
        className={cn(
          'flex flex-col gap-4 transition-opacity duration-300',
          query.isPlaceholderData && 'opacity-60'
        )}
      >
        <StatusFilterChips
          counts={page.statusCounts}
          active={page.activeFilter}
          shown={page.visibleGroups.length}
          total={page.sortedGroups.length}
          onToggle={page.toggleStatusFilter}
        />
        {page.sortKey === 'custom' ? (
          <p className='text-muted-foreground text-xs'>
            {t(
              'Use Move up and Move down to save your group order on this device. Filters keep hidden groups in place.'
            )}
          </p>
        ) : null}
        <GroupCardGrid
          groups={page.visibleGroups}
          pricingByGroup={page.pricingByGroup}
          window={page.availabilityWindow}
          sortKey={page.sortKey}
          customOrder={page.customOrder}
          allGroups={page.allGroupKeys}
          onMove={page.moveGroup}
          onOpenModels={page.setSelectedGroup}
        />
      </div>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Performance')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <div className='flex items-center gap-2'>
          <TimeRangeTabs value={page.hours} onChange={page.setHours} />
          <Button
            size='icon'
            variant='outline'
            aria-label={t('Refresh')}
            className='size-8'
            disabled={query.isFetching}
            onClick={page.refresh}
          >
            <RefreshCw
              className={cn(
                'size-4',
                query.isFetching && 'animate-spin motion-reduce:animate-none'
              )}
              aria-hidden
            />
          </Button>
        </div>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-7xl flex-col gap-4'>
          <div className='flex flex-wrap items-center justify-between gap-x-4 gap-y-2'>
            <p className='text-muted-foreground text-sm'>
              {t(
                'Live availability and latency for the groups your account can use.'
              )}
            </p>
            {query.dataUpdatedAt > 0 ? (
              <p className='text-muted-foreground/70 text-xs'>
                {t('Updated {{time}}', {
                  time: dayjs(query.dataUpdatedAt).format('HH:mm:ss'),
                })}
                {' \u00b7 '}
                {t('Auto-refreshes every minute')}
              </p>
            ) : null}
          </div>
          <div className='flex flex-wrap items-center justify-end gap-2'>
            <SortSelect value={page.sortKey} onChange={page.setSortKey} />
            <CacheMetricsPopover />
            <ReferenceCacheRatePopover
              value={page.referenceCacheRateInput}
              onChange={page.setReferenceCacheRateInput}
            />
          </div>
          {body}
          <ModelBreakdownDialog
            summary={page.selectedSummary}
            pricing={page.selectedPricing}
            window={page.availabilityWindow}
            hours={page.hours}
            onClose={() => page.setSelectedGroup(null)}
          />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
