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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'

import { getInviterInvitees } from '../api'
import { REFERRAL_PAGE_SIZE } from '../lib/format'
import type { ReferralInviter } from '../types'
import { InviteesTable } from './invitees-table'
import { ListPagination } from './list-pagination'

interface InviterInviteesDialogProps {
  inviter: ReferralInviter | null
  onOpenChange: (open: boolean) => void
}

/**
 * Admin dialog listing every user a given inviter invited.
 */
export function InviterInviteesDialog(props: InviterInviteesDialogProps) {
  const { t } = useTranslation()
  const inviter = props.inviter
  const inviterId = inviter?.id ?? 0
  const [page, setPage] = useState(1)

  // A different inviter always starts on its own first page.
  useEffect(() => {
    setPage(1)
  }, [inviterId])

  const inviteesQuery = useQuery({
    queryKey: ['referral', 'inviter-invitees', inviterId, page],
    queryFn: () => getInviterInvitees(inviterId, page, REFERRAL_PAGE_SIZE),
    enabled: inviterId > 0,
    placeholderData: (previousData) => previousData,
  })

  return (
    <Dialog
      open={inviter !== null}
      onOpenChange={props.onOpenChange}
      title={t('Users invited by {{username}}', {
        username: inviter?.username ?? '',
      })}
      description={t('Invited: {{registered}} · Valid: {{valid}}', {
        registered: inviter?.registered_count ?? 0,
        valid: inviter?.valid_invitees ?? 0,
      })}
      contentHeight='auto'
      footer={
        <Button variant='outline' onClick={() => props.onOpenChange(false)}>
          {t('Close')}
        </Button>
      }
    >
      <InviteesTable
        invitees={inviteesQuery.data?.data?.items ?? []}
        loading={inviteesQuery.isLoading}
        showId
      />
      <ListPagination
        page={page}
        pageSize={REFERRAL_PAGE_SIZE}
        total={inviteesQuery.data?.data?.total ?? 0}
        onPageChange={setPage}
      />
    </Dialog>
  )
}
