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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Gift } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { formatQuota } from '@/lib/format'

import { getRedemptionCategories } from './api'
import { RedemptionCategoryPanel } from './components/redemption-category-panel'
import { RedemptionsDialogs } from './components/redemptions-dialogs'
import { RedemptionsPrimaryButtons } from './components/redemptions-primary-buttons'
import { RedemptionsProvider } from './components/redemptions-provider'
import { RedemptionsTable } from './components/redemptions-table'

const route = getRouteApi('/_authenticated/redemption-codes/')

export function Redemptions() {
  const { t } = useTranslation()
  const navigate = route.useNavigate()
  const { category_id: categoryId } = route.useSearch()
  const categories = useQuery({
    queryKey: ['redemption-categories'],
    queryFn: getRedemptionCategories,
  })
  const selectCategory = (id?: number) => {
    void navigate({ search: (prev) => ({ ...prev, category_id: id, page: 1 }) })
  }
  const current = categories.data?.find(
    (category) => category.id === categoryId
  )
  let title = t('All codes')
  if (categoryId === 0) title = t('Uncategorized')
  if (categoryId) title = current?.name ?? t('Category')

  return (
    <RedemptionsProvider categories={categories.data} categoryId={categoryId}>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Redemption Codes')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <RedemptionsPrimaryButtons />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-3 lg:flex-row lg:gap-5'>
            <RedemptionCategoryPanel
              onSelect={selectCategory}
              isLoading={categories.isLoading}
              isError={categories.isError}
              retry={() => void categories.refetch()}
            />
            <section
              aria-label={title}
              className='flex min-h-0 min-w-0 flex-1 flex-col gap-2.5'
            >
              <div className='hidden min-h-8 items-center justify-between gap-3 lg:flex'>
                <h3 className='truncate text-base font-semibold'>{title}</h3>
                {current && (
                  <div className='flex shrink-0 items-center gap-1.5 rounded-md bg-amber-500/10 px-2.5 py-1 text-xs text-amber-800 dark:text-amber-300'>
                    <Gift aria-hidden className='size-3.5' />
                    {t('Bonus per code')}
                    <strong className='tabular-nums'>
                      {formatQuota(
                        Math.round(
                          (current.quota * current.bonus_rate_bps) / 10000
                        )
                      )}
                    </strong>
                  </div>
                )}
              </div>
              <div className='min-h-0 flex-1'>
                <RedemptionsTable key={categoryId ?? 'all'} />
              </div>
            </section>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <RedemptionsDialogs />
    </RedemptionsProvider>
  )
}
