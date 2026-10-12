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
import i18next from 'i18next'
import { useState, useEffect, useCallback } from 'react'
import { toast } from 'sonner'

import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getSelf } from '@/lib/api'

import { getAffiliateCode, getReferralSummary, transferAffiliateQuota } from '../api'
import { generateAffiliateLink } from '../lib'

import type { ReferralSummary } from '../types'

// ============================================================================
// Affiliate Hook
// ============================================================================

export function useAffiliate() {
  const [affiliateCode, setAffiliateCode] = useState<string>('')
  const [affiliateLink, setAffiliateLink] = useState<string>('')
  const [summary, setSummary] = useState<ReferralSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [transferring, setTransferring] = useState(false)
  const { copyToClipboard } = useCopyToClipboard()

  // Fetch affiliate code and referral program status
  const fetchAffiliateCode = useCallback(async () => {
    try {
      setLoading(true)
      const [codeResponse, summaryResponse] = await Promise.all([
        getAffiliateCode(),
        getReferralSummary(),
      ])

      if (codeResponse.success && codeResponse.data) {
        setAffiliateCode(codeResponse.data)
        const link = generateAffiliateLink(codeResponse.data)
        setAffiliateLink(link)
      }
      if (summaryResponse.success && summaryResponse.data) {
        setSummary(summaryResponse.data)
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch affiliate code:', error)
    } finally {
      setLoading(false)
    }
  }, [])

  // Copy affiliate link
  const copyAffiliateLink = useCallback(() => {
    copyToClipboard(affiliateLink)
  }, [affiliateLink, copyToClipboard])

  // Transfer affiliate quota to balance
  const transferQuota = useCallback(async (quota: number): Promise<boolean> => {
    try {
      setTransferring(true)
      const response = await transferAffiliateQuota({ quota })

      if (response.success) {
        toast.success(response.message || i18next.t('Transfer successful'))
        await getSelf()
        return true
      }

      toast.error(response.message || i18next.t('Transfer failed'))
      return false
    } catch (_error) {
      toast.error(i18next.t('Transfer failed'))
      return false
    } finally {
      setTransferring(false)
    }
  }, [])

  useEffect(() => {
    fetchAffiliateCode()
  }, [fetchAffiliateCode])

  return {
    affiliateCode,
    affiliateLink,
    summary,
    loading,
    transferring,
    copyAffiliateLink,
    transferQuota,
    refetch: fetchAffiliateCode,
  }
}
