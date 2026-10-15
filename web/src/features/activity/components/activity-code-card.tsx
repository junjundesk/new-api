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
import { Gift } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'

import { getActivityOffers } from '../api'
import { ActivityRedeemPanel } from './activity-redeem-panel'

export function ActivityCodeCard() {
  const { t } = useTranslation()
  const { data } = useQuery({
    queryKey: ['activity-offers'],
    queryFn: async () => {
      const result = await getActivityOffers()
      if (!result.success) {
        return { enabled: false, offers: [] }
      }
      return result.data ?? { enabled: false, offers: [] }
    },
  })

  // 站点没有启用中的活动码时不占用钱包版面。
  if (!data?.enabled) return null

  return (
    <Card>
      <CardHeader>
        <div className='flex items-center gap-2'>
          <IconBadge tone='warning' size='sm'>
            <Gift />
          </IconBadge>
          <div className='space-y-0.5'>
            <p className='text-sm font-medium'>{t('Activity Code')}</p>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Redeem a code to unlock a group for a limited time and at a special ratio.'
              )}
            </p>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <ActivityRedeemPanel showOffers />
      </CardContent>
    </Card>
  )
}
