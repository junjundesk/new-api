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
import { Gift, Plus, Trash2 } from 'lucide-react'
import { Controller, useFieldArray, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { FieldError } from '@/components/ui/field'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from '@/components/ui/input-group'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { formatQuota } from '@/lib/format'

import { saveRedemptionBonusRules } from '../api'
import { REDEMPTION_VALIDATION } from '../constants'
import { getRedemptionBonusQuota } from '../lib'
import type { RedemptionBonusRule } from '../types'
import { useRedemptions } from './redemptions-provider'

type BonusRulesDialogProps = {
  onClose: () => void
}

export function RedemptionBonusRulesDialog(props: BonusRulesDialogProps) {
  const { t } = useTranslation()
  const { categories, bonusRules } = useRedemptions()
  const queryClient = useQueryClient()
  const quotaOf = (categoryId: number) =>
    categories.find((category) => category.id === categoryId)?.quota ?? 0
  const schema = z.object({
    rules: z
      .array(
        z
          .object({
            category_id: z
              .number()
              .refine(
                (id) => categories.some((category) => category.id === id),
                t('Select a category')
              ),
            percent: z
              .number(t('Enter a valid bonus rate'))
              .min(0, t('Enter a valid bonus rate'))
              .max(1000, t('Enter a valid bonus rate'))
              .multipleOf(0.01, t('Use up to two decimal places')),
          })
          .refine(
            (rule) =>
              getRedemptionBonusQuota(
                quotaOf(rule.category_id),
                Math.round(rule.percent * 100)
              ) < REDEMPTION_VALIDATION.MAX_QUOTA,
            { path: ['percent'], message: t('Enter a valid bonus rate') }
          )
      )
      .superRefine((rules, ctx) => {
        const seen = new Set<number>()
        rules.forEach((rule, index) => {
          if (seen.has(rule.category_id)) {
            ctx.addIssue({
              code: 'custom',
              path: [index, 'category_id'],
              message: t('This category already has a rule'),
            })
          }
          seen.add(rule.category_id)
        })
      }),
  })
  const form = useForm({
    resolver: zodResolver(schema),
    defaultValues: {
      rules: bonusRules.map((rule) => ({
        category_id: rule.category_id,
        percent: rule.bonus_rate_bps / 100,
      })),
    },
  })
  const rules = useFieldArray({ control: form.control, name: 'rules' })
  const watched = form.watch('rules')
  const errors = form.formState.errors.rules
  const save = useMutation({
    mutationFn: saveRedemptionBonusRules,
    onSuccess: (saved) => {
      queryClient.setQueryData<RedemptionBonusRule[]>(
        ['redemption-bonus-rules'],
        saved
      )
      void queryClient.invalidateQueries({
        queryKey: ['redemption-bonus-rules'],
      })
      toast.success(t('Bonus rules saved'))
      props.onClose()
    },
  })
  const addRule = () => {
    const used = new Set(watched.map((rule) => rule.category_id))
    const next = categories.find((category) => !used.has(category.id))
    if (next) rules.append({ category_id: next.id, percent: 5 })
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !save.isPending) props.onClose()
      }}
    >
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Bonus rules')}</DialogTitle>
          <DialogDescription>
            {t(
              'Set an extra bonus percentage per category. Changes apply immediately to all unused codes.'
            )}
          </DialogDescription>
        </DialogHeader>
        <form
          noValidate
          id='redemption-bonus-rules-form'
          className='flex max-h-[60vh] flex-col gap-2 overflow-y-auto'
          onSubmit={form.handleSubmit((values) =>
            save.mutate(
              values.rules.map((rule) => ({
                category_id: rule.category_id,
                bonus_rate_bps: Math.round(rule.percent * 100),
              }))
            )
          )}
        >
          {rules.fields.length > 0 && (
            <div className='text-muted-foreground hidden grid-cols-[minmax(0,1fr)_8rem_minmax(0,1.4fr)_2rem] gap-3 px-1 text-xs sm:grid'>
              <span>{t('Category')}</span>
              <span>{t('Bonus rate')}</span>
              <span>{t('Face value + Bonus = Actual amount')}</span>
            </div>
          )}
          {rules.fields.map((field, index) => {
            const rule = watched[index]
            const quota = quotaOf(rule?.category_id ?? 0)
            const bonus = getRedemptionBonusQuota(
              quota,
              Math.round((rule?.percent ?? 0) * 100)
            )
            const rowErrors = errors?.[index]
            const valid = quota > 0 && Number.isFinite(bonus) && bonus >= 0
            return (
              <div
                key={field.id}
                className='grid grid-cols-[minmax(0,1fr)_2rem] items-start gap-x-3 gap-y-2 rounded-lg border p-2.5 sm:grid-cols-[minmax(0,1fr)_8rem_minmax(0,1.4fr)_2rem] sm:border-0 sm:p-0 sm:px-1'
              >
                <div className='flex flex-col gap-1'>
                  <Controller
                    control={form.control}
                    name={`rules.${index}.category_id`}
                    render={({ field: select }) => (
                      <NativeSelect
                        ref={select.ref}
                        name={select.name}
                        value={select.value}
                        onBlur={select.onBlur}
                        onChange={(event) =>
                          select.onChange(Number(event.target.value))
                        }
                        className='w-full'
                        aria-label={t('Category')}
                        aria-invalid={!!rowErrors?.category_id}
                      >
                        {categories.map((category) => (
                          <NativeSelectOption
                            key={category.id}
                            value={category.id}
                          >
                            {category.name} · {formatQuota(category.quota)}
                          </NativeSelectOption>
                        ))}
                      </NativeSelect>
                    )}
                  />
                  {rowErrors?.category_id && (
                    <FieldError>{rowErrors.category_id.message}</FieldError>
                  )}
                </div>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-sm'
                  className='text-muted-foreground hover:text-destructive sm:order-last'
                  aria-label={t('Delete rule')}
                  onClick={() => rules.remove(index)}
                >
                  <Trash2 />
                </Button>
                <div className='flex flex-col gap-1'>
                  <InputGroup>
                    <InputGroupInput
                      type='number'
                      min={0}
                      max={1000}
                      step='0.01'
                      inputMode='decimal'
                      aria-label={t('Bonus rate')}
                      aria-invalid={!!rowErrors?.percent}
                      {...form.register(`rules.${index}.percent`, {
                        valueAsNumber: true,
                      })}
                    />
                    <InputGroupAddon align='inline-end'>%</InputGroupAddon>
                  </InputGroup>
                  {rowErrors?.percent && (
                    <FieldError>{rowErrors.percent.message}</FieldError>
                  )}
                </div>
                <output className='bg-muted/50 col-span-2 flex h-8 flex-wrap items-center gap-x-1.5 rounded-md px-2.5 font-mono text-sm tabular-nums sm:col-span-1'>
                  {valid ? (
                    <>
                      <span>{formatQuota(quota)}</span>
                      <span className='text-muted-foreground'>+</span>
                      <span className='text-amber-700 dark:text-amber-300'>
                        {formatQuota(bonus)}
                      </span>
                      <span className='text-muted-foreground'>=</span>
                      <strong>{formatQuota(quota + bonus)}</strong>
                    </>
                  ) : (
                    '—'
                  )}
                </output>
              </div>
            )
          })}
          {rules.fields.length === 0 && (
            <div className='text-muted-foreground flex flex-col items-center gap-2 rounded-lg border border-dashed px-4 py-8 text-center text-sm'>
              <Gift aria-hidden className='size-5' />
              {categories.length
                ? t('No bonus rules yet')
                : t('Create a category before adding bonus rules')}
            </div>
          )}
          {save.error && (
            <FieldError role='alert'>{save.error.message}</FieldError>
          )}
        </form>
        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            className='sm:mr-auto'
            disabled={watched.length >= categories.length}
            onClick={addRule}
          >
            <Plus />
            {t('Add rule')}
          </Button>
          <Button
            variant='outline'
            disabled={save.isPending}
            onClick={props.onClose}
          >
            {t('Cancel')}
          </Button>
          <Button
            form='redemption-bonus-rules-form'
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
