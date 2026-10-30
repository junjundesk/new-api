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
  describeTokenUsage,
} from '../lib/activity-format'
import type { ActivityGrant } from '../types'

function grantStatusLabelKey(status: string): string {
  if (status === 'active') return 'Active'
  if (status === 'revoked') return 'Revoked'
  return 'Expired'
}

type ActivityGrantRowProps = {
  grant: ActivityGrant
  // 同一次兑换（redeem_batch）包含多条权益时，已支付金额只展示一次。
  showPaid: boolean
}

function ActivityGrantRow(props: ActivityGrantRowProps) {
  const { t, i18n } = useTranslation()
  const grant = props.grant
  // ratio 授权的 benefit_key 为空；benefit 授权即使不限量（token_total=0）
  // 也带 benefit_key，因此可以据此区分两类授权，兼容旧接口缺失新字段的情况。
  const isBenefit =
    grant.type === 'benefit' ||
    Boolean(grant.benefit_key) ||
    (grant.token_total ?? 0) > 0 ||
    (grant.token_used ?? 0) > 0

  const parts: string[] = [grant.code_name || grant.code]
  if (isBenefit) {
    const modelNames = Object.keys(grant.model_ratio_overrides ?? {})
    // benefit 授权快照：整组免费 ratio_scope=group；单模型免费 ratio_scope=models
    // 且 model_ratio_overrides 只含该模型，据此展示权益范围。
    if (grant.ratio_scope === 'models' && modelNames.length > 0) {
      parts.push(t('Model {{model}}', { model: modelNames.join(', ') }))
    } else {
      parts.push(t('Whole group'))
    }
    parts.push(
      describeTokenUsage(
        t,
        grant.token_used ?? 0,
        grant.token_total ?? 0,
        i18n.language
      )
    )
  } else {
    parts.push(describeActivityRatio(t, grant.ratio_mode, grant.ratio_override))
    const listedModels = formatModelRatioOverrides(
      t,
      grant.model_ratio_overrides
    )
    if (listedModels) parts.push(`${t('Models (ratio)')}: ${listedModels}`)
  }
  parts.push(`${t('Expires at')}: ${describeExpireTime(t, grant.expire_time)}`)
  if (props.showPaid && grant.price_quota > 0) {
    parts.push(`${t('Paid')}: ${formatQuota(grant.price_quota)}`)
  }

  return (
    <div className='flex items-center justify-between gap-3 rounded-lg border p-3'>
      <div className='min-w-0 space-y-1'>
        <div className='flex items-center gap-2'>
          <span className='text-sm font-medium'>{grant.group_name}</span>
          <StatusBadge
            label={t(grantStatusLabelKey(grant.status))}
            variant={grant.status === 'active' ? 'success' : 'neutral'}
            copyable={false}
          />
        </div>
        <p className='text-muted-foreground text-xs break-all'>
          {parts.join(' · ')}
        </p>
      </div>
    </div>
  )
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
  // 同一兑换批次的多条权益共享一笔支付金额，只在第一条授权上展示。
  const seenBatches = new Set<string>()
  const rows = grants.map((grant) => {
    const batch = grant.redeem_batch
    const showPaid = !batch || !seenBatches.has(batch)
    if (batch) seenBatches.add(batch)
    return { grant, showPaid }
  })

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
        {rows.map((row) => (
          <ActivityGrantRow
            key={row.grant.id}
            grant={row.grant}
            showPaid={row.showPaid}
          />
        ))}
      </CardContent>
    </Card>
  )
}
