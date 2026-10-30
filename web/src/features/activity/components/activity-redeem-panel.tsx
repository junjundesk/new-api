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
import { useQuery } from '@tanstack/react-query'
import { Gift, Loader2 } from 'lucide-react'
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { getActivityOffers } from '../api'
import { useActivity } from '../hooks/use-activity'
import type { ActivityPreview, RedeemActivityResult } from '../types'
import { ActivityOfferCards } from './activity-offer-cards'
import { ActivityRedeemConfirmDialog } from './dialogs/activity-redeem-confirm-dialog'

type ActivityRedeemPanelProps = {
  showOffers?: boolean
}

// 一次兑换可能覆盖多个分组：优先拼接后端返回的 group_names，旧接口回退 group_name。
function redeemedGroupLabel(result: RedeemActivityResult): string {
  const names = (result.group_names ?? []).filter((name) => name.trim() !== '')
  if (names.length > 0) return names.join(', ')
  return result.group_name
}

export function ActivityRedeemPanel({
  showOffers = false,
}: ActivityRedeemPanelProps) {
  const { t } = useTranslation()
  const { verifying, redeeming, verify, redeem } = useActivity()
  const [code, setCode] = useState('')
  const [preview, setPreview] = useState<ActivityPreview | null>(null)
  const [confirmOpen, setConfirmOpen] = useState(false)

  const { data: offersData } = useQuery({
    queryKey: ['activity-offers'],
    queryFn: async () => {
      const result = await getActivityOffers()
      if (!result.success) {
        toast.error(result.message || t('Failed to load activity offers'))
        return { enabled: false, offers: [] }
      }
      return result.data ?? { enabled: false, offers: [] }
    },
  })

  const offers = offersData?.offers ?? []

  const startRedeem = useCallback(
    async (inputCode: string) => {
      const { preview: verified, message } = await verify(inputCode)
      if (message || !verified) {
        toast.error(message || t('Invalid activity code'))
        return
      }
      if (!verified.redeemable) {
        toast.error(
          verified.message || t('This activity code is not redeemable now')
        )
        return
      }
      if (!verified.need_confirm) {
        const outcome = await redeem({
          code: verified.code,
          confirm: false,
          expectedPriceQuota: 0,
        })
        if (outcome.ok) {
          toast.success(
            t(
              'Redeemed {{group}}. Switch your token group to start using it.',
              {
                group: redeemedGroupLabel(outcome.data),
              }
            )
          )
          setCode('')
          return
        }
        toast.error(outcome.message)
        return
      }
      setPreview(verified)
      setConfirmOpen(true)
    },
    [redeem, t, verify]
  )

  const handleConfirm = useCallback(async () => {
    if (!preview) return
    const outcome = await redeem({
      code: preview.code,
      confirm: true,
      expectedPriceQuota: preview.price_quota,
    })
    if (outcome.ok) {
      toast.success(
        t('Redeemed {{group}}. Switch your token group to start using it.', {
          group: redeemedGroupLabel(outcome.data),
        })
      )
      setConfirmOpen(false)
      setPreview(null)
      setCode('')
      return
    }
    // 价格变动等情况由后端返回最新预览，刷新弹窗内容后请用户重新确认。
    if (outcome.preview) {
      setPreview(outcome.preview)
    } else {
      setConfirmOpen(false)
    }
    toast.error(outcome.message)
  }, [preview, redeem, t])

  const isBusy = verifying || redeeming

  return (
    <div className='space-y-3'>
      <div className='grid grid-cols-[minmax(0,1fr)_auto] gap-2'>
        <Input
          id='activity-code'
          value={code}
          onChange={(event) => setCode(event.target.value)}
          placeholder={t('Enter your activity code')}
          className='h-9 min-w-0'
        />
        <Button
          disabled={isBusy}
          variant='outline'
          className='h-9 px-4'
          onClick={() => void startRedeem(code)}
        >
          {isBusy && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
          {t('Verify')}
        </Button>
      </div>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Free activity codes are redeemed right away; paid ones ask for confirmation first.'
        )}
      </p>

      {showOffers && offers.length > 0 && (
        <div className='space-y-3 border-t pt-3'>
          <div className='flex items-center gap-2'>
            <Gift className='text-muted-foreground h-4 w-4' />
            <Label className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
              {t('Available Activities')}
            </Label>
          </div>
          <ActivityOfferCards
            offers={offers}
            disabled={isBusy}
            onRedeem={(offer) => {
              setCode(offer.code)
              void startRedeem(offer.code)
            }}
          />
        </div>
      )}

      <ActivityRedeemConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        preview={preview}
        redeeming={redeeming}
        onConfirm={() => void handleConfirm()}
      />
    </div>
  )
}
