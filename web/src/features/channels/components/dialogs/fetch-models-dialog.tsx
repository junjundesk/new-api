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
import { useQueryClient } from '@tanstack/react-query'
import { Loader2, Search, Info, ChevronDown } from 'lucide-react'
import { useState, useEffect, useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { fetchUpstreamModels, updateChannel } from '../../api'
import {
  categorizeModels,
  categorizeModelsWithRedirect,
  channelsQueryKeys,
  normalizeModelName,
  parseModelsString,
} from '../../lib'
import { parseModelMappingEntries } from '../../lib/model-mapping-edit'
import { useChannels } from '../channels-provider'
import { ModelMappingPopover } from './model-mapping-popover'

function normalizeModelNameList(models: readonly string[]): string[] {
  return [...new Set(models.map((m) => normalizeModelName(m)).filter(Boolean))]
}

type FetchModelsDialogBaseProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  redirectModels?: string[]
  redirectSourceModels?: string[]
  customFetcher?: () => Promise<string[]>
  channelName?: string | null
  /** Current model_mapping JSON. Provide it together with onModelMappingChange. */
  modelMapping?: string
  onModelMappingChange?: (modelMapping: string) => void
}

type FetchModelsDialogProps = FetchModelsDialogBaseProps &
  (
    | {
        onModelsSelected: (models: string[]) => void
        existingModelsOverride: string[]
      }
    | {
        onModelsSelected?: undefined
        existingModelsOverride?: undefined
      }
  )

