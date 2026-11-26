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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'

import { migrateUncategorizedRedemptions } from '../api'
import { useRedemptions } from './redemptions-provider'

export function RedemptionMigrationDialog(props: { onClose: () => void }) {
  const { t } = useTranslation()
  const { categories } = useRedemptions()
  const queryClient = useQueryClient()
  const [allowFuzzy, setAllowFuzzy] = useState(false)
  const migration = useMutation({
    mutationFn: migrateUncategorizedRedemptions,
    onSuccess: ({ migrated, unmatched }) => {
      void queryClient.invalidateQueries({ queryKey: ['redemptions'] })
      toast.success(
        t(
          'Redemption code migration: {{migrated}} moved, {{unmatched}} unmatched',
          {
            migrated,
            unmatched,
          }
        )
      )
      props.onClose()
    },
  })

  return (
    <ConfirmDialog
      open
      className='max-h-[85dvh] overflow-y-auto data-[size=default]:sm:max-w-md'
      title={t('Migrate uncategorized codes')}
      desc={t(
        'This checks all unused codes in Uncategorized, not just this page. Disabled and expired codes are skipped.'
      )}
      confirmText={
        migration.isPending ? t('Processing...') : t('Confirm migration')
      }
      destructive={allowFuzzy}
      disabled={categories.length === 0}
      isLoading={migration.isPending}
      onOpenChange={(open) => {
        if (!open && !migration.isPending) props.onClose()
      }}
      handleConfirm={() => migration.mutate(allowFuzzy ? 'fuzzy' : 'exact')}
    >
      {categories.length === 0 ? (
        <FieldDescription>
          {t('Create a category before migrating codes.')}
        </FieldDescription>
      ) : (
        <FieldSet>
          <FieldLegend variant='label'>{t('Matching rule')}</FieldLegend>
          <FieldDescription>
            {t(
              'By default, only codes with an exact category face value are moved. Their amounts do not change.'
            )}
          </FieldDescription>
          <FieldGroup>
            <Field orientation='horizontal' data-disabled={migration.isPending}>
              <Checkbox
                id='redemption-migration-fuzzy'
                aria-describedby='redemption-migration-fuzzy-description'
                checked={allowFuzzy}
                onCheckedChange={setAllowFuzzy}
                disabled={migration.isPending}
              />
              <FieldContent>
                <FieldLabel htmlFor='redemption-migration-fuzzy'>
                  {t('Allow fuzzy matching (round down)')}
                </FieldLabel>
                <FieldDescription id='redemption-migration-fuzzy-description'>
                  {t(
                    'Set the code amount to the highest category value it covers. With tiers of 10 and 20, 10.3 becomes 10 and 21 becomes 20.'
                  )}
                </FieldDescription>
              </FieldContent>
            </Field>
          </FieldGroup>
          <FieldDescription>
            {t(
              'If no tier matches or several categories share the matching face value, the code stays uncategorized.'
            )}
          </FieldDescription>
        </FieldSet>
      )}
      {migration.error && (
        <FieldError>
          {migration.error.message || t('Failed to migrate redemption codes')}
        </FieldError>
      )}
    </ConfirmDialog>
  )
}
