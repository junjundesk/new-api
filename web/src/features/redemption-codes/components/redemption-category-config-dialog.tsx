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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Field, FieldError, FieldLabel } from '@/components/ui/field'
import { Textarea } from '@/components/ui/textarea'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import {
  exportRedemptionCategoryConfig,
  importRedemptionCategoryConfig,
} from '../api'
import {
  redemptionCategoryConfigSchema,
  type RedemptionCategoryConfig,
} from '../types'

type CategoryConfigDialogProps = {
  mode: 'import' | 'export'
  onClose: () => void
}

export function RedemptionCategoryConfigDialog(
  props: CategoryConfigDialogProps
) {
  const { t } = useTranslation()
  const isExport = props.mode === 'export'
  const queryClient = useQueryClient()
  const { copyToClipboard } = useCopyToClipboard()
  const [input, setInput] = useState('')
  const [inputError, setInputError] = useState<'syntax' | 'config' | null>(null)
  const exported = useQuery({
    queryKey: ['redemption-category-config'],
    queryFn: exportRedemptionCategoryConfig,
    select: (config: RedemptionCategoryConfig) =>
      JSON.stringify(config, null, 2),
    enabled: isExport,
    gcTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
  })
  const importConfig = useMutation({
    mutationFn: importRedemptionCategoryConfig,
    onSuccess: ({ created, skipped }) => {
      void queryClient.invalidateQueries({
        queryKey: ['redemption-categories'],
      })
      void queryClient.invalidateQueries({
        queryKey: ['redemption-bonus-rules'],
      })
      void queryClient.invalidateQueries({
        queryKey: ['redemption-category-config'],
      })
      toast.success(
        t('Categories imported: {{created}} new, {{skipped}} unchanged', {
          created,
          skipped,
        })
      )
      props.onClose()
    },
  })
  const exportedJson = exported.isSuccess ? exported.data : ''
  const exportPlaceholder = exported.isFetching ? t('Loading...') : undefined
  let error: string | undefined
  if (isExport && exported.isError) {
    error = exported.error.message || t('Failed to load')
  } else if (inputError === 'syntax') {
    error = t('Invalid JSON format')
  } else if (inputError === 'config') {
    error = t(
      'Use a version 1 category export with valid face values and bonus rates. Do not include IDs or redemption codes.'
    )
  } else if (importConfig.error) {
    error = importConfig.error.message || t('Request failed')
  }

  const submitImport = () => {
    let parsed: unknown
    try {
      parsed = JSON.parse(input)
    } catch {
      setInputError('syntax')
      return
    }
    const config = redemptionCategoryConfigSchema.safeParse(parsed)
    if (!config.success) {
      setInputError('config')
      return
    }
    setInputError(null)
    importConfig.mutate(config.data)
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !importConfig.isPending) props.onClose()
      }}
    >
      <DialogContent
        className='max-h-[85dvh] overflow-y-auto sm:max-w-xl'
        showCloseButton={!importConfig.isPending}
      >
        <DialogHeader>
          <DialogTitle>
            {isExport
              ? t('Export code categories')
              : t('Import code categories')}
          </DialogTitle>
          <DialogDescription>
            {isExport
              ? t(
                  'Copy the category names, face values in quota units and optional bonus rates (100 basis points = 1%). No redemption codes or IDs are included.'
                )
              : t(
                  'Paste an exported configuration. Identical categories are skipped. If the same name has different settings, nothing is imported. Existing codes are never changed.'
                )}
          </DialogDescription>
        </DialogHeader>
        <Field data-invalid={!!error}>
          <FieldLabel htmlFor='redemption-category-json'>
            {t('Category JSON')}
          </FieldLabel>
          <Textarea
            id='redemption-category-json'
            className='h-48 max-h-[40dvh] font-mono'
            spellCheck={false}
            readOnly={isExport}
            disabled={importConfig.isPending}
            aria-invalid={!!error}
            aria-describedby={
              error ? 'redemption-category-json-error' : undefined
            }
            placeholder={
              isExport
                ? exportPlaceholder
                : '{"version":1,"categories":[{"name":"Standard","quota":5000000,"bonus_rate_bps":200}]}'
            }
            value={isExport ? exportedJson : input}
            onChange={(event) => {
              setInput(event.target.value)
              setInputError(null)
              if (importConfig.isError) importConfig.reset()
            }}
          />
          {error && (
            <FieldError id='redemption-category-json-error'>{error}</FieldError>
          )}
        </Field>
        <DialogFooter>
          <Button
            variant='outline'
            disabled={importConfig.isPending}
            onClick={props.onClose}
          >
            {isExport ? t('Close') : t('Cancel')}
          </Button>
          {isExport && exported.isError && (
            <Button
              onClick={() => void exported.refetch()}
              disabled={exported.isFetching}
            >
              {t('Retry')}
            </Button>
          )}
          {isExport && !exported.isError && (
            <Button
              disabled={!exportedJson || exported.isFetching}
              onClick={() => void copyToClipboard(exportedJson)}
            >
              {t('Copy JSON')}
            </Button>
          )}
          {!isExport && (
            <Button
              disabled={!input.trim() || importConfig.isPending}
              onClick={submitImport}
            >
              {importConfig.isPending
                ? t('Processing...')
                : t('Confirm import')}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
