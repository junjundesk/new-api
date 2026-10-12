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
import { FormDirtyIndicator } from '@/features/system-settings/components/form-dirty-indicator'
import { FormNavigationGuard } from '@/features/system-settings/components/form-navigation-guard'
import { SettingsForm } from '@/features/system-settings/components/settings-form-layout'
import { SettingsPageFormActions } from '@/features/system-settings/components/settings-page-context'
import { SettingsSection } from '@/features/system-settings/components/settings-section'
import { useSettingsForm } from '@/features/system-settings/hooks/use-settings-form'
import { useUpdateOption } from '@/features/system-settings/hooks/use-update-option'

const notificationSchema = z.object({
  QuotaRemindThreshold: z
    .string()
    .trim()
    .refine((value) => {
      if (!value) return true
      return !Number.isNaN(Number(value)) && Number(value) >= 0
    }, 'Enter a non-negative number or leave empty'),
})

type NotificationFormValues = z.infer<typeof notificationSchema>

type NotificationSettingsProps = {
  defaultValues: NotificationFormValues
}

export function NotificationSettings(props: NotificationSettingsProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<NotificationFormValues>({
      resolver: zodResolver(notificationSchema),
      defaultValues: props.defaultValues,
      compareValues: (left, right) =>
        String(left ?? '').trim() === String(right ?? '').trim(),
      onSubmit: async (data) => {
        const result = await updateOption.mutateAsync({
          key: 'QuotaRemindThreshold',
          value: data.QuotaRemindThreshold,
        })
        if (!result.success) {
          throw new Error(result.message || t('Failed to update setting'))
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
    <SettingsSection title={t('Email Notifications')}>
      <FormNavigationGuard when={isDirty} />
      <Form {...form}>
        <SettingsForm onSubmit={handleSave}>
          <SettingsPageFormActions
            onSave={handleSave}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <FormField
            control={form.control}
            name='QuotaRemindThreshold'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Quota reminder (tokens)')}</FormLabel>
                <FormControl>
                  <Input type='number' min={0} step={1} {...field} />
                </FormControl>
                <FormDescription>
                  {t('Send email alerts when a user falls below this quota')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
