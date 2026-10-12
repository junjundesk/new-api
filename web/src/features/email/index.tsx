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
import { useNavigate, useSearch } from '@tanstack/react-router'
import { lazy, Suspense } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { EmailComposer } from './components/email-composer'
import { emailSearchSchema } from './search'

const EmailConfiguration = lazy(() =>
  import('./components/email-configuration').then((module) => ({
    default: module.EmailConfiguration,
  }))
)

export function Email() {
  const { t } = useTranslation()
  const search = useSearch({ from: '/_authenticated/email/' })
  const navigate = useNavigate({ from: '/email/' })
  const isRoot = useAuthStore(
    (state) => (state.auth.user?.role ?? 0) >= ROLE.SUPER_ADMIN
  )
  const activeTab = isRoot ? (search.tab ?? 'send') : 'send'

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Email Center')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto w-full max-w-3xl space-y-5'>
          <p className='text-muted-foreground text-sm'>
            {t('Manage outgoing email and notification settings.')}
          </p>
          <Tabs
            value={activeTab}
            onValueChange={(value) => {
              const next = emailSearchSchema.parse({ tab: value })
              void navigate({ search: next })
            }}
            className='gap-5'
          >
            <div className='max-w-full overflow-x-auto overflow-y-hidden p-1'>
              <TabsList aria-label={t('Email Center')}>
                <TabsTrigger value='send'>{t('Send Email')}</TabsTrigger>
                {isRoot && (
                  <TabsTrigger value='smtp'>{t('SMTP Email')}</TabsTrigger>
                )}
                {isRoot && (
                  <TabsTrigger value='templates'>
                    {t('Email Templates')}
                  </TabsTrigger>
                )}
                {isRoot && (
                  <TabsTrigger value='notifications'>
                    {t('Email Notifications')}
                  </TabsTrigger>
                )}
              </TabsList>
            </div>
            <TabsContent value='send'>
              <EmailComposer />
            </TabsContent>
            {isRoot && activeTab !== 'send' && (
              <TabsContent value={activeTab}>
                <Suspense
                  fallback={<p role='status'>{t('Loading settings...')}</p>}
                >
                  <EmailConfiguration key={activeTab} tab={activeTab} />
                </Suspense>
              </TabsContent>
            )}
          </Tabs>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
