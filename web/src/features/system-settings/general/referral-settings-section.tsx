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
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Alert, AlertDescription } from '@/components/ui/alert'
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
import { Switch } from '@/components/ui/switch'
import { formatQuota } from '@/lib/format'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

// The ladder is edited as percentage values but stored in basis points, so a
// rate the admin types as "1.5" becomes 150 and survives the round trip
// without float drift.
const tierSchema = z.object({
  minInvitees: z.coerce.number().int().min(1),
  ratePercent: z.coerce.number().min(0).max(100),
})

const schema = z.object({
  enabled: z.boolean(),
  inviteeEnabled: z.boolean(),
  inviteeRatePercent: z.coerce.number().min(0).max(100),
  inviteeMaxQuota: z.coerce.number().min(0),
  inviterEnabled: z.boolean(),
  inviterMaxQuota: z.coerce.number().min(0),
  bonusDuration: z.string(),
  countRedeemAsValid: z.boolean(),
  tiers: z.array(tierSchema).min(1),
})

type Values = z.infer<typeof schema>

export type ReferralTierDefaults = {
  min_invitees: number
  rate_bps: number
}

export type ReferralSettingsDefaults = {
  enabled: boolean
  inviteeFirstTopUpEnabled: boolean
  inviteeFirstTopUpRateBps: number
  inviteeFirstTopUpMaxQuota: number
  inviterEnabled: boolean
  inviterTiers: ReferralTierDefaults[]
  inviterMaxQuota: number
  bonusDuration: string
  countRedeemAsValid: boolean
}

// toBps converts a percentage typed by the admin into basis points.
function toBps(percent: number): number {
  return Math.round(percent * 100)
}

// toPercent converts stored basis points back into a percentage for display.
function toPercent(rateBps: number): number {
  return rateBps / 100
}

function buildTierValue(tiers: Values['tiers']): string {
  return JSON.stringify(
    tiers.map((tier) => ({
      min_invitees: tier.minInvitees,
      rate_bps: toBps(tier.ratePercent),
    }))
  )
}

