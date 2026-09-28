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
import { ChevronDown, ChevronUp } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

export type GroupMoveControlsProps = {
  groupName: string
  disabledUp: boolean
  disabledDown: boolean
  onMove: (delta: number) => void
}

export function GroupMoveControls(props: GroupMoveControlsProps) {
  const { t } = useTranslation()
  return (
    <div className='flex items-center justify-end gap-2'>
      <Button
        type='button'
        size='sm'
        variant='outline'
        disabled={props.disabledUp}
        aria-label={t('Move {{group}} up', { group: props.groupName })}
        onClick={() => props.onMove(-1)}
      >
        <ChevronUp className='size-3.5' aria-hidden />
        {t('Move up')}
      </Button>
      <Button
        type='button'
        size='sm'
        variant='outline'
        disabled={props.disabledDown}
        aria-label={t('Move {{group}} down', { group: props.groupName })}
        onClick={() => props.onMove(1)}
      >
        <ChevronDown className='size-3.5' aria-hidden />
        {t('Move down')}
      </Button>
    </div>
  )
}
