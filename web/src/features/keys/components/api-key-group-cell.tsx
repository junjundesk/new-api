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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { BadgeCell, TruncatedCell } from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { StatusBadge } from '@/components/status-badge'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { updateApiKeyGroup } from '../api'
import {
  AutoGroupBadge,
  GroupRatioBadge,
  type GroupRatio,
} from './auto-group-visuals'

type GroupOption = {
  value: string
  label: string
  ratio?: GroupRatio
  chainType?: 'groups' | 'models'
  chainModelCount?: number
  chainGroupCount?: number
}

type ApiKeyGroupCellProps = {
  apiKeyId?: number
  crossGroupRetry: boolean
  group: string
  label?: string
  isChain?: boolean
  chainGroups?: string[]
  chainType?: 'groups' | 'models'
  chainItems?: { group: string; models: string[] }[]
  fallbackEnabled?: boolean
  onGroupChanged?: () => void
  ratio?: GroupRatio
  options?: GroupOption[]
  shouldReduceMotion: boolean
}

type ChainSummaryBadgeProps = {
  type?: 'groups' | 'models'
  modelCount?: number
  groupCount?: number
}

function ChainSummaryBadge(props: ChainSummaryBadgeProps) {
  const { t } = useTranslation()

  if (!props.type) return null
  const label =
    props.type === 'models'
      ? t('{{count}} models', { count: props.modelCount ?? 0 })
      : t('{{count}} groups', { count: props.groupCount ?? 0 })

  return (
    <Badge
      variant='outline'
      className='text-muted-foreground h-[18px] shrink-0 px-1.5 text-[10px] leading-none font-medium'
    >
      {label}
    </Badge>
  )
}

function GroupCellContent(props: ApiKeyGroupCellProps) {
  const { t } = useTranslation()

  if (props.isChain) {
    const ratio = typeof props.ratio === 'number' ? props.ratio : undefined
    const chainLabel = props.label || props.group
    const chainGroups = (props.chainGroups || []).join(' -> ')
    const modelRows = (props.chainItems || []).map(
      (item) => `${item.group}: ${item.models.join(', ')}`
    )
    const detail =
      props.chainType === 'models' && modelRows.length > 0
        ? modelRows.join('\n')
        : chainGroups
    const modelCount =
      props.chainType === 'models'
        ? new Set((props.chainItems || []).flatMap((item) => item.models)).size
        : undefined
    return (
      <TruncatedCell
        className='-ml-1.5'
        tooltipContent={detail ? `${chainLabel}\n${detail}` : chainLabel}
        tooltipClassName='max-h-80 break-all whitespace-pre-line overflow-y-auto'
      >
        <span
          data-api-key-group-cell='chain'
          className='flex min-w-0 items-center gap-1.5'
        >
          <GroupBadge label={chainLabel} ratio={ratio} />
          <ChainSummaryBadge
            type={props.chainType}
            modelCount={modelCount}
            groupCount={(props.chainGroups || []).length}
          />
        </span>
      </TruncatedCell>
    )
  }

  if (props.group !== 'auto') {
    const ratio = typeof props.ratio === 'number' ? props.ratio : undefined
    return (
      <TruncatedCell
        className='-ml-1.5'
        tooltipContent={props.group || '-'}
        tooltipClassName='break-all'
      >
        <GroupBadge group={props.group} ratio={ratio} />
      </TruncatedCell>
    )
  }

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <BadgeCell
            data-api-key-group-cell='auto'
            className='gap-1.5 overflow-visible text-xs'
          />
        }
      >
        <StatusBadge label={t('Cross-group')} variant='info' copyable={false} />
        <AutoGroupBadge shouldReduceMotion={props.shouldReduceMotion} />
        <GroupRatioBadge
          ratio={props.ratio}
          isAuto
          shouldReduceMotion={props.shouldReduceMotion}
        />
      </TooltipTrigger>
      <TooltipContent>
        <span className='text-xs'>
          {t(
            'Automatically selects the best available group with circuit breaker mechanism'
          )}
        </span>
      </TooltipContent>
    </Tooltip>
  )
}

export function ApiKeyGroupCell(props: ApiKeyGroupCellProps) {
  const { t } = useTranslation()
  const [updating, setUpdating] = useState(false)
  const selectOptions = useMemo(() => {
    const options = [...(props.options ?? [])]
    if (
      props.group &&
      !options.some((option) => option.value === props.group)
    ) {
      options.push({
        value: props.group,
        label: props.label || props.group,
        ratio: props.ratio,
      })
    }
    if (!props.group && !options.some((option) => option.value === '')) {
      options.unshift({ value: '', label: t('User Group') })
    }
    return options
  }, [props.group, props.label, props.options, props.ratio, t])

  const handleChange = async (value: string | null) => {
    if (value === props.group || updating) return
    if (!props.apiKeyId || !value) return
    const nextOption = selectOptions.find((option) => option.value === value)
    if (nextOption?.chainType === 'models' && props.fallbackEnabled) {
      toast.error(
        t(
          'Model-based group chains do not support cross-model fallback. Disable fallback before selecting this chain; your draft will be kept.'
        )
      )
      return
    }
    setUpdating(true)
    try {
      const result = await updateApiKeyGroup(
        props.apiKeyId,
        value,
        value === 'auto'
      )
      if (result.success) {
        toast.success(t('Group updated'))
        props.onGroupChanged?.()
      } else {
        toast.error(result.message || t('An unexpected error occurred'))
      }
    } catch {
      toast.error(t('An unexpected error occurred'))
    } finally {
      setUpdating(false)
    }
  }

  if (!props.options || props.options.length === 0) {
    return <GroupCellContent {...props} />
  }

  const currentLabel =
    selectOptions.find((option) => option.value === props.group)?.label ??
    props.label ??
    props.group ??
    t('User Group')
  const currentOption = selectOptions.find(
    (option) => option.value === props.group
  )

  return (
    <Select value={props.group} onValueChange={handleChange}>
      <SelectTrigger
        size='sm'
        disabled={updating}
        className='hover:bg-muted/60 h-7 max-w-full min-w-28 border-transparent bg-transparent px-1.5'
      >
        <SelectValue placeholder={currentLabel}>
          <span className='flex min-w-0 items-center gap-1.5'>
            <span className='truncate'>{currentLabel}</span>
            <ChainSummaryBadge
              type={currentOption?.chainType}
              modelCount={currentOption?.chainModelCount}
              groupCount={currentOption?.chainGroupCount}
            />
            {currentOption?.ratio != null && (
              <GroupRatioBadge
                ratio={currentOption.ratio}
                isAuto={currentOption.value === 'auto'}
                shouldReduceMotion={props.shouldReduceMotion}
              />
            )}
          </span>
        </SelectValue>
      </SelectTrigger>
      <SelectContent className='min-w-72'>
        <SelectGroup>
          {selectOptions.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              <span className='min-w-0 flex-1 truncate'>{option.label}</span>
              <ChainSummaryBadge
                type={option.chainType}
                modelCount={option.chainModelCount}
                groupCount={option.chainGroupCount}
              />
              <GroupRatioBadge
                ratio={option.ratio}
                isAuto={option.value === 'auto'}
                shouldReduceMotion={props.shouldReduceMotion}
              />
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}
