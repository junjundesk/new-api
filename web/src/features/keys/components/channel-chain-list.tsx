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
import { ArrowRight, Pencil, Trash2 } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

import {
  countChannelChainModels,
  normalizeChannelChainType,
} from '../lib/user-channel-chain'
import type { ChannelChain } from '../types'

type ChannelChainListProps = {
  chains: ChannelChain[]
  tokenUsage: Record<string, number>
  onEdit: (chain: ChannelChain) => void
  onDelete: (chain: ChannelChain) => void
}

export function ChannelChainList(props: ChannelChainListProps) {
  const { t } = useTranslation()

  if (props.chains.length === 0) {
    return (
      <p className='text-muted-foreground py-6 text-center text-sm'>
        {t('You have not created any group chains yet.')}
      </p>
    )
  }

  return (
    <ul className='space-y-2'>
      {props.chains.map((chain) => {
        const type = normalizeChannelChainType(chain.type)
        const groups = chain.groups ?? []
        const items = chain.items ?? []
        const isModels = type === 'models'
        const modelCount = countChannelChainModels(chain)
        const summaryCount = isModels ? modelCount : groups.length
        const summary = isModels
          ? t('{{count}} models', { count: summaryCount })
          : t('{{count}} groups', { count: summaryCount })
        let detail: ReactNode
        if (isModels) {
          if (items.length === 0) {
            detail = (
              <p className='text-destructive text-xs'>
                {t('This chain has no model rows left. Edit it or delete it.')}
              </p>
            )
          } else {
            detail = (
              <div className='flex flex-wrap items-center gap-1.5'>
                {items.map((item) => {
                  const label = `${item.group || '-'}: ${(item.models ?? []).join(', ')}`
                  return (
                    <span
                      key={`${chain.id}-${item.group}`}
                      className='bg-muted inline-flex max-w-full min-w-0 items-center gap-1 rounded-md px-2 py-1 text-xs'
                      title={label}
                    >
                      <span className='max-w-[10rem] truncate'>
                        {item.group || '-'}
                      </span>
                      <Badge
                        variant='outline'
                        className='h-4 shrink-0 px-1 text-[10px]'
                      >
                        {t('{{count}} models', {
                          count: (item.models ?? []).length,
                        })}
                      </Badge>
                    </span>
                  )
                })}
              </div>
            )
          }
        } else if (groups.length === 0) {
          detail = (
            <p className='text-destructive text-xs'>
              {t('This chain has no groups left. Edit it or delete it.')}
            </p>
          )
        } else {
          detail = (
            <div className='flex min-w-0 flex-wrap items-center gap-1.5'>
              {groups.map((group, index) => (
                <span
                  key={`${chain.id}-${group}`}
                  className='inline-flex min-w-0 items-center gap-1'
                >
                  {index > 0 ? (
                    <ArrowRight
                      aria-hidden='true'
                      className='text-muted-foreground size-3 shrink-0'
                    />
                  ) : null}
                  <Badge variant='secondary' className='max-w-[12rem] truncate'>
                    {group}
                  </Badge>
                </span>
              ))}
            </div>
          )
        }
        return (
          <li
            key={chain.id}
            className='space-y-2 rounded-lg border p-3'
            data-channel-chain-id={chain.id}
            data-channel-chain-type={type}
          >
            <div className='flex min-w-0 items-start justify-between gap-2'>
              <div className='min-w-0 flex-1'>
                <div className='flex min-w-0 flex-wrap items-center gap-2'>
                  <span
                    className='block min-w-0 truncate font-medium'
                    title={chain.name}
                  >
                    {chain.name}
                  </span>
                  <Badge variant='outline' className='shrink-0'>
                    {isModels ? t('Models') : t('Groups')}
                  </Badge>
                  <span className='text-muted-foreground shrink-0 text-xs'>
                    {summary}
                  </span>
                </div>
                <p className='text-muted-foreground mt-1 text-xs'>
                  {t('Bound API keys: {{count}}', {
                    count: props.tokenUsage[`chain:${chain.id}`] ?? 0,
                  })}
                </p>
              </div>
              <div className='flex shrink-0 items-center gap-1'>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='size-8'
                  aria-label={`${t('Edit chain')} ${chain.name}`}
                  onClick={() => props.onEdit(chain)}
                >
                  <Pencil aria-hidden='true' className='size-4' />
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='text-destructive size-8'
                  aria-label={`${t('Delete chain')} ${chain.name}`}
                  onClick={() => props.onDelete(chain)}
                >
                  <Trash2 aria-hidden='true' className='size-4' />
                </Button>
              </div>
            </div>

            {detail}
          </li>
        )
      })}
    </ul>
  )
}
