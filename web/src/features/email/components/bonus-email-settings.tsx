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
import { zodResolver } from '@hookform/resolvers/zod'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { FormDirtyIndicator } from '@/features/system-settings/components/form-dirty-indicator'
import { FormNavigationGuard } from '@/features/system-settings/components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsFormGridItem,
} from '@/features/system-settings/components/settings-form-layout'
import { SettingsPageFormActions } from '@/features/system-settings/components/settings-page-context'
import { SettingsSection } from '@/features/system-settings/components/settings-section'
import { useSettingsForm } from '@/features/system-settings/hooks/use-settings-form'
import { useUpdateOption } from '@/features/system-settings/hooks/use-update-option'

const bonusEmailSchema = z.object({
  bonus_setting: z.object({
    bonus_email_subject: z.string(),
    bonus_email_content: z.string(),
  }),
})

type BonusEmailFormValues = z.infer<typeof bonusEmailSchema>

type BonusEmailSettingsProps = {
  defaultValues: BonusEmailFormValues
}

// Mirrors operation_setting.DefaultBonusEmailContent on the backend, which is
// used when the admin leaves the template empty.
const DEFAULT_BONUS_EMAIL_CONTENT = `<p>您好 $display_name，</p>
<p>您已收到一笔赠金：<b>$amount</b>。</p>
<p>有效期：$expire_text</p>
$remark_block`

// Mirrors the placeholders substituted by renderBonusEmail in the backend.
const BONUS_EMAIL_VARIABLES = [
  { token: '$username', description: 'Login username' },
  {
    token: '$display_name',
    description: 'Display name (falls back to username)',
  },
  { token: '$amount', description: 'Bonus amount granted' },
  {
    token: '$expire_time',
    description: 'Expiry timestamp (empty if permanent)',
  },
  {
    token: '$expire_text',
    description: 'Expiry sentence, e.g. "… expires" or "Never expires"',
  },
  { token: '$remark', description: 'Grant remark text' },
  {
    token: '$remark_block',
    description: 'Remark paragraph, empty when there is no remark',
  },
  { token: '$site_name', description: 'Site name' },
  {
    token: '$base_url',
    description: 'Site base URL (no trailing slash), for absolute links',
  },
] as const

export function BonusEmailSettings(props: BonusEmailSettingsProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<BonusEmailFormValues>({
      resolver: zodResolver(bonusEmailSchema),
      defaultValues: props.defaultValues,
      onSubmit: async (data, changedFields) => {
        for (const field of [
          'bonus_email_subject',
          'bonus_email_content',
        ] as const) {
          const key = `bonus_setting.${field}`
          if (!Object.hasOwn(changedFields, key)) continue

          const result = await updateOption.mutateAsync({
            key,
            value: data.bonus_setting[field],
          })
          if (!result.success) {
            throw new Error(result.message || t('Failed to update setting'))
          }
        }
      },
    })

  const handleSave = async (
    event?: FormEvent<HTMLFormElement>
  ): Promise<void> => {
    try {
      await handleSubmit(event)
    } catch {
      // The mutation reports the error; keep the form dirty for retry.
    }
  }

  return (
    <SettingsSection title={t('Bonus Email Template')}>
      <FormNavigationGuard when={isDirty} />
      <Form {...form}>
        <SettingsForm onSubmit={handleSave}>
          <SettingsPageFormActions
            onSave={handleSave}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='bonus_setting.bonus_email_subject'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Bonus Email Subject')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('Bonus arrival notification')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Leave empty to use the built-in default subject')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='bonus_setting.bonus_email_content'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Bonus Email Template')}</FormLabel>
                    <FormControl>
                      <Textarea
                        rows={8}
                        className='font-mono text-xs'
                        placeholder={DEFAULT_BONUS_EMAIL_CONTENT}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'HTML body of the bonus arrival email. Leave empty to use the built-in template.'
                      )}
                      <br />
                      {t(
                        'Line breaks are preserved: pressing Enter or writing \\n both render as a new line.'
                      )}
                    </FormDescription>
                    <div className='text-muted-foreground space-y-1 text-xs'>
                      <div className='font-medium'>
                        {t('Available variables:')}
                      </div>
                      <ul className='grid gap-0.5 sm:grid-cols-2'>
                        {BONUS_EMAIL_VARIABLES.map((variable) => (
                          <li key={variable.token}>
                            <code className='text-foreground'>
                              {variable.token}
                            </code>{' '}
                            — {t(variable.description)}
                          </li>
                        ))}
                      </ul>
                    </div>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
