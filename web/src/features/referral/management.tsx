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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { UserInviterDialog } from '@/features/users/components/user-inviter-dialog'
import { useDebounce } from '@/hooks'
import { formatQuota } from '@/lib/format'

import { getInviters } from './api'
import { InviterInviteesDialog } from './components/inviter-invitees-dialog'
import { ListPagination } from './components/list-pagination'
import { StatGrid } from './components/stat-grid'
import { REFERRAL_PAGE_SIZE } from './lib/format'
import type { ReferralInviter } from './types'

const SEARCH_DEBOUNCE_MS = 300

/**
 * Admin invite page: who invited how many users, how many of those counted,
 * and the manual "bind user to inviter" repair tool.
 */
export function ReferralManagement() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [bindOpen, setBindOpen] = useState(false)
  const [viewing, setViewing] = useState<ReferralInviter | null>(null)
  const debouncedKeyword = useDebounce(keyword.trim(), SEARCH_DEBOUNCE_MS)

  const invitersQuery = useQuery({
    queryKey: ['referral', 'inviters', debouncedKeyword, page],
    queryFn: () => getInviters(debouncedKeyword, page, REFERRAL_PAGE_SIZE),
    placeholderData: (previousData) => previousData,
  })

  const data = invitersQuery.data?.data
  const inviters = data?.items ?? []
  const overview = data?.overview

  const stats = [
    {
      label: t('Inviters'),
      value: String(overview?.inviters ?? 0),
      hint: t('Invited at least one user'),
    },
    { label: t('Invited users'), value: String(overview?.invitees ?? 0) },
    {
      label: t('Valid invitees'),
      value: String(overview?.valid_invitees ?? 0),
    },
    {
      label: t('Commission recorded'),
      value: formatQuota(overview?.reward_quota ?? 0),
    },
  ]

  let tableBody: React.ReactNode
  if (invitersQuery.isLoading) {
    tableBody = (
      <TableRow>
        <TableCell colSpan={5}>
          <Skeleton className='h-8 w-full' />
        </TableCell>
      </TableRow>
    )
  } else if (inviters.length === 0) {
    tableBody = (
      <TableRow>
        <TableCell
          colSpan={5}
          className='text-muted-foreground py-8 text-center'
        >
          {t('No inviters found')}
        </TableCell>
      </TableRow>
    )
  } else {
    tableBody = inviters.map((inviter) => (
      <TableRow key={inviter.id}>
        <TableCell>
          <span className='text-muted-foreground mr-1.5 font-mono text-xs'>
            #{inviter.id}
          </span>
          {inviter.username || t('Deleted user')}
        </TableCell>
        <TableCell className='text-right tabular-nums'>
          {inviter.registered_count}
        </TableCell>
        <TableCell className='text-right tabular-nums'>
          {inviter.valid_invitees}
        </TableCell>
        <TableCell className='text-right tabular-nums'>
          {inviter.history_quota > 0 ? formatQuota(inviter.history_quota) : '-'}
        </TableCell>
        <TableCell className='text-right'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => setViewing(inviter)}
          >
            {t('View invited users')}
          </Button>
        </TableCell>
      </TableRow>
    ))
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Invite Management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button onClick={() => setBindOpen(true)}>{t('Bind Inviter')}</Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='space-y-4'>
            <StatGrid items={stats} loading={invitersQuery.isLoading} />

            <Card data-card-hover='false'>
              <CardContent className='space-y-3'>
                <div className='relative max-w-xs'>
                  <Search
                    className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2'
                    aria-hidden='true'
                  />
                  <Input
                    aria-label={t('Search inviters')}
                    placeholder={t('Search by inviter ID or username')}
                    className='pl-8'
                    value={keyword}
                    onChange={(event) => {
                      setKeyword(event.target.value)
                      setPage(1)
                    }}
                  />
                </div>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Inviter')}</TableHead>
                      <TableHead className='text-right'>
                        {t('Invited')}
                      </TableHead>
                      <TableHead className='text-right'>
                        {t('Valid invitees')}
                      </TableHead>
                      <TableHead className='text-right'>
                        {t('Total Earned')}
                      </TableHead>
                      <TableHead />
                    </TableRow>
                  </TableHeader>
                  <TableBody>{tableBody}</TableBody>
                </Table>
                <ListPagination
                  page={page}
                  pageSize={REFERRAL_PAGE_SIZE}
                  total={data?.total ?? 0}
                  onPageChange={setPage}
                />
              </CardContent>
            </Card>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <UserInviterDialog
        open={bindOpen}
        onOpenChange={setBindOpen}
        userIds={[]}
        onSuccess={() =>
          queryClient.invalidateQueries({ queryKey: ['referral'] })
        }
      />
      <InviterInviteesDialog
        inviter={viewing}
        onOpenChange={(open) => {
          if (!open) setViewing(null)
        }}
      />
    </>
  )
}
