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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  Field,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import { formatQuotaWithCurrency } from '@/lib/currency'

import { getRedemptionCategories, updateWalletTierVisibility } from '../api'

export function WalletTierVisibilityDialog(props: { disabled?: boolean }) {
  const { t } = useTranslation()
  const prefix = useId()
  const [open, setOpen] = useState(false)
  const [edits, setEdits] = useState<Partial<Record<number, boolean>>>({})
  const queryClient = useQueryClient()
  const catalog = useQuery({
    queryKey: ['redemption-categories'],
    queryFn: getRedemptionCategories,
    enabled: open,
  })
  const save = useMutation({
    mutationFn: updateWalletTierVisibility,
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['redemption-categories'],
      })
      setOpen(false)
    },
  })
  const tiers = open && catalog.data ? catalog.data : []
  const show: number[] = []
  const hide: number[] = []
  for (const tier of tiers) {
    const edited = edits[tier.id]
    if (edited === undefined || edited === (tier.wallet_visible === true)) {
      continue
    }
    if (edited) {
      show.push(tier.id)
    } else {
      hide.push(tier.id)
    }
  }
  const title = t('Choose wallet tiers')
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (save.isPending) return
        if (next) {
          setEdits({})
          save.reset()
        }
        setOpen(next)
      }}
    >
      <DialogTrigger
        disabled={props.disabled}
        render={<Button type='button' size='sm' variant='outline' />}
      >
        {title}
      </DialogTrigger>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            {t('Only checked tiers appear in the wallet.')}{' '}
            {t(
              'Hiding tiers does not affect card purchases or code redemption.'
            )}
          </DialogDescription>
        </DialogHeader>
        {catalog.isPending && <p role='status'>{t('Loading...')}</p>}
        {catalog.isError && (
          <div className='flex items-center justify-between gap-2'>
            <p role='alert'>
              {catalog.error.message || t('Unable to load code tiers.')}
            </p>
            <Button
              type='button'
              size='sm'
              variant='outline'
              onClick={() => void catalog.refetch()}
            >
              {t('Retry')}
            </Button>
          </div>
        )}
        {catalog.isSuccess && tiers.length === 0 && (
          <p>{t('No categories yet')}</p>
        )}
        {catalog.isSuccess && tiers.length > 0 && (
          <FieldSet disabled={save.isPending}>
            <FieldLegend className='sr-only'>
              {t('Show code tiers in the wallet')}
            </FieldLegend>
            <FieldGroup className='grid max-h-[50vh] grid-cols-1 gap-2 overflow-y-auto p-1 sm:grid-cols-2'>
              {tiers.map((tier) => {
                const id = `${prefix}-${tier.id}`
                return (
                  <Field
                    key={tier.id}
                    orientation='horizontal'
                    className='min-w-0 rounded-md border px-2.5 py-2'
                  >
                    <Checkbox
                      id={id}
                      disabled={save.isPending}
                      checked={edits[tier.id] ?? tier.wallet_visible === true}
                      onCheckedChange={(checked) =>
                        setEdits((previous) => ({
                          ...previous,
                          [tier.id]: checked,
                        }))
                      }
                    />
                    <FieldLabel
                      htmlFor={id}
                      className='min-w-0 flex-wrap gap-x-1.5 wrap-anywhere'
                    >
                      {tier.name}
                      <span className='text-muted-foreground font-normal tabular-nums'>
                        {formatQuotaWithCurrency(tier.quota, {
                          abbreviate: false,
                          digitsSmall: 6,
                        })}
                      </span>
                    </FieldLabel>
                  </Field>
                )
              })}
            </FieldGroup>
          </FieldSet>
        )}
        {save.error && (
          <p className='text-destructive text-sm' role='alert'>
            {save.error.message || t('Save failed')}
          </p>
        )}
        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            disabled={save.isPending}
            onClick={() => setOpen(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            disabled={
              !catalog.isSuccess ||
              save.isPending ||
              show.length + hide.length === 0
            }
            onClick={() => save.mutate({ show, hide })}
          >
            {save.isPending ? t('Saving...') : t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
