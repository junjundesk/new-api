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
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Card, CardContent, CardHeader } from '@/components/ui/card'

import { ActivityGrantsCard } from './components/activity-grants-card'
import { ActivityRedeemPanel } from './components/activity-redeem-panel'

export function ActivityCenter() {
  const { t } = useTranslation()
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Activity Center')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <Card>
            <CardHeader>
              <div className='space-y-0.5'>
                <p className='text-sm font-medium'>{t('Redeem an Activity Code')}</p>
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Free codes are redeemed instantly. Paid codes ask you to confirm the wallet deduction first.'
                  )}
                </p>
              </div>
            </CardHeader>
            <CardContent>
              <ActivityRedeemPanel showOffers />
            </CardContent>
          </Card>
          <ActivityGrantsCard />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
