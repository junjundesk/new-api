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

import { CopyButton } from '@/components/copy-button'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getReferralCharges } from '@/features/wallet/api'
import { TransferDialog } from '@/features/wallet/components/dialogs/transfer-dialog'
import { useAffiliate } from '@/features/wallet/hooks'
import { formatQuota } from '@/lib/format'

import { getMyInvitees } from './api'
import { ChargesTable } from './components/charges-table'
import { InviteesTable } from './components/invitees-table'
import { ListPagination } from './components/list-pagination'
import { StatGrid } from './components/stat-grid'
import { TierCard } from './components/tier-card'
import { formatRate, REFERRAL_PAGE_SIZE } from './lib/format'

/**
 * User-facing invite page: the invite link, how many users were invited and
 * how many of them counted, the rewards earned, and the per-invitee detail.
 */
export function Referral() {
  const { t } = useTranslation()
  const affiliate = useAffiliate()
  const summary = affiliate.summary
  const [transferOpen, setTransferOpen] = useState(false)
  const [inviteePage, setInviteePage] = useState(1)
  const [chargePage, setChargePage] = useState(1)

  const inviteesQuery = useQuery({
    queryKey: ['referral', 'invitees', inviteePage],
    queryFn: () => getMyInvitees(inviteePage, REFERRAL_PAGE_SIZE),
    placeholderData: (previousData) => previousData,
  })
  const chargesQuery = useQuery({
    queryKey: ['referral', 'charges', chargePage],
    queryFn: () => getReferralCharges(chargePage, REFERRAL_PAGE_SIZE),
    placeholderData: (previousData) => previousData,
  })

  const programEnabled = summary?.enabled !== false
  // The summary carries no counts while the program is disabled, but the
  // invite relationship still exists, so the list total is the fallback.
  const invitedCount =
    summary?.registered_count ?? inviteesQuery.data?.data?.total ?? 0
  const claimable = summary?.claimable_quota ?? 0
  const transferBlocked = summary?.compliance_confirmed === false

  const handleTransfer = async (amount: number) => {
    const success = await affiliate.transferQuota(amount)
    if (success) {
      await affiliate.refetch()
    }
    return success
  }

  const stats = [
    {
      label: t('Invited'),
      value: String(invitedCount),
      hint: t('Registered through your link'),
    },
    {
      label: t('Valid invitees'),
      value: String(summary?.valid_invitees ?? 0),
      hint: summary?.count_redeem_as_valid
        ? t('Topped up or redeemed a code')
        : t('Topped up'),
    },
    { label: t('Pending'), value: formatQuota(summary?.pending_quota ?? 0) },
    { label: t('Transferable'), value: formatQuota(claimable) },
    {
      label: t('Total Earned'),
      value: formatQuota(summary?.history_quota ?? 0),
    },
  ]

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Invite Rewards')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            onClick={() => setTransferOpen(true)}
            disabled={claimable <= 0 || transferBlocked}
          >
            {t('Transfer to Balance')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='space-y-4'>
            {programEnabled ? null : (
              <p className='text-muted-foreground text-sm'>
                {t('The referral program is currently disabled.')}
              </p>
            )}
            {transferBlocked ? (
              <p className='text-muted-foreground text-sm'>
                {t(
                  'Referral reward transfer is disabled until the administrator confirms compliance terms.'
                )}
              </p>
            ) : null}

            <StatGrid items={stats} loading={affiliate.loading} />

            <Card data-card-hover='false'>
              <CardHeader>
                <p className='text-sm font-medium'>{t('Invite link')}</p>
              </CardHeader>
              <CardContent className='space-y-2'>
                <div className='flex items-center gap-2'>
                  <Input
                    value={affiliate.affiliateLink}
                    readOnly
                    aria-label={t('Invite link')}
                    className='h-9 min-w-0 flex-1 font-mono text-xs'
                  />
                  <CopyButton
                    value={affiliate.affiliateLink}
                    variant='outline'
                    className='size-9 shrink-0'
                    iconClassName='size-4'
                    tooltip={t('Copy referral link')}
                    aria-label={t('Copy referral link')}
                  />
                </div>
                {summary?.invitee_reward_enabled ? (
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Invitees receive {{rate}} of their first top-up as bonus credit',
                      { rate: formatRate(summary.invitee_rate_bps) }
                    )}
                  </p>
                ) : null}
              </CardContent>
            </Card>

            {summary?.inviter_reward_enabled ? (
              <TierCard summary={summary} />
            ) : null}

            <Card data-card-hover='false'>
              <CardContent>
                <Tabs defaultValue='invitees'>
                  <TabsList>
                    <TabsTrigger value='invitees'>
                      {t('Invited users')}
                    </TabsTrigger>
                    <TabsTrigger value='charges'>
                      {t('Reward history')}
                    </TabsTrigger>
                  </TabsList>
                  <TabsContent value='invitees'>
                    <InviteesTable
                      invitees={inviteesQuery.data?.data?.items ?? []}
                      loading={inviteesQuery.isLoading}
                    />
                    <ListPagination
                      page={inviteePage}
                      pageSize={REFERRAL_PAGE_SIZE}
                      total={inviteesQuery.data?.data?.total ?? 0}
                      onPageChange={setInviteePage}
                    />
                  </TabsContent>
                  <TabsContent value='charges'>
                    <ChargesTable
                      charges={chargesQuery.data?.data?.items ?? []}
                      loading={chargesQuery.isLoading}
                    />
                    <ListPagination
                      page={chargePage}
                      pageSize={REFERRAL_PAGE_SIZE}
                      total={chargesQuery.data?.data?.total ?? 0}
                      onPageChange={setChargePage}
                    />
                  </TabsContent>
                </Tabs>
              </CardContent>
            </Card>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <TransferDialog
        open={transferOpen}
        onOpenChange={setTransferOpen}
        onConfirm={handleTransfer}
        availableQuota={claimable}
        transferring={affiliate.transferring}
      />
    </>
  )
}
