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
import {
  Download,
  Folder,
  FolderOpen,
  FolderPlus,
  Layers3,
  Pencil,
  Ticket,
  Upload,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { RedemptionCategory } from '../types'
import { RedemptionCategoryConfigDialog } from './redemption-category-config-dialog'
import { RedemptionCategoryDialog } from './redemption-category-dialog'
import { useRedemptions } from './redemptions-provider'
import { WalletTierVisibilityDialog } from './wallet-tier-visibility-dialog'

type CategoryPanelProps = {
  onSelect: (id?: number) => void
  isLoading: boolean
  isError: boolean
  retry: () => void
}

export function RedemptionCategoryPanel(props: CategoryPanelProps) {
  const { t } = useTranslation()
  const { categories, bonusRules, categoryId } = useRedemptions()
  const bonusPercent = (id: number) => {
    const rule = bonusRules.find((entry) => entry.category_id === id)
    return rule ? ` · +${rule.bonus_rate_bps / 100}%` : ''
  }
  const [editing, setEditing] = useState<RedemptionCategory | 'new' | null>(
    null
  )
  const [configMode, setConfigMode] = useState<'export' | 'import' | null>(null)
  const current = categories.find((category) => category.id === categoryId)
  const categoryLabel = t('Code categories')
  const rowClass =
    'flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-start text-sm hover:bg-muted focus-visible:outline-ring aria-pressed:bg-primary/10 aria-pressed:text-primary'

  return (
    <aside
      aria-label={categoryLabel}
      className='bg-card flex shrink-0 flex-col gap-2 rounded-xl border p-3 lg:w-56 xl:w-64'
    >
      <div className='flex items-center justify-between gap-2'>
        <h3 className='flex items-center gap-2 text-sm font-semibold'>
          <Layers3 aria-hidden className='text-muted-foreground size-4' />
          {categoryLabel}
        </h3>
        <Button
          variant='outline'
          size='sm'
          aria-label={t('New category')}
          title={t('New category')}
          onClick={() => setEditing('new')}
        >
          <FolderPlus />
          <span className='hidden lg:inline'>{t('New')}</span>
          <span className='lg:hidden'>{t('New category')}</span>
        </Button>
      </div>
      <div className='grid grid-cols-2 gap-2'>
        <Button
          variant='outline'
          size='sm'
          onClick={() => setConfigMode('export')}
        >
          <Download aria-hidden />
          {t('Export JSON')}
        </Button>
        <Button
          variant='outline'
          size='sm'
          onClick={() => setConfigMode('import')}
        >
          <Upload aria-hidden />
          {t('Import JSON')}
        </Button>
      </div>
      <WalletTierVisibilityDialog />
      <div className='flex items-center gap-2 lg:hidden'>
        <NativeSelect
          aria-label={categoryLabel}
          className='min-w-0 flex-1'
          value={categoryId ?? ''}
          onChange={(event) =>
            props.onSelect(
              event.target.value === '' ? undefined : Number(event.target.value)
            )
          }
        >
          <NativeSelectOption value=''>{t('All codes')}</NativeSelectOption>
          <NativeSelectOption value={0}>
            {t('Uncategorized')}
          </NativeSelectOption>
          {categories.map((category) => (
            <NativeSelectOption key={category.id} value={category.id}>
              {category.name} · {formatQuota(category.quota)}
              {bonusPercent(category.id)}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        {current && (
          <Button
            variant='outline'
            size='icon-sm'
            onClick={() => setEditing(current)}
            aria-label={t('Edit category')}
          >
            <Pencil />
          </Button>
        )}
      </div>
      <nav
        aria-label={categoryLabel}
        className='hidden min-h-0 flex-1 flex-col gap-1 overflow-y-auto lg:flex'
      >
        <button
          type='button'
          className={rowClass}
          aria-pressed={categoryId === undefined}
          onClick={() => props.onSelect()}
        >
          <Ticket aria-hidden className='size-4' />
          {t('All codes')}
        </button>
        <button
          type='button'
          className={rowClass}
          aria-pressed={categoryId === 0}
          onClick={() => props.onSelect(0)}
        >
          <FolderOpen aria-hidden className='size-4' />
          {t('Uncategorized')}
        </button>
        {categories.length > 0 && <div className='my-1 border-t' />}
        {categories.map((category) => (
          <div
            key={category.id}
            className={cn(
              'group flex items-center gap-1 rounded-lg',
              categoryId === category.id && 'bg-primary/10'
            )}
          >
            <button
              type='button'
              aria-pressed={categoryId === category.id}
              onClick={() => props.onSelect(category.id)}
              className={cn(
                rowClass,
                'min-w-0 flex-1 aria-pressed:bg-transparent'
              )}
            >
              <Folder aria-hidden className='size-4 shrink-0' />
              <span className='min-w-0'>
                <span
                  className='block truncate font-medium'
                  title={category.name}
                >
                  {category.name}
                </span>
                <span className='text-muted-foreground block text-xs tabular-nums'>
                  {formatQuota(category.quota)}
                  {bonusPercent(category.id)}
                </span>
              </span>
            </button>
            <Button
              variant='ghost'
              size='icon-sm'
              className='text-muted-foreground mr-1 shrink-0'
              aria-label={t('Edit {{name}}', { name: category.name })}
              onClick={() => setEditing(category)}
            >
              <Pencil />
            </Button>
          </div>
        ))}
        {props.isLoading && <Skeleton className='mt-2 h-12 w-full' />}
        {!categories.length && !props.isLoading && !props.isError && (
          <p className='text-muted-foreground mt-2 border-t px-2 pt-3 text-xs'>
            {t('No categories yet')}
          </p>
        )}
      </nav>
      {props.isError && (
        <Button variant='outline' size='sm' onClick={props.retry}>
          {t('Reload categories')}
        </Button>
      )}
      {configMode && (
        <RedemptionCategoryConfigDialog
          mode={configMode}
          onClose={() => setConfigMode(null)}
        />
      )}
      {editing && (
        <RedemptionCategoryDialog
          category={editing === 'new' ? undefined : editing}
          onClose={() => setEditing(null)}
          onSelect={props.onSelect}
        />
      )}
    </aside>
  )
}
