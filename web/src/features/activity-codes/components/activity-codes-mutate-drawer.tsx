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
import { CalendarClock, Layers3, Tag, Ticket } from 'lucide-react'
import { type FormEvent, useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DateTimePicker } from '@/components/datetime-picker'
import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { getCurrencyLabel } from '@/lib/currency'
import { getEditableQuotaStep } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'

import {
  createActivityCode,
  getActivityCode,
  getGroupNames,
  updateActivityCode,
} from '../api'
import {
  ACTIVITY_RATIO_MODE,
  ERROR_MESSAGES,
  SUCCESS_MESSAGES,
  getActivityRatioModeOptions,
} from '../constants'
import {
  ACTIVITY_CODE_FORM_DEFAULT_VALUES,
  type ActivityCodeFormValues,
  getActivityCodeFormSchema,
  transformActivityCodeToFormValues,
  transformFormValuesToPayload,
} from '../lib/activity-form'
import type { ActivityCode } from '../types'
import { useActivityCodes } from './activity-codes-provider'

type ActivityCodesMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: ActivityCode
}

function numberFieldValue(value: number): number {
  return Number.isFinite(value) ? value : 0
}

export function ActivityCodesMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: ActivityCodesMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const activityCodeId = currentRow?.id
  const { triggerRefresh } = useActivityCodes()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [groupNames, setGroupNames] = useState<string[]>([])

  const form = useForm<ActivityCodeFormValues>({
    resolver: zodResolver(getActivityCodeFormSchema(t)),
    defaultValues: ACTIVITY_CODE_FORM_DEFAULT_VALUES,
  })

  useEffect(() => {
    if (!open) return
    let ignoreResult = false
    void getGroupNames()
      .then((result) => {
        if (ignoreResult) return
        setGroupNames(result.data ?? [])
      })
      .catch((error) => {
        if (ignoreResult) return
        handleServerError(error)
      })
    return () => {
      ignoreResult = true
    }
  }, [open])

  useEffect(() => {
    if (!open) return
    if (!isUpdate || activityCodeId === undefined) {
      form.reset(ACTIVITY_CODE_FORM_DEFAULT_VALUES)
      return
    }
    let ignoreResult = false
    setIsLoading(true)
    form.reset(ACTIVITY_CODE_FORM_DEFAULT_VALUES)
    void getActivityCode(activityCodeId)
      .then((result) => {
        if (ignoreResult) return
        if (result.success && result.data) {
          form.reset(transformActivityCodeToFormValues(result.data))
          return
        }
        toast.error(result.message || t(ERROR_MESSAGES.LOAD_FAILED))
        onOpenChange(false)
      })
      .catch((error) => {
        if (ignoreResult) return
        handleServerError(error)
        onOpenChange(false)
      })
      .finally(() => {
        if (!ignoreResult) setIsLoading(false)
      })
    return () => {
      ignoreResult = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, isUpdate, activityCodeId])

  const onSubmit = async (values: ActivityCodeFormValues) => {
    setIsSubmitting(true)
    try {
      const payload = transformFormValuesToPayload(values)
      if (isUpdate && activityCodeId !== undefined) {
        const result = await updateActivityCode({
          ...payload,
          id: activityCodeId,
        })
        if (!result.success) {
          toast.error(result.message || t(ERROR_MESSAGES.UPDATE_FAILED))
          return
        }
        toast.success(t(SUCCESS_MESSAGES.UPDATED))
      } else {
        const result = await createActivityCode(payload)
        if (!result.success) {
          toast.error(result.message || t(ERROR_MESSAGES.CREATE_FAILED))
          return
        }
        toast.success(t(SUCCESS_MESSAGES.CREATED))
      }
      onOpenChange(false)
      triggerRefresh()
    } catch (error) {
      handleServerError(error)
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    void form.handleSubmit(onSubmit)(event)
  }

  const ratioMode = form.watch('ratio_mode')
  const currencyLabel = getCurrencyLabel()
  const quotaStep = getEditableQuotaStep()
  let submitButtonLabel = t('Save')
  if (isSubmitting) {
    submitButtonLabel = t('Saving...')
  } else if (isLoading) {
    submitButtonLabel = t('Loading...')
  }

  return (
    <Sheet
      open={open}
      onOpenChange={(nextOpen) => {
        onOpenChange(nextOpen)
        if (!nextOpen) form.reset()
      }}
    >
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[620px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isUpdate ? t('Update Activity Code') : t('Create Activity Code')}
          </SheetTitle>
          <SheetDescription>
            {t(
              'An activity code grants its target group to the user for a limited time. The group must already exist in group ratios.'
            )}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='activity-code-form'
            onSubmit={handleSubmit}
            className={sideDrawerFormClassName()}
            aria-busy={isLoading}
          >
            <fieldset disabled={isSubmitting || isLoading} className='contents'>
              <SideDrawerSection>
                <SideDrawerSectionHeader
                  title={t('Details')}
                  icon={<Tag />}
                  iconTone='info'
                />
                <FormField
                  control={form.control}
                  name='code'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Activity Code')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          disabled={isUpdate}
                          placeholder={t('For example: hdmfree or 夏日免费')}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Letters (any language), digits, _ and -. Latin letters are saved in lowercase. The code cannot be changed after creation.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='name'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Name')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder={t('Enter a name')} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='description'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Description')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder={t('Optional description')} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='group_name'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Target Group')}</FormLabel>
                      <Select
                        items={groupNames.map((group) => ({
                          value: group,
                          label: group,
                        }))}
                        value={field.value}
                        onValueChange={field.onChange}
                      >
                        <FormControl>
                          <SelectTrigger className='w-full'>
                            <SelectValue placeholder={t('Select a group')} />
                          </SelectTrigger>
                        </FormControl>
                        {/* 触发框占满表单宽度；弹层按内容自适应（至少与触发框同宽），
                            过高时按可用高度滚动，避免长分组名或长列表被截断。 */}
                        <SelectContent
                          alignItemWithTrigger={false}
                          className='max-h-[min(22rem,var(--available-height))] w-max min-w-(--anchor-width) max-w-[min(32rem,var(--available-width))]'
                        >
                          <SelectGroup>
                            {groupNames.map((group) => (
                              <SelectItem key={group} value={group}>
                                {group}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                      <FormDescription>
                        {t(
                          'Users with an active grant can switch their token group to it.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </SideDrawerSection>

              <SideDrawerSection>
                <SideDrawerSectionHeader
                  title={t('Pricing and Limits')}
                  icon={<Layers3 />}
                  iconTone='success'
                />
                <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
                  <FormField
                    control={form.control}
                    name='duration_hours'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Validity (hours)')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={0}
                            step={1}
                            value={field.value}
                            onChange={(event) =>
                              field.onChange(
                                numberFieldValue(event.target.valueAsNumber)
                              )
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t('0 means the grant never expires.')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='rpm_limit'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('RPM limit')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={0}
                            step={1}
                            value={field.value}
                            onChange={(event) =>
                              field.onChange(
                                numberFieldValue(event.target.valueAsNumber)
                              )
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t('Requests per minute. 0 means unlimited.')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='price_dollars'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('Price ({{currency}})', { currency: currencyLabel })}
                        </FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={0}
                            step={quotaStep}
                            value={field.value}
                            onChange={(event) =>
                              field.onChange(
                                numberFieldValue(event.target.valueAsNumber)
                              )
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'Paid with wallet balance only. 0 means the code is free.'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='ratio_mode'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Ratio')}</FormLabel>
                        <Select
                          items={getActivityRatioModeOptions(t)}
                          value={field.value}
                          onValueChange={field.onChange}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              {getActivityRatioModeOptions(t).map((option) => (
                                <SelectItem key={option.value} value={option.value}>
                                  {option.label}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
                {ratioMode === ACTIVITY_RATIO_MODE.OVERRIDE && (
                  <FormField
                    control={form.control}
                    name='ratio_override'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Custom ratio')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={0}
                            step={0.01}
                            value={field.value}
                            onChange={(event) =>
                              field.onChange(
                                numberFieldValue(event.target.valueAsNumber)
                              )
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t('0 means this group is completely free.')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
                <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
                  <FormField
                    control={form.control}
                    name='total_limit'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Total redemptions')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={0}
                            step={1}
                            value={field.value}
                            onChange={(event) =>
                              field.onChange(
                                numberFieldValue(event.target.valueAsNumber)
                              )
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t('0 means unlimited.')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='per_user_limit'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Redemptions per user')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={0}
                            step={1}
                            value={field.value}
                            onChange={(event) =>
                              field.onChange(
                                numberFieldValue(event.target.valueAsNumber)
                              )
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            '0 means unlimited. Extra redemptions extend the expiry.'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              </SideDrawerSection>

              <SideDrawerSection>
                <SideDrawerSectionHeader
                  title={t('Availability')}
                  icon={<CalendarClock />}
                  iconTone='warning'
                />
                <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
                  <FormField
                    control={form.control}
                    name='start_time'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Redemption starts')}</FormLabel>
                        <FormControl>
                          <DateTimePicker
                            value={field.value}
                            onChange={field.onChange}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='end_time'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Redemption ends')}</FormLabel>
                        <FormControl>
                          <DateTimePicker
                            value={field.value}
                            onChange={field.onChange}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
                <FormField
                  control={form.control}
                  name='public_visible'
                  render={({ field }) => (
                    <FormItem className='flex flex-row items-center justify-between rounded-lg border p-3'>
                      <div className='space-y-0.5'>
                        <FormLabel>{t('Show in activity center')}</FormLabel>
                        <FormDescription>
                          {t('Users can redeem it directly from their wallet.')}
                        </FormDescription>
                      </div>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              </SideDrawerSection>
            </fieldset>
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose
            render={<Button variant='outline' disabled={isSubmitting} />}
          >
            {t('Cancel')}
          </SheetClose>
          <Button type='submit' form='activity-code-form' disabled={isSubmitting}>
            <Ticket className='mr-2 h-4 w-4' />
            {submitButtonLabel}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
