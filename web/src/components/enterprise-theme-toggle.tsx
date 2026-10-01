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
import { Gem } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { useThemeCustomization } from '@/context/theme-customization-provider'
import { ENTERPRISE_PRESET } from '@/features/enterprise/lib'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

/**
 * Header toggle that switches the enterprise black-and-gold theme on and off.
 *
 * Only rendered for accounts that have unlocked the enterprise identity, so the
 * gold accent is never offered to regular users.
 */
export function EnterpriseThemeToggle() {
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.auth.user)
  const { customization, setPreset, defaults } = useThemeCustomization()

  if (user?.enterprise_unlocked !== true) return null

  const active = customization.preset === ENTERPRISE_PRESET

  return (
    <Button
      size='icon'
      variant='ghost'
      aria-label={t('Enterprise black-gold theme')}
      aria-pressed={active}
      title={t('Enterprise black-gold theme')}
      className={cn('max-md:hidden', active && 'text-primary')}
      onClick={() => setPreset(active ? defaults.preset : ENTERPRISE_PRESET)}
    >
      <Gem className='size-[1.2rem]' aria-hidden='true' />
    </Button>
  )
}
