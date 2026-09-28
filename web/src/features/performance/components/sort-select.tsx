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
import { useTranslation } from 'react-i18next'

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { GROUP_SORT_OPTIONS, type GroupSortKey } from '../lib/sort'

export type SortSelectProps = {
  value: GroupSortKey
  onChange: (value: GroupSortKey) => void
}

export function SortSelect(props: SortSelectProps) {
  const { t } = useTranslation()
  const current =
    GROUP_SORT_OPTIONS.find((option) => option.value === props.value) ??
    GROUP_SORT_OPTIONS[1]
  return (
    <Select
      value={props.value}
      onValueChange={(value) => {
        const next = GROUP_SORT_OPTIONS.find((option) => option.value === value)
        if (next) props.onChange(next.value)
      }}
    >
      <SelectTrigger className='w-52' aria-label={t('Sort groups')}>
        <SelectValue>{t(current.labelKey)}</SelectValue>
      </SelectTrigger>
      <SelectContent>
        {GROUP_SORT_OPTIONS.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            {t(option.labelKey)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
