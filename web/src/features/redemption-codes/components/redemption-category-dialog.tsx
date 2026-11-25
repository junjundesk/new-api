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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Gift, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { IconBadge } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import { getCurrencyLabel } from '@/lib/currency'
import {
  formatQuota,
  getEditableQuotaStep,
  parseQuotaFromDollars,
  quotaUnitsToEditableAmount,
} from '@/lib/format'

import { deleteRedemptionCategory, saveRedemptionCategory } from '../api'
import { REDEMPTION_VALIDATION } from '../constants'
import type { RedemptionCategory } from '../types'
import { useRedemptions } from './redemptions-provider'

type CategoryDialogProps = {
  category?: RedemptionCategory
  onClose: () => void
  onSelect: (id?: number) => void
}

export function RedemptionCategoryDialog(props: CategoryDialogProps) {
  const { t } = useTranslation()
  const { categoryId } = useRedemptions()
  const queryClient = useQueryClient()
  const category = props.category
  const [confirmDelete, setConfirmDelete] = useState(false)
  const schema = z
    .object({
      name: z
        .string()
        .trim()
        .min(1, t('Enter a name'))
        .max(
          20,
          t('Name must be between {{min}} and {{max}} characters', {
            min: 1,
            max: 20,
          })
        ),
      amount: z.number(t('Enter a valid face value')).refine((value) => {
        const quota = category?.quota ?? parseQuotaFromDollars(value)
        return quota > 0 && quota <= REDEMPTION_VALIDATION.MAX_QUOTA
      }, t('Enter a valid face value')),
      percent: z
        .number(t('Enter a valid bonus rate'))
        .min(0, t('Enter a valid bonus rate'))
        .multipleOf(0.01, t('Use up to two decimal places')),
    })
    .refine(
      ({ amount, percent }) => {
        const bps = Math.round(percent * 100)
        const quota = category?.quota ?? parseQuotaFromDollars(amount)
        return (
          bps <= 2_147_483_647 &&
          Math.round((quota * bps) / 10000) <= REDEMPTION_VALIDATION.MAX_QUOTA
        )
      },
      { path: ['percent'], message: t('Enter a valid bonus rate') }
    )
  const {
    register,
    handleSubmit,
    watch,
    formState: { errors },
  } = useForm({
    resolver: zodResolver(schema),
    defaultValues: {
      name: category?.name ?? '',
      amount: category ? quotaUnitsToEditableAmount(category.quota) : 100,
      percent: (category?.bonus_rate_bps ?? 0) / 100,
    },
  })
  const [amount, percent] = watch(['amount', 'percent'])
  const faceValue = category?.quota ?? parseQuotaFromDollars(amount)
  const bonus = Math.round((faceValue * Math.round(percent * 100)) / 10000)
  const save = useMutation({
    mutationFn: saveRedemptionCategory,
    onSuccess: (saved) => {
      queryClient.setQueryData<RedemptionCategory[]>(
        ['redemption-categories'],
        (previous = []) =>
          [...previous.filter((entry) => entry.id !== saved.id), saved].sort(
            (a, b) => a.quota - b.quota || a.id - b.id
          )
      )
      void queryClient.invalidateQueries({
        queryKey: ['redemption-categories'],
      })
      props.onSelect(saved.id)
      props.onClose()
    },
  })
  const remove = useMutation({
    mutationFn: deleteRedemptionCategory,
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['redemption-categories'],
      })
      if (categoryId === category?.id) props.onSelect(undefined)
      props.onClose()
    },
  })
  if (confirmDelete && category) {
    return (
      <ConfirmDialog
        open
        destructive
        isLoading={remove.isPending}
        title={t('Delete category?')}
        desc={
          <>
            {t('Only empty categories can be deleted.')}{' '}
            {remove.error && <span role='alert'>{remove.error.message}</span>}
          </>
        }
        confirmText={t('Delete')}
        onOpenChange={(open) => {
          if (!open && !remove.isPending) setConfirmDelete(false)
        }}
        handleConfirm={() => remove.mutate(category.id)}
      />
    )
  }
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !save.isPending) props.onClose()
      }}
    >
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>
            {category ? t('Edit category') : t('New category')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'The bonus rate applies to all unused codes in this category at redemption.'
            )}
          </DialogDescription>
        </DialogHeader>
        <form
          noValidate
          id='redemption-category-form'
          onSubmit={handleSubmit((values) =>
            save.mutate({
              id: category?.id,
              name: values.name,
              quota: category?.quota ?? parseQuotaFromDollars(values.amount),
              bonus_rate_bps: Math.round(values.percent * 100),
            })
          )}
        >
          <FieldGroup className='gap-4'>
            <Field data-invalid={!!errors.name}>
              <FieldLabel htmlFor='category-name'>
                {t('Category name')}
              </FieldLabel>
              <Input
                id='category-name'
                autoFocus
                maxLength={20}
                aria-invalid={!!errors.name}
                {...register('name')}
              />
              {errors.name && <FieldError>{errors.name.message}</FieldError>}
            </Field>
            <div className='grid grid-cols-2 gap-3'>
              <Field data-invalid={!!errors.amount}>
                <FieldLabel htmlFor='category-amount'>
                  {t('Face value ({{currency}})', {
                    currency: getCurrencyLabel(),
                  })}
                </FieldLabel>
                <Input
                  id='category-amount'
                  type='number'
                  required
                  readOnly={!!category}
                  min={0}
                  step={getEditableQuotaStep()}
                  aria-invalid={!!errors.amount}
                  {...register('amount', { valueAsNumber: true })}
                />
                {errors.amount && (
                  <FieldError>{errors.amount.message}</FieldError>
                )}
              </Field>
              <Field data-invalid={!!errors.percent}>
                <FieldLabel htmlFor='category-percent'>
                  {t('Bonus rate (%)')}
                </FieldLabel>
                <Input
                  id='category-percent'
                  type='number'
                  min={0}
                  required
                  step='0.01'
                  aria-invalid={!!errors.percent}
                  {...register('percent', { valueAsNumber: true })}
                />
                {errors.percent && (
                  <FieldError>{errors.percent.message}</FieldError>
                )}
              </Field>
            </div>
            <div className='bg-muted/50 flex items-center justify-between rounded-lg border px-3 py-2.5'>
              <span className='flex items-center gap-2 text-sm'>
                <IconBadge tone='warning'>
                  <Gift aria-hidden className='size-4' />
                </IconBadge>
                {t('Bonus per code')}
              </span>
              <output className='font-mono font-semibold tabular-nums'>
                {Number.isFinite(bonus) && bonus >= 0
                  ? formatQuota(bonus)
                  : '—'}
              </output>
            </div>
            {save.error && (
              <FieldError role='alert'>{save.error.message}</FieldError>
            )}
          </FieldGroup>
        </form>
        <DialogFooter>
          {category && (
            <Button
              variant='ghost'
              className='text-destructive sm:mr-auto'
              disabled={save.isPending}
              onClick={() => setConfirmDelete(true)}
            >
              <Trash2 />
              {t('Delete')}
            </Button>
          )}
          <Button
            variant='outline'
            disabled={save.isPending}
            onClick={props.onClose}
          >
            {t('Cancel')}
          </Button>
          <Button
            form='redemption-category-form'
            type='submit'
            disabled={save.isPending}
          >
            {save.isPending ? t('Saving...') : t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
