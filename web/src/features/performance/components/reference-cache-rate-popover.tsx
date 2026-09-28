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
import { useId } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'

import {
  MAX_REFERENCE_CACHE_RATE_PERCENT,
  parseReferenceCacheRate,
} from '../lib/reference-cache-rate'

export type ReferenceCacheRatePopoverProps = {
  value: string
  onChange: (value: string) => void
}

export function ReferenceCacheRatePopover(
  props: ReferenceCacheRatePopoverProps
) {
  const { t } = useTranslation()
  const inputId = useId()
  const valid = parseReferenceCacheRate(props.value) !== null
  return (
    <Popover>
      <PopoverTrigger
        render={<Button variant='outline' size='sm' aria-invalid={!valid} />}
      >
        {valid
          ? t('Reference cache: {{rate}}%', { rate: props.value })
          : t('Reference cache rate')}
      </PopoverTrigger>
      <PopoverContent align='end' className='w-96 gap-3 text-sm'>
        <p className='font-medium'>{t('Reference cache rate')}</p>
        <div className='flex flex-col gap-1.5'>
          <Label htmlFor={inputId}>{t('Reference cache rate (%)')}</Label>
          <Input
            id={inputId}
            type='number'
            min={0}
            max={MAX_REFERENCE_CACHE_RATE_PERCENT}
            step='any'
            value={props.value}
            aria-invalid={!valid}
            onChange={(event) => props.onChange(event.target.value)}
          />
        </div>
        <div className='text-muted-foreground flex flex-col gap-2 text-xs'>
          <p>
            {t(
              '90% is an adjustable multi-turn coding scenario, not a guaranteed official cache rate. OpenAI documents examples above 90%, with results depending on the workload.'
            )}
          </p>
          <p>
            {t(
              'Changing this reference only changes the estimate, not your billing.'
            )}
          </p>
          <p>
            {t(
              'The reference changes cache reads for non-write input. Actual cache-write costs stay fixed; Coding cache percentages include writes in total input.'
            )}
          </p>
        </div>
      </PopoverContent>
    </Popover>
  )
}
