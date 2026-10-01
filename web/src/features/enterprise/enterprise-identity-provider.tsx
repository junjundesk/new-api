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
import { ShieldCheck } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { useThemeCustomization } from '@/context/theme-customization-provider'
import { ENTERPRISE_PRESET } from '@/features/enterprise/lib'
import { useAuthStore } from '@/stores/auth-store'

/**
 * Applies the enterprise black-and-gold identity for unlocked accounts.
 *
 * - On the first mount where the account is unlocked, switches the active
 *   preset to `enterprise` and shows a one-time gold welcome toast
 *   ("Enterprise identity recognized").
 * - Subsequent visits do not override a preset the user picked manually, but
 *   the toast is shown once per browser session.
 *
 * Renders nothing; it only drives side effects.
 */
export function EnterpriseIdentityProvider({
  children,
}: {
  children?: React.ReactNode
}) {
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.auth.user)
  const { customization, setPreset } = useThemeCustomization()
  const unlocked = user?.enterprise_unlocked === true
  const announcedRef = useRef(false)

  useEffect(() => {
    if (!unlocked) return

    // Apply the gold theme once per browser session. The guard keeps a
    // deliberate later preset choice from being clobbered on every mount.
    const appliedKey = `enterprise_theme_applied_${user?.id ?? 'anon'}`
    let alreadyApplied = false
    try {
      alreadyApplied = window.sessionStorage.getItem(appliedKey) === 'true'
    } catch {
      /* storage may be unavailable; fall through and apply once */
    }
    if (!alreadyApplied && customization.preset !== ENTERPRISE_PRESET) {
      setPreset(ENTERPRISE_PRESET)
    }
    try {
      window.sessionStorage.setItem(appliedKey, 'true')
    } catch {
      /* ignore */
    }
  }, [unlocked, user?.id, customization.preset, setPreset])

  useEffect(() => {
    if (!unlocked || announcedRef.current) return
    announcedRef.current = true
    toast.custom(
      (id) => (
        <div
          className='border-primary/60 bg-popover text-popover-foreground flex w-[min(92vw,26rem)] items-center gap-3 rounded-full border px-4 py-2.5 shadow-lg'
          onClick={() => toast.dismiss(id)}
          role='status'
        >
          <ShieldCheck className='text-primary size-5 shrink-0' />
          <span className='text-sm font-medium'>
            {t('Enterprise identity recognized, have a great day')}
          </span>
        </div>
      ),
      { duration: 5000 }
    )
  }, [unlocked, t])

  return children
}
