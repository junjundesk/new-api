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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { formatQuota, parseQuotaFromDollars } from '@/lib/format'

import { grantAllUsersBonus, grantUserBonus } from '../api'

interface BonusGrantDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Target user for a single grant; omit to grant to all users. */
  userId?: number
  username?: string
  onSuccess?: () => void
}

/**
 * Admin dialog for granting bonus (赠金) to one user or every user.
 * The duration field defaults to hours: "1" = 1 hour, "1天" = 24 hours.
 */
export function BonusGrantDialog(props: BonusGrantDialogProps) {
  const { t } = useTranslation()
  const [amount, setAmount] = useState('')
  const [expireInput, setExpireInput] = useState('24')
  const [remark, setRemark] = useState('')
  const [sendEmail, setSendEmail] = useState(true)
  const [loading, setLoading] = useState(false)

  const amountValue = parseFloat(amount) || 0
  const quotaValue = parseQuotaFromDollars(amountValue)
  const allUsers = !props.userId

  const reset = () => {
    setAmount('')
    setExpireInput('24')
    setRemark('')
    setSendEmail(true)
  }

  const handleConfirm = async () => {
    if (quotaValue <= 0) {
      toast.error(t('Please enter a valid bonus amount'))
      return
    }
    setLoading(true)
    try {
      const payload = {
        amount: quotaValue,
        expire_input: expireInput.trim(),
        remark: remark.trim(),
        send_email: sendEmail,
      }
      if (allUsers) {
        const result = await grantAllUsersBonus({ ...payload, all_users: true })
        if (result.success) {
          toast.success(
            t('Bonus granted to {{count}} users', {
              count: result.data?.granted ?? 0,
            })
          )
          reset()
          props.onOpenChange(false)
          props.onSuccess?.()
        } else {
          toast.error(result.message || t('Failed to grant bonus'))
        }
        return
      }
      const result = await grantUserBonus(props.userId as number, payload)
      if (result.success) {
        toast.success(t('Bonus granted successfully'))
        reset()
        props.onOpenChange(false)
        props.onSuccess?.()
      } else {
        toast.error(result.message || t('Failed to grant bonus'))
      }
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : t('Failed to grant bonus'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Grant Bonus')}
      description={
        allUsers
          ? t('Grant bonus to all users')
          : t('Grant bonus to {{username}}', {
              username: props.username ?? '',
            })
      }
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={loading}>
            {loading ? t('Granting...') : t('Grant')}
          </Button>
        </>
      }
    >
      <div className='space-y-2'>
        <Label>{t('Bonus Amount')}</Label>
        <Input
          type='number'
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          placeholder={t('Enter amount')}
        />
        <p className='text-muted-foreground text-xs'>
          {t('Parsed quota: {{quota}}', {
            quota: formatQuota(quotaValue),
          })}
        </p>
      </div>

      <div className='space-y-2'>
        <Label>{t('Validity Period')}</Label>
        <Input
          value={expireInput}
          onChange={(e) => setExpireInput(e.target.value)}
          placeholder='24'
        />
        <p className='text-muted-foreground text-xs'>
          {t(
            'Default unit is hours: "1" = 1 hour, "1d" = 24 hours. Leave empty to never expire.'
          )}
        </p>
      </div>

      <div className='space-y-2'>
        <Label>{t('Remark')}</Label>
        <Input
          value={remark}
          onChange={(e) => setRemark(e.target.value)}
          placeholder={t('Optional note shown to the user')}
        />
      </div>

      <div className='flex items-start gap-2'>
        <Checkbox
          id='bonus-send-email'
          checked={sendEmail}
          onCheckedChange={(checked) => setSendEmail(checked === true)}
          className='mt-0.5'
        />
        <Label
          htmlFor='bonus-send-email'
          className='cursor-pointer text-sm font-normal'
        >
          {t('Send arrival notification email')}
        </Label>
      </div>
    </Dialog>
  )
}
