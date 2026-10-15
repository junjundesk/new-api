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
import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useState } from 'react'

import { getSelf } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'

import { redeemActivityCode, verifyActivityCode } from '../api'
import type { ActivityPreview, RedeemOutcome } from '../types'

// ============================================================================
// Activity Hook
// ============================================================================

export function useActivity() {
  const queryClient = useQueryClient()
  const [verifying, setVerifying] = useState(false)
  const [redeeming, setRedeeming] = useState(false)

  const refresh = useCallback(async () => {
    await getSelf()
    await queryClient.invalidateQueries({ queryKey: ['activity-offers'] })
    await queryClient.invalidateQueries({ queryKey: ['activity-grants'] })
  }, [queryClient])

  // 校验活动码：成功返回预览（付费码由调用方弹窗确认），失败返回错误文案。
  const verify = useCallback(
    async (
      code: string
    ): Promise<{ preview?: ActivityPreview; message?: string }> => {
      if (!code.trim()) {
        return { message: 'Please enter an activity code' }
      }
      setVerifying(true)
      try {
        const result = await verifyActivityCode(code.trim())
        if (!result.success || !result.data) {
          return { message: result.message }
        }
        return { preview: result.data }
      } catch (error) {
        handleServerError(error)
        return {}
      } finally {
        setVerifying(false)
      }
    },
    []
  )

  // 兑换活动码。后端在付费场景会要求 confirm=true 并核对价格。
  const redeem = useCallback(
    async (input: {
      code: string
      confirm: boolean
      expectedPriceQuota: number
    }): Promise<RedeemOutcome> => {
      setRedeeming(true)
      try {
        const result = await redeemActivityCode(input)
        if (!result.success || !result.data) {
          const preview = result.data as unknown as
            | ActivityPreview
            | undefined
          return {
            ok: false,
            message: result.message || 'Redemption failed',
            preview:
              preview && typeof preview === 'object' && 'price_quota' in preview
                ? preview
                : undefined,
          }
        }
        await refresh()
        return { ok: true, data: result.data }
      } catch (error) {
        handleServerError(error)
        return { ok: false, message: 'Redemption failed' }
      } finally {
        setRedeeming(false)
      }
    },
    [refresh]
  )

  return { verifying, redeeming, verify, redeem, refresh }
}
