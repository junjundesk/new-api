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
import { Ticket } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { formatModelRatioOverrides, formatQuota } from '@/lib/format'

import { getActivityGrants } from '../api'
import {
  describeActivityRatio,
  describeExpireTime,
} from '../lib/activity-format'

function grantStatusLabelKey(status: string): string {
  if (status === 'active') return 'Active'
  if (status === 'revoked') return 'Revoked'
  return 'Expired'
}

export function ActivityGrantsCard() {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ['activity-grants'],
    queryFn: async () => {
      const result = await getActivityGrants()
      if (!result.success) {
        return { grants: [], active: [] }
      }
      return result.data ?? { grants: [], active: [] }
    },
  })

  const grants = data?.grants ?? []

  return (
    <Card>
      <CardHeader>
        <div className='flex items-center gap-2'>
          <IconBadge tone='info' size='sm'>
            <Ticket />
          </IconBadge>
          <div className='space-y-0.5'>
            <p className='text-sm font-medium'>{t('My Activity Grants')}</p>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Set a token group to the granted group to use it while the grant is active.'
              )}
            </p>
          </div>
        </div>
      </CardHeader>
      <CardContent className='space-y-2'>
        {isLoading && (
          <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
        )}
        {!isLoading && grants.length === 0 && (
          <p className='text-muted-foreground text-sm'>
            {t('No activity grants yet.')}
          </p>
        )}
        {grants.map((grant) => {
          // 指定模型时直接列出具体模型 ID 与倍率；未指定模型则不加多余信息。
          const listedModels = formatModelRatioOverrides(
            grant.model_ratio_overrides
          )
          const summary = [
            grant.code_name || grant.code,
            describeActivityRatio(t, grant.ratio_mode, grant.ratio_override),
            listedModels ? `${t('Models (ratio)')}: ${listedModels}` : '',
            `${t('Expires at')}: ${describeExpireTime(t, grant.expire_time)}`,
            grant.price_quota > 0
              ? `${t('Paid')}: ${formatQuota(grant.price_quota)}`
              : '',
          ]
            .filter(Boolean)
            .join(' · ')

          return (
            <div
              key={grant.id}
              className='flex items-center justify-between gap-3 rounded-lg border p-3'
            >
              <div className='min-w-0 space-y-1'>
                <div className='flex items-center gap-2'>
                  <span className='text-sm font-medium'>
                    {grant.group_name}
                  </span>
                  <StatusBadge
                    label={t(grantStatusLabelKey(grant.status))}
                    variant={grant.status === 'active' ? 'success' : 'neutral'}
                    copyable={false}
                  />
                </div>
                <p className='text-muted-foreground text-xs break-all'>
                  {summary}
                </p>
              </div>
            </div>
          )
        })}
      </CardContent>
    </Card>
  )
}
