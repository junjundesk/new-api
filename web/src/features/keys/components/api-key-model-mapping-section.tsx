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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

import { applyModelMappingPreset } from '../lib/model-mapping'
import type {
  TokenModelMapping,
  TokenModelMappingOptions,
  TokenModelMappingPreset,
} from '../types'
import { ModelMappingEditor } from './model-mapping-editor'

type ApiKeyModelMappingSectionProps = {
  value: TokenModelMapping[]
  onChange: (value: TokenModelMapping[]) => void
  options?: TokenModelMappingOptions
  loading: boolean
  failed: boolean
  onRetry: () => void
  disabled?: boolean
  showValidation?: boolean
}

export function ApiKeyModelMappingSection(
  props: ApiKeyModelMappingSectionProps
) {
  const { t } = useTranslation()
  const [pendingPreset, setPendingPreset] =
    useState<TokenModelMappingPreset | null>(null)

  useEffect(() => {
    setPendingPreset(null)
  }, [props.options])

  const pendingConflicts = pendingPreset
    ? applyModelMappingPreset(props.value, pendingPreset.mappings).conflicts
    : []
  const models =
    props.loading || props.failed ? undefined : props.options?.models
  const presets = props.options?.presets ?? []
  const ready = !props.loading && !props.failed

  return (
    <div className='flex min-w-0 flex-col gap-4'>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Match the exact model name sent by your client. Requests use and are billed for the target model. Mapping runs once; other model names are unchanged.'
        )}
      </p>

      {props.loading && (
        <div
          role='status'
          aria-label={t('Loading available models')}
          className='flex flex-col gap-2'
        >
          <Skeleton className='h-4 w-2/3' />
          <Skeleton className='h-8 w-full' />
        </div>
      )}

      {props.failed && (
        <Alert variant='destructive'>
          <AlertTitle>{t('Could not load available models')}</AlertTitle>
          <AlertDescription>
            <p>{t('Retry to select and validate target models for this group.')}</p>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={props.disabled || props.loading}
              onClick={props.onRetry}
            >
              {t('Retry')}
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {ready && props.options && props.options.models.length === 0 && (
        <Alert>
          <AlertTitle>{t('No models available in this group')}</AlertTitle>
          <AlertDescription>
            {t('Choose another group or remove the mappings before saving.')}
          </AlertDescription>
        </Alert>
      )}

      {ready && presets.length > 0 && (
        <div className='flex flex-col gap-2'>
          <p className='text-sm font-medium'>{t('Mapping presets')}</p>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Apply a preset, then edit the copied mappings. Future preset changes do not change this key.'
            )}
          </p>
          {presets.map((preset) => {
            const unavailable = preset.mappings.some(
              (mapping) => !props.options?.models.includes(mapping.target_model)
            )
            return (
              <div
                key={preset.id}
                className='flex min-w-0 items-start justify-between gap-3 py-1'
              >
                <div className='flex min-w-0 flex-col gap-1'>
                  <div className='flex flex-wrap items-center gap-2'>
                    <span className='text-sm font-medium break-words'>
                      {preset.name}
                    </span>
                    {preset.recommended && (
                      <Badge variant='secondary'>{t('Recommended')}</Badge>
                    )}
                  </div>
                  {preset.description && (
                    <p className='text-muted-foreground text-xs break-words'>
                      {preset.description}
                    </p>
                  )}
                  {unavailable && (
                    <p className='text-destructive text-xs'>
                      {t('Preset targets are unavailable in this group')}
                    </p>
                  )}
                </div>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={props.disabled || unavailable}
                  onClick={() => {
                    const result = applyModelMappingPreset(
                      props.value,
                      preset.mappings
                    )
                    if (result.conflicts.length > 0) {
                      setPendingPreset(preset)
                      return
                    }
                    props.onChange(result.mappings)
                  }}
                >
                  {t('Apply')}
                </Button>
              </div>
            )
          })}
        </div>
      )}

      <ModelMappingEditor
        value={props.value}
        onChange={props.onChange}
        models={models}
        disabled={props.disabled}
        targetsDisabled={props.loading || props.failed || !props.options}
        showValidation={props.showValidation}
      />

      <AlertDialog
        open={pendingPreset !== null}
        onOpenChange={(open) => {
          if (!open) setPendingPreset(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Replace existing mappings?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This preset contains source models already configured on this key. Replace these mappings to apply the preset.'
              )}
              <span className='mt-2 block break-all'>
                {pendingConflicts.join(', ')}
              </span>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              disabled={props.disabled}
              onClick={() => {
                if (!pendingPreset) return
                props.onChange(
                  applyModelMappingPreset(
                    props.value,
                    pendingPreset.mappings,
                    true
                  ).mappings
                )
                setPendingPreset(null)
              }}
            >
              {t('Replace and apply')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