export function ReferralSettingsSection({
  defaultValues,
  complianceConfirmed = true,
}: {
  defaultValues: ReferralSettingsDefaults
  complianceConfirmed?: boolean
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const initialTiers: Values['tiers'] =
    defaultValues.inviterTiers.length > 0
      ? defaultValues.inviterTiers.map((tier) => ({
          minInvitees: tier.min_invitees,
          ratePercent: toPercent(tier.rate_bps),
        }))
      : [{ minInvitees: 1, ratePercent: 1 }]

  const formDefaults: Values = {
    enabled: defaultValues.enabled,
    inviteeEnabled: defaultValues.inviteeFirstTopUpEnabled,
    inviteeRatePercent: toPercent(defaultValues.inviteeFirstTopUpRateBps),
    inviteeMaxQuota: defaultValues.inviteeFirstTopUpMaxQuota,
    inviterEnabled: defaultValues.inviterEnabled,
    inviterMaxQuota: defaultValues.inviterMaxQuota,
    bonusDuration: defaultValues.bonusDuration,
    countRedeemAsValid: defaultValues.countRedeemAsValid,
    tiers: initialTiers,
  }

  const { form, handleSubmit, isDirty, isSubmitting } = useSettingsForm<Values>({
    resolver: zodResolver(schema) as unknown as Parameters<typeof useSettingsForm<Values>>[0]['resolver'],
    defaultValues: formDefaults,
    onSubmit: async (data) => {
      await updateOption.mutateAsync({
        key: 'referral_setting.enabled',
        value: String(data.enabled),
      })
      await updateOption.mutateAsync({
        key: 'referral_setting.invitee_first_topup_enabled',
        value: String(data.inviteeEnabled),
      })
      await updateOption.mutateAsync({
        key: 'referral_setting.invitee_first_topup_rate_bps',
        value: String(toBps(data.inviteeRatePercent)),
      })
      await updateOption.mutateAsync({
        key: 'referral_setting.invitee_first_topup_max_quota',
        value: String(data.inviteeMaxQuota),
      })
      await updateOption.mutateAsync({
        key: 'referral_setting.inviter_enabled',
        value: String(data.inviterEnabled),
      })
      await updateOption.mutateAsync({
        key: 'referral_setting.inviter_tiers',
        value: buildTierValue(data.tiers),
      })
      await updateOption.mutateAsync({
        key: 'referral_setting.inviter_max_quota',
        value: String(data.inviterMaxQuota),
      })
      await updateOption.mutateAsync({
        key: 'referral_setting.bonus_duration',
        value: data.bonusDuration,
      })
      await updateOption.mutateAsync({
        key: 'referral_setting.count_redeem_as_valid',
        value: String(data.countRedeemAsValid),
      })
      form.reset(data)
      toast.success(t('Referral settings saved'))
    },
  })

  const enabled = form.watch('enabled')
  const tiers = form.watch('tiers')
  const inviteeEnabled = form.watch('inviteeEnabled')
  const inviterEnabled = form.watch('inviterEnabled')

  return (
    <SettingsSection title={t('Referral Program')}>
      {!complianceConfirmed ? (
        <Alert variant='destructive'>
          <AlertDescription>
            {t(
              'The referral program pays out quota and requires compliance confirmation in Payment Gateway settings.'
            )}
          </AlertDescription>
        </Alert>
      ) : null}

      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={updateOption.isPending || isSubmitting}
            isSaveDisabled={!isDirty}
            saveLabel={t('Save referral settings')}
          />

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable referral program')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Reward inviters and invitees when an invited user completes their first top-up.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={updateOption.isPending || isSubmitting}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          {enabled && (
            <div className='flex flex-col gap-6'>
              <div className='grid gap-6 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='bonusDuration'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Reward validity')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder='720'
                          {...field}
                          disabled={updateOption.isPending || isSubmitting}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'How long reward bonus stays valid. A bare number is hours, so 720 is 30 days.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='countRedeemAsValid'
                  render={({ field }) => (
                    <SettingsSwitchItem>
                      <SettingsSwitchContent>
                        <FormLabel>
                          {t('Count redemption codes as valid')}
                        </FormLabel>
                        <FormDescription>
                          {t(
                            'When on, a redemption-code top-up also advances the inviter tier. Rewards are paid either way.'
                          )}
                        </FormDescription>
                      </SettingsSwitchContent>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                          disabled={updateOption.isPending || isSubmitting}
                        />
                      </FormControl>
                    </SettingsSwitchItem>
                  )}
                />
              </div>

              <div className='flex flex-col gap-4'>
                <FormField
                  control={form.control}
                  name='inviteeEnabled'
                  render={({ field }) => (
                    <SettingsSwitchItem>
                      <SettingsSwitchContent>
                        <FormLabel>{t('Invitee first top-up reward')}</FormLabel>
                        <FormDescription>
                          {t(
                            'Give the invited user a share of their own first top-up as bonus credit.'
                          )}
                        </FormDescription>
                      </SettingsSwitchContent>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                          disabled={updateOption.isPending || isSubmitting}
                        />
                      </FormControl>
                    </SettingsSwitchItem>
                  )}
                />
                {inviteeEnabled && (
                  <div className='grid gap-6 sm:grid-cols-2'>
                    <FormField
                      control={form.control}
                      name='inviteeRatePercent'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('Invitee reward rate (%)')}
                          </FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              step='0.01'
                              min={0}
                              {...field}
                              disabled={updateOption.isPending || isSubmitting}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Percentage of the first top-up credited back to the invitee.'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name='inviteeMaxQuota'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('Invitee reward cap (quota)')}
                          </FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              min={0}
                              {...field}
                              disabled={updateOption.isPending || isSubmitting}
                            />
                          </FormControl>
                          <FormDescription>
                            {t('0 means uncapped ({{formatted}})', {
                              formatted: formatQuota(field.value ?? 0),
                            })}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                )}
              </div>

              <div className='flex flex-col gap-4'>
                <FormField
                  control={form.control}
                  name='inviterEnabled'
                  render={({ field }) => (
                    <SettingsSwitchItem>
                      <SettingsSwitchContent>
                        <FormLabel>{t('Inviter commission')}</FormLabel>
                        <FormDescription>
                          {t(
                            'Pay the inviter a commission on each invitee top-up, at the rate their tier allows.'
                          )}
                        </FormDescription>
                      </SettingsSwitchContent>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                          disabled={updateOption.isPending || isSubmitting}
                        />
                      </FormControl>
                    </SettingsSwitchItem>
                  )}
                />

                {inviterEnabled && (
                  <div className='flex flex-col gap-4'>
                    <div className='flex flex-col gap-1'>
                      <span className='text-sm font-medium'>
                        {t('Commission tiers')}
                      </span>
                      <span className='text-muted-foreground text-xs'>
                        {t(
                          'The tier with the highest qualifying invitee count applies. Tiers are matched by valid invitee count.'
                        )}
                      </span>
                    </div>

                    {tiers.map((_tier, index) => (
                      <div
                        key={index}
                        className='grid grid-cols-[1fr_1fr_auto] items-end gap-3'
                      >
                        <FormField
                          control={form.control}
                          name={`tiers.${index}.minInvitees`}
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>
                                {t('Valid invitees at least')}
                              </FormLabel>
                              <FormControl>
                                <Input
                                  type='number'
                                  min={1}
                                  {...field}
                                  disabled={
                                    updateOption.isPending || isSubmitting
                                  }
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <FormField
                          control={form.control}
                          name={`tiers.${index}.ratePercent`}
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>{t('Rate (%)')}</FormLabel>
                              <FormControl>
                                <Input
                                  type='number'
                                  step='0.01'
                                  min={0}
                                  max={100}
                                  {...field}
                                  disabled={
                                    updateOption.isPending || isSubmitting
                                  }
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <Button
                          type='button'
                          variant='outline'
                          size='icon'
                          className='mb-0.5'
                          disabled={
                            tiers.length <= 1 ||
                            updateOption.isPending ||
                            isSubmitting
                          }
                          onClick={() =>
                            form.setValue(
                              'tiers',
                              tiers.filter((_, i) => i !== index),
                              { shouldDirty: true }
                            )
                          }
                          aria-label={t('Remove tier')}
                        >
                          <Trash2 className='size-4' />
                        </Button>
                      </div>
                    ))}

                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      className='self-start'
                      disabled={updateOption.isPending || isSubmitting}
                      onClick={() =>
                        form.setValue(
                          'tiers',
                          [...tiers, { minInvitees: 1, ratePercent: 0 }],
                          { shouldDirty: true }
                        )
                      }
                    >
                      <Plus className='size-4' />
                      {t('Add tier')}
                    </Button>

                    <FormField
                      control={form.control}
                      name='inviterMaxQuota'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Commission cap (quota)')}</FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              min={0}
                              {...field}
                              disabled={updateOption.isPending || isSubmitting}
                            />
                          </FormControl>
                          <FormDescription>
                            {t('0 means uncapped ({{formatted}})', {
                              formatted: formatQuota(field.value ?? 0),
                            })}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                )}
              </div>
            </div>
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