export function FetchModelsDialog({
  open,
  onOpenChange,
  onModelsSelected,
  redirectModels = [],
  redirectSourceModels = [],
  customFetcher,
  existingModelsOverride,
  channelName,
  modelMapping,
  onModelMappingChange,
}: FetchModelsDialogProps) {
  const { t } = useTranslation()
  const { currentRow } = useChannels()
  const activeChannel = customFetcher ? null : currentRow
  const queryClient = useQueryClient()
  const [isFetching, setIsFetching] = useState(false)
  const [isSaving, setIsSaving] = useState(false)
  const [fetchedModels, setFetchedModels] = useState<string[]>([])
  const [selectedModels, setSelectedModels] = useState<string[]>([])
  const [searchKeyword, setSearchKeyword] = useState('')
  // Standalone mode keeps mapping edits local until "Save Models" persists them.
  const [localModelMapping, setLocalModelMapping] = useState('')

  // Parse existing models
  const existingModels = useMemo(
    () =>
      existingModelsOverride ?? parseModelsString(activeChannel?.models || ''),
    [existingModelsOverride, activeChannel?.models]
  )

  // Categorize models with redirect models
  const modelCategories = useMemo(
    () => categorizeModelsWithRedirect(existingModels, redirectModels),
    [existingModels, redirectModels]
  )

  const { classificationSet, redirectOnlySet } = modelCategories

  const currentModelMapping = onModelMappingChange
    ? modelMapping || ''
    : localModelMapping

  // The inline editor is upstream-first: a fetched row maps to public names.
  const exposedNamesByUpstream = useMemo(() => {
    const entries = parseModelMappingEntries(currentModelMapping) ?? {}
    const namesByUpstream = new Map<string, string[]>()
    for (const [publicName, upstreamModels] of Object.entries(entries)) {
      for (const upstreamModel of upstreamModels) {
        const names = namesByUpstream.get(upstreamModel) ?? []
        names.push(publicName)
        namesByUpstream.set(upstreamModel, names)
      }
    }
    return namesByUpstream
  }, [currentModelMapping])

  const fetchedModelSet = useMemo(
    () => new Set(normalizeModelNameList(fetchedModels)),
    [fetchedModels]
  )

  // Source keys in model_mapping are aliases, not real upstream IDs, so we
  // must skip them when computing "removed upstream" entries to avoid false
  // positives.
  const redirectSourceKeysSet = useMemo(
    () => new Set(normalizeModelNameList(redirectSourceModels)),
    [redirectSourceModels]
  )

  const removedModels = useMemo(() => {
    const kw = searchKeyword.toLowerCase().trim()
    return normalizeModelNameList(selectedModels).filter((model) => {
      if (fetchedModelSet.has(model)) return false
      if (redirectSourceKeysSet.has(model)) return false
      if (!kw) return true
      return model.toLowerCase().includes(kw)
    })
  }, [fetchedModelSet, redirectSourceKeysSet, searchKeyword, selectedModels])

  useEffect(() => {
    if (open && (activeChannel || customFetcher)) {
      handleFetchModels()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, activeChannel?.id, customFetcher])

  useEffect(() => {
    if (!open) return
    setLocalModelMapping(activeChannel?.model_mapping || '')
  }, [open, activeChannel?.id, activeChannel?.model_mapping])

  const handleFetchModels = async () => {
    if (!activeChannel && !customFetcher) return

    setIsFetching(true)
    try {
      if (customFetcher) {
        const list = await customFetcher()
        setFetchedModels(list)
        setSelectedModels(existingModels)
        toast.success(t('Fetched {{count}} models', { count: list.length }))
      } else if (activeChannel) {
        const response = await fetchUpstreamModels(activeChannel.id)
        if (response.success) {
          const list = Array.isArray(response.data) ? response.data : []
          setFetchedModels(list)
          setSelectedModels(existingModels)
          toast.success(t('Fetched {{count}} models', { count: list.length }))
        } else {
          toast.error(response.message || t('Failed to fetch models'))
          setFetchedModels([])
        }
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to fetch models')
      )
      setFetchedModels([])
    } finally {
      setIsFetching(false)
    }
  }

  const handleSave = async () => {
    // If onModelsSelected callback is provided, use it (form filling mode)
    if (onModelsSelected) {
      onModelsSelected(selectedModels)
      toast.success(t('Models filled to form'))
      onOpenChange(false)
      return
    }

    // Otherwise, directly save to API (standalone mode)
    if (!activeChannel) return
    setIsSaving(true)
    try {
      const modelsString = selectedModels.join(',')
      const mappingChanged =
        localModelMapping !== (activeChannel.model_mapping || '')
      const response = await updateChannel(activeChannel.id, {
        models: modelsString,
        ...(mappingChanged ? { model_mapping: localModelMapping } : {}),
      })
      if (response.success) {
        toast.success(t('Models updated successfully'))
        queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
        onOpenChange(false)
      } else {
        toast.error(response.message || t('Failed to update models'))
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to update models')
      )
    } finally {
      setIsSaving(false)
    }
  }

  const handleClose = () => {
    setFetchedModels([])
    setSelectedModels([])
    setSearchKeyword('')
    onOpenChange(false)
  }

  // Filter models by search
  const filteredModels = useMemo(() => {
    if (!searchKeyword) return fetchedModels
    return fetchedModels.filter((model) =>
      model.toLowerCase().includes(searchKeyword.toLowerCase())
    )
  }, [fetchedModels, searchKeyword])

  const {
    newModels,
    existingFilteredModels,
    newModelsByCategory,
    existingModelsByCategory,
  } = useMemo(() => {
    const newModels: string[] = []
    const existingFilteredModels: string[] = []

    for (const model of filteredModels) {
      if (classificationSet.has(normalizeModelName(model))) {
        existingFilteredModels.push(model)
      } else {
        newModels.push(model)
      }
    }

    return {
      newModels,
      existingFilteredModels,
      newModelsByCategory: categorizeModels(newModels),
      existingModelsByCategory: categorizeModels(existingFilteredModels),
    }
  }, [classificationSet, filteredModels])

  // 厂商分类按 a-z 排序，Other 放最后，便于查找
  const getSortedCategoryEntries = (
    categories: Record<string, string[]>
  ): [string, string[]][] =>
    Object.entries(categories).sort(([a], [b]) => {
      if (a === 'Other') return 1
      if (b === 'Other') return -1
      return a.localeCompare(b, undefined, { sensitivity: 'base' })
    })

  const toggleModel = (model: string) => {
    setSelectedModels((prev) =>
      prev.includes(model) ? prev.filter((m) => m !== model) : [...prev, model]
    )
  }

  const handleModelMappingApply = (
    exposedName: string,
    nextMapping: string
  ) => {
    if (onModelMappingChange) {
      onModelMappingChange(nextMapping)
    } else {
      setLocalModelMapping(nextMapping)
    }
    // Clients request the public name, so it has to be in the published list.
    if (exposedName) {
      setSelectedModels((prev) =>
        prev.includes(exposedName) ? prev : [...prev, exposedName]
      )
    }
    toast.success(t('Model mapping updated'))
  }

  const toggleCategory = (categoryModels: string[], isChecked: boolean) => {
    setSelectedModels((prev) => {
      if (isChecked) {
        const newSelected = [...prev]
        categoryModels.forEach((model) => {
          if (!newSelected.includes(model)) {
            newSelected.push(model)
          }
        })
        return newSelected
      } else {
        return prev.filter((m) => !categoryModels.includes(m))
      }
    })
  }

  const isCategorySelected = (categoryModels: string[]) => {
    return categoryModels.every((m) => selectedModels.includes(m))
  }

  const renderModelCategory = (
    categoryName: string,
    categoryModels: string[]
  ) => {
    const allSelected = isCategorySelected(categoryModels)

    return (
      <Collapsible key={categoryName} defaultOpen>
        <CollapsibleTrigger className='hover:bg-muted/50 flex w-full items-center justify-between rounded-lg border p-3'>
          <div className='flex items-center gap-2'>
            <ChevronDown className='h-4 w-4' />
            <span className='font-medium'>
              {categoryName} ({categoryModels.length})
            </span>
          </div>
          <div className='flex items-center gap-2'>
            <span className='text-muted-foreground text-sm'>
              {categoryModels.filter((m) => selectedModels.includes(m)).length}{' '}
              / {categoryModels.length} selected
            </span>
            <Checkbox
              checked={allSelected}
              onCheckedChange={(checked) =>
                toggleCategory(categoryModels, !!checked)
              }
              onClick={(e) => e.stopPropagation()}
            />
          </div>
        </CollapsibleTrigger>
        <CollapsibleContent className='px-4 py-2'>
          <div className='grid grid-cols-2 gap-2'>
            {categoryModels.map((model) => {
              const exposedNames =
                exposedNamesByUpstream.get(model.trim()) ?? []
              return (
                <div
                  key={model}
                  className='hover:bg-muted/40 flex min-w-0 items-center gap-2 rounded-md px-1 py-0.5'
                >
                  <Checkbox
                    id={model}
                    checked={selectedModels.includes(model)}
                    onCheckedChange={() => toggleModel(model)}
                  />
                  <Label
                    htmlFor={model}
                    className='min-w-0 flex-1 cursor-pointer truncate text-sm font-normal'
                    title={model}
                  >
                    {model}
                  </Label>
                  {exposedNames.length > 0 && (
                    <span
                      className='text-muted-foreground max-w-24 shrink-0 truncate text-xs'
                      title={exposedNames.join(', ')}
                    >
                      → {exposedNames.join(', ')}
                    </span>
                  )}
                  {redirectOnlySet.has(normalizeModelName(model)) && (
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Info className='h-3.5 w-3.5 shrink-0 text-amber-500' />
                        }
                      />
                      <TooltipContent>
                        {t('From model redirect, not yet added to models list')}
                      </TooltipContent>
                    </Tooltip>
                  )}
                  <div className='flex shrink-0 items-center'>
                    <ModelMappingPopover
                      model={model}
                      modelMapping={currentModelMapping}
                      onApply={(nextMapping, exposedName) =>
                        handleModelMappingApply(exposedName, nextMapping)
                      }
                    />
                    <CopyButton
                      value={model}
                      size='icon'
                      className='text-muted-foreground hover:text-foreground size-6'
                      iconClassName='size-3.5'
                      tooltip={t('Copy model name')}
                      successTooltip={t('Copied!')}
                      aria-label={t('Copy model name')}
                    />
                  </div>
                </div>
              )
            })}
          </div>
        </CollapsibleContent>
      </Collapsible>
    )
  }

  const showFooterActions =
    !!(activeChannel || customFetcher) &&
    !isFetching &&
    (fetchedModels.length > 0 || removedModels.length > 0)

  let dialogDescription: ReactNode = t('Fetch available models from upstream')
  if (activeChannel) {
    dialogDescription = (
      <>
        {t('Channel:')} <strong>{activeChannel.name}</strong>
      </>
    )
  } else if (channelName) {
    dialogDescription = (
      <>
        {t('Channel:')} <strong>{channelName}</strong>
      </>
    )
  }

  let defaultTab = 'existing'
  if (newModels.length > 0) {
    defaultTab = 'new'
  } else if (removedModels.length > 0) {
    defaultTab = 'removed'
  }

  let dialogBody: ReactNode
  if (!activeChannel && !customFetcher) {
    dialogBody = (
      <div className='text-muted-foreground py-8 text-center'>
        {t('No channel selected')}
      </div>
    )
  } else if (isFetching) {
    dialogBody = (
      <div className='flex items-center justify-center py-12'>
        <Loader2 className='text-muted-foreground h-8 w-8 animate-spin' />
      </div>
    )
  } else if (fetchedModels.length === 0 && removedModels.length === 0) {
    dialogBody = (
      <div className='text-muted-foreground py-8 text-center'>
        <p>{t('No models fetched yet.')}</p>
        <Button
          className='mt-4'
          onClick={handleFetchModels}
          disabled={isFetching}
        >
          {t('Fetch Models')}
        </Button>
      </div>
    )
  } else {
    dialogBody = (
      <div className='space-y-4'>
        {/* Search Bar */}
        <div className='relative'>
          <Search className='text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2' />
          <Input
            placeholder={t('Search models...')}
            value={searchKeyword}
            onChange={(e) => setSearchKeyword(e.target.value)}
            className='pl-9'
          />
        </div>

        {/* Tabs for New vs Existing vs Removed */}
        <Tabs
          key={`${activeChannel?.id ?? 'custom'}-${fetchedModels.length}-${removedModels.length}`}
          defaultValue={defaultTab}
        >
          <TabsList
            className={`grid w-full ${removedModels.length > 0 ? 'grid-cols-3' : 'grid-cols-2'}`}
          >
            <TabsTrigger value='new' disabled={newModels.length === 0}>
              {t('New Models ({{count}})', { count: newModels.length })}
            </TabsTrigger>
            <TabsTrigger
              value='existing'
              disabled={existingFilteredModels.length === 0}
            >
              {t('Existing Models ({{count}})', {
                count: existingFilteredModels.length,
              })}
            </TabsTrigger>
            {removedModels.length > 0 && (
              <TabsTrigger value='removed'>
                {t('Removed Models ({{count}})', {
                  count: removedModels.length,
                })}
              </TabsTrigger>
            )}
          </TabsList>

          <TabsContent
            value='new'
            className='max-h-96 space-y-2 overflow-y-auto'
          >
            {getSortedCategoryEntries(newModelsByCategory).map(
              ([category, models]) => renderModelCategory(category, models)
            )}
          </TabsContent>

          <TabsContent
            value='existing'
            className='max-h-96 space-y-2 overflow-y-auto'
          >
            {getSortedCategoryEntries(existingModelsByCategory).map(
              ([category, models]) => renderModelCategory(category, models)
            )}
          </TabsContent>

          {removedModels.length > 0 && (
            <TabsContent
              value='removed'
              className='max-h-96 space-y-2 overflow-y-auto'
            >
              <p className='text-muted-foreground text-xs'>
                {t(
                  'These models are still in your selection but were not returned by the upstream listing. Entries that are only model_mapping source aliases are omitted. Toggle to adjust before saving.'
                )}
              </p>
              {renderModelCategory(t('Removed'), removedModels)}
            </TabsContent>
          )}
        </Tabs>

        {/* Selection Summary */}
        <div className='bg-muted/50 rounded-lg border p-3 text-sm'>
          {t('{{n}} model(s) selected', { n: selectedModels.length })}
        </div>
      </div>
    )
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleClose}
      title={t('Fetch Models')}
      description={dialogDescription}
      contentClassName='max-w-3xl'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        showFooterActions ? (
          <>
            <Button variant='outline' onClick={handleClose} disabled={isSaving}>
              {t('Cancel')}
            </Button>
            <Button onClick={handleSave} disabled={isSaving}>
              {isSaving && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              {isSaving ? t('Saving...') : t('Save Models')}
            </Button>
          </>
        ) : null
      }
    >
      {dialogBody}
    </Dialog>
  )
}
