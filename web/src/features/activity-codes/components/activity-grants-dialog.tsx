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
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { formatQuota, formatTimestampToDate } from '@/lib/format'

import { getActivityCodeGrants, revokeActivityGrant } from '../api'
import {
  ACTIVITY_GRANT_STATUS,
  ERROR_MESSAGES,
  SUCCESS_MESSAGES,
  getActivityGrantStatusLabelKey,
} from '../constants'
import { useActivityCodes } from './activity-codes-provider'

export function ActivityGrantsDialog() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { open, setOpen, currentRow } = useActivityCodes()
  const activityCodeId = currentRow?.id ?? 0

  const { data, isLoading } = useQuery({
    queryKey: ['activity-code-grants', activityCodeId],
    enabled: open === 'grants' && activityCodeId > 0,
    queryFn: async () => {
      const result = await getActivityCodeGrants(activityCodeId)
      if (!result.success) {
        toast.error(result.message || t(ERROR_MESSAGES.GRANTS_FAILED))
        return { items: [], total: 0 }
      }
      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
  })

  const grants = data?.items || []

  const handleRevoke = async (grantId: number) => {
    const result = await revokeActivityGrant(grantId)
    if (!result.success) {
      toast.error(result.message || t(ERROR_MESSAGES.REVOKE_FAILED))
      return
    }
    toast.success(t(SUCCESS_MESSAGES.GRANT_REVOKED))
    await queryClient.invalidateQueries({
      queryKey: ['activity-code-grants', activityCodeId],
    })
  }

  let expireText = t('Permanent')
  return (
    <Dialog open={open === 'grants'} onOpenChange={(nextOpen) => !nextOpen && setOpen(null)}>
      <DialogContent className='sm:max-w-[640px]'>
        <DialogHeader>
          <DialogTitle>
            {t('Activity Grants')}
            {currentRow ? ` · ${currentRow.code}` : ''}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Redeemed grants for this activity code. Revoking one takes effect immediately.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className='max-h-[60vh] space-y-2 overflow-y-auto pr-1'>
          {isLoading && (
            <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
          )}
          {!isLoading && grants.length === 0 && (
            <p className='text-muted-foreground text-sm'>
              {t('No grants yet.')}
            </p>
          )}
          {grants.map((grant) => {
            expireText =
              grant.expire_time > 0
                ? formatTimestampToDate(grant.expire_time)
                : t('Permanent')
            return (
              <div
                key={grant.id}
                className='flex items-center justify-between gap-3 rounded-lg border p-3'
              >
                <div className='min-w-0 space-y-1'>
                  <div className='flex items-center gap-2'>
                    <span className='text-sm font-medium'>
                      {t('User')} #{grant.user_id}
                    </span>
                    <StatusBadge
                      label={t(getActivityGrantStatusLabelKey(grant.status))}
                      variant={
                        grant.status === ACTIVITY_GRANT_STATUS.ACTIVE
                          ? 'success'
                          : 'neutral'
                      }
                      copyable={false}
                    />
                  </div>
                  <p className='text-muted-foreground text-xs'>
                    {grant.group_name} · {t('Expires at')}: {expireText} ·{' '}
                    {t('Paid')}: {formatQuota(grant.price_quota)}
                  </p>
                </div>
                <Button
                  size='sm'
                  variant='outline'
                  disabled={grant.status !== ACTIVITY_GRANT_STATUS.ACTIVE}
                  onClick={() => void handleRevoke(grant.id)}
                >
                  {t('Revoke')}
                </Button>
              </div>
            )
          })}
        </div>
      </DialogContent>
    </Dialog>
  )
}
