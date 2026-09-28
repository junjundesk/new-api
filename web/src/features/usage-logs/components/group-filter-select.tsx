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

import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import {
  GROUP_FILTER_ALL_VALUE,
  buildGroupFilterOptions,
} from '../lib/group-filter'

interface GroupFilterSelectProps {
  value?: string
  groups?: readonly string[] | Record<string, unknown>
  onChange: (group: string | undefined) => void
}

export function GroupFilterSelect(props: GroupFilterSelectProps) {
  const { t } = useTranslation()
  const allLabel = t('All Groups')
  const options = useMemo(() => {
    const base = buildGroupFilterOptions(props.groups, allLabel)
    if (props.value && !base.some((option) => option.value === props.value)) {
      base.push({ value: props.value, label: props.value })
    }
    return base
  }, [props.groups, props.value, allLabel])

  const selectedValue = props.value || GROUP_FILTER_ALL_VALUE
  const selectedLabel =
    options.find((option) => option.value === selectedValue)?.label ?? allLabel

  return (
    <Select
      items={options}
      value={selectedValue}
      onValueChange={(value) => {
        const next = typeof value === 'string' && value ? value : GROUP_FILTER_ALL_VALUE
        props.onChange(next === GROUP_FILTER_ALL_VALUE ? undefined : next)
      }}
    >
      <SelectTrigger>
        <SelectValue>{selectedLabel}</SelectValue>
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          {options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}
