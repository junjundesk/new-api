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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatQuotaWithCurrency } from '@/lib/currency'

import { getRevenueReport, type RevenueRange, type RevenueTotals } from './api'
import { CostGroupDialog } from './cost-group-dialog'

const amount = (quota: string) =>
  formatQuotaWithCurrency(Number(quota), {
    digitsLarge: 2,
    digitsSmall: 6,
    abbreviate: false,
  })

export function RevenuePage() {
  const { t } = useTranslation()
  const [range, setRange] = useState<RevenueRange>('today')
  const [editing, setEditing] = useState(false)
  const report = useQuery({
    queryKey: ['revenue-report', range],
    queryFn: () => getRevenueReport(range),
    refetchInterval: range === 'today' ? 30_000 : false,
    refetchIntervalInBackground: false,
  })
  const data = report.data
  const ranges: [RevenueRange, string][] = [
    ['today', t('Today (live)')],
    ['yesterday', t('Yesterday')],
    ['7d', t('Last 7 days')],
    ['30d', t('Last 30 days')],
    ['all', t('All time')],
  ]
  const metrics: [keyof RevenueTotals, string][] = [
    ['total', t('Net usage price')],
    ['cost', t('Estimated cost (priced usage)')],
    ['profit', t('Estimated profit (priced usage)')],
    ['unpriced', t('Usage without a cost rate')],
    ['online', t('Online top-up credit')],
    ['redeemed', t('Redeemed wallet credit')],
    ['admin', t('Admin-added balance')],
    ['credited', t('Total wallet inflows')],
  ]
  const priced = data?.items.some((item) => item.cost_ratio !== null)
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Revenue report')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          disabled={report.isFetching}
          onClick={() => void report.refetch()}
        >
          {t('Refresh')}
        </Button>
        <Button onClick={() => setEditing(true)}>
          {t('Bind cost groups')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-5'>
          <div
            className='flex flex-wrap gap-2'
            role='group'
            aria-label={t('Time range')}
          >
            {ranges.map(([key, label]) => (
              <Button
                key={key}
                variant={range === key ? 'default' : 'outline'}
                aria-pressed={range === key}
                onClick={() => setRange(key)}
              >
                {label}
              </Button>
            ))}
          </div>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Calendar days use UTC+8. Today refreshes every 30 seconds. Wallet inflows are credited balances, not cash receipts or additional usage revenue.'
            )}
          </p>
          {report.isPending && <p role='status'>{t('Loading...')}</p>}
          {report.isError && (
            <p role='alert'>{t('Unable to load the revenue report.')}</p>
          )}
          {data && !report.isError && (
            <>
              <dl className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
                {metrics.map(([key, label]) => (
                  <div key={key} className='bg-card rounded-lg border p-3'>
                    <dt className='text-muted-foreground text-sm'>{label}</dt>
                    <dd className='mt-1 font-semibold tabular-nums'>
                      {!priced && (key === 'cost' || key === 'profit')
                        ? '—'
                        : amount(data.summary[key])}
                    </dd>
                  </div>
                ))}
              </dl>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'Net usage price is billed quota less refunds. Cost is net usage × the current cost multiplier. Unbound usage has no estimated cost or profit. Changing rates recalculates estimates, not user charges.'
                )}
              </p>
              <h2 className='font-semibold'>{t('Daily totals')}</h2>
              {data.days.length === 0 ? (
                <p>{t('No usage or wallet credits in this period.')}</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      {[t('Date'), ...metrics.map(([, label]) => label)].map(
                        (label) => (
                          <TableHead key={label}>{label}</TableHead>
                        )
                      )}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {data.days.map((d) => (
                      <TableRow key={d.date}>
                        <TableCell>{d.date}</TableCell>
                        {metrics.map(([key]) => (
                          <TableCell key={key} className='tabular-nums'>
                            {amount(d[key])}
                          </TableCell>
                        ))}
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
              <h2 className='font-semibold'>{t('Usage by group and model')}</h2>
              {data.items.length === 0 ? (
                <p>{t('No usage records in this period.')}</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      {[
                        t('Group'),
                        t('Model'),
                        t('Cost multiplier'),
                        ...metrics.slice(0, 3).map(([, label]) => label),
                      ].map((label) => (
                        <TableHead key={label}>{label}</TableHead>
                      ))}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {data.items.map((item) => (
                      <TableRow key={JSON.stringify([item.group, item.model])}>
                        <TableCell>{item.group || '—'}</TableCell>
                        <TableCell>{item.model || '—'}</TableCell>
                        <TableCell>
                          {item.cost_ratio ?? t('Not configured')}
                        </TableCell>
                        <TableCell>{amount(item.total)}</TableCell>
                        <TableCell>
                          {item.cost_ratio === null ? '—' : amount(item.cost)}
                        </TableCell>
                        <TableCell>
                          {item.cost_ratio === null ? '—' : amount(item.profit)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </>
          )}
        </div>
        {editing && (
          <CostGroupDialog
            items={data?.items ?? []}
            onClose={() => setEditing(false)}
          />
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
