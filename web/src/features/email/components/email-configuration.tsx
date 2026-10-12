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
import { Link } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { SettingsPageProvider } from '@/features/system-settings/components/settings-page-context'
import {
  getOptionValue,
  useSystemOptions,
} from '@/features/system-settings/hooks/use-system-options'

import type { EmailTab } from '../search'
import { BonusEmailSettings } from './bonus-email-settings'
import { NotificationSettings } from './notification-settings'
import { EmailSettingsSection } from './smtp-settings'

const DEFAULT_EMAIL_SETTINGS = {
  SMTPServer: '',
  SMTPPort: '',
  SMTPAccount: '',
  SMTPFrom: '',
  SMTPToken: '',
  SMTPSSLEnabled: false,
  SMTPStartTLSEnabled: false,
  SMTPInsecureSkipVerify: false,
  SMTPForceAuthLogin: false,
  QuotaRemindThreshold: '',
  'bonus_setting.bonus_email_subject': '',
  'bonus_setting.bonus_email_content': '',
}

type EmailConfigurationProps = {
  tab: Exclude<EmailTab, 'send'>
}

export function EmailConfiguration(props: EmailConfigurationProps) {
  const { t } = useTranslation()
  const options = useSystemOptions()
  const settings = useMemo(
    () => getOptionValue(options.data?.data, DEFAULT_EMAIL_SETTINGS),
    [options.data?.data]
  )
  if (options.isPending) return <p role='status'>{t('Loading settings...')}</p>

  if (!options.data?.success) {
    return (
      <Alert variant='destructive'>
        <AlertDescription className='flex flex-wrap items-center gap-3'>
          <span>{t('Failed to load email settings')}</span>
          <Button
            type='button'
            variant='outline'
            disabled={options.isFetching}
            onClick={() => void options.refetch()}
          >
            {t('Retry')}
          </Button>
        </AlertDescription>
      </Alert>
    )
  }

  return <EmailSettingsForm tab={props.tab} initialSettings={settings} />
}

function EmailSettingsForm(
  props: EmailConfigurationProps & {
    initialSettings: typeof DEFAULT_EMAIL_SETTINGS
  }
) {
  const { t } = useTranslation()
  // Option writes refetch after each field; keep in-progress forms independent of partial saves.
  const [settings] = useState(props.initialSettings)
  const [actionsContainer, setActionsContainer] =
    useState<HTMLDivElement | null>(null)
  const [titleStatusContainer, setTitleStatusContainer] =
    useState<HTMLSpanElement | null>(null)
  const title = {
    smtp: t('SMTP Email'),
    templates: t('Email Templates'),
    notifications: t('Email Notifications'),
  }[props.tab]

  return (
    <SettingsPageProvider
      actionsContainer={actionsContainer}
      titleStatusContainer={titleStatusContainer}
    >
      <div className='space-y-5'>
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <h3 className='flex min-w-0 items-center gap-2 text-base font-semibold'>
            {title}
            <span ref={setTitleStatusContainer} />
          </h3>
          <div
            ref={setActionsContainer}
            className='flex flex-wrap items-center gap-2'
          />
        </div>
        {props.tab === 'smtp' && (
          <EmailSettingsSection
            defaultValues={{
              SMTPServer: settings.SMTPServer,
              SMTPPort: settings.SMTPPort,
              SMTPAccount: settings.SMTPAccount,
              SMTPFrom: settings.SMTPFrom,
              SMTPToken: settings.SMTPToken,
              SMTPSSLEnabled: settings.SMTPSSLEnabled,
              SMTPStartTLSEnabled: settings.SMTPStartTLSEnabled,
              SMTPInsecureSkipVerify: settings.SMTPInsecureSkipVerify,
              SMTPForceAuthLogin: settings.SMTPForceAuthLogin,
            }}
          />
        )}
        {props.tab === 'templates' && (
          <BonusEmailSettings
            defaultValues={{
              bonus_setting: {
                bonus_email_subject:
                  settings['bonus_setting.bonus_email_subject'],
                bonus_email_content:
                  settings['bonus_setting.bonus_email_content'],
              },
            }}
          />
        )}
        {props.tab === 'notifications' && (
          <>
            <NotificationSettings
              defaultValues={{
                QuotaRemindThreshold: settings.QuotaRemindThreshold,
              }}
            />
            <Alert>
              <AlertDescription className='gap-3'>
                <p>
                  {t(
                    'Personal delivery preferences and account verification rules are managed separately.'
                  )}
                </p>
                <div className='flex flex-wrap gap-3'>
                  <Link
                    to='/profile'
                    className='text-primary underline underline-offset-4'
                  >
                    {t('Settings & Preferences')}
                  </Link>
                  <Link
                    to='/system-settings/auth/$section'
                    params={{ section: 'basic-auth' }}
                    className='text-primary underline underline-offset-4'
                  >
                    {t('Basic Authentication')}
                  </Link>
                </div>
              </AlertDescription>
            </Alert>
          </>
        )}
      </div>
    </SettingsPageProvider>
  )
}
