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
import { ArrowRight, Code, Plus, Table, Trash2, X } from 'lucide-react'
import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { JsonCodeEditor } from '@/components/json-code-editor'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

type ModelMappingEditorProps = {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  sourceModelOptions?: string[]
  targetModelOptions?: string[]
}

type MappingRow = {
  id: string
  from: string
  tos: string[]
}

export const MAX_CANDIDATE_MODELS_PER_SOURCE = 32

const DUPLICATE_MAPPING_SENTINEL = '{ "duplicate_source_models": '

function getDuplicateSources(rows: MappingRow[]): string[] {
  const seen = new Set<string>()
  const duplicates = new Set<string>()

  for (const row of rows) {
    const source = row.from.trim()
    if (!source) continue
    if (seen.has(source)) {
      duplicates.add(source)
    } else {
      seen.add(source)
    }
  }

  return Array.from(duplicates)
}

function normalizeCandidateList(value: unknown): string[] | null {
  if (typeof value === 'string') return [value]
  if (!Array.isArray(value)) return null
  const candidates: string[] = []
  for (const item of value) {
    if (typeof item !== 'string') return null
    candidates.push(item)
  }
  return candidates
}

export function ModelMappingEditor(props: ModelMappingEditorProps) {
  const { t } = useTranslation()
  const sourceListId = useId()
  const targetListId = useId()
  const [mode, setMode] = useState<'visual' | 'json'>('visual')
  const [rows, setRows] = useState<MappingRow[]>([])
  const [jsonValue, setJsonValue] = useState(props.value)
  const [jsonError, setJsonError] = useState<string | null>(null)
  const nextRowIdRef = useRef(0)
  const lastEmittedJsonRef = useRef<string | null>(null)
  const duplicateSources = useMemo(() => getDuplicateSources(rows), [rows])

  const createRowId = () => {
    nextRowIdRef.current += 1
    return `mapping-${nextRowIdRef.current}`
  }

  const parseJsonToRows = (json: string): boolean => {
    try {
      if (!json.trim()) {
        setRows([])
        setJsonError(null)
        return true
      }
      const parsed = JSON.parse(json)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        setJsonError(t('Model mapping must be a valid JSON object'))
        return false
      }
      const entries = Object.entries(parsed)
      const invalidValue = entries.find(
        ([, to]) => normalizeCandidateList(to) === null
      )
      if (invalidValue) {
        setJsonError(
          t(
            'Model mapping values must be a model name or an array of model names'
          )
        )
        return false
      }
      setRows((previousRows) => {
        const remainingRows = [...previousRows]
        return entries.map(([from, to], index) => {
          const tos = normalizeCandidateList(to) ?? []
          const previousRow = previousRows[index]
          let existingIndex = remainingRows.findIndex(
            (row) => row.from === from
          )
          if (existingIndex < 0 && previousRow) {
            existingIndex = remainingRows.findIndex(
              (row) => row.id === previousRow.id
            )
          }
          if (existingIndex >= 0) {
            const [existing] = remainingRows.splice(existingIndex, 1)
            return {
              id: existing.id,
              from,
              tos,
            }
          }
          return {
            id: createRowId(),
            from,
            tos,
          }
        })
      })
      setJsonError(null)
      return true
    } catch (_error) {
      setJsonError(t('Model mapping must be valid JSON format'))
      return false
    }
  }

  // Parse JSON to rows when value changes externally. Values this component
  // just emitted are ignored: re-parsing them would rebuild the rows from JSON,
  // which cannot carry an empty candidate slot and would collapse rows the user
  // is still filling in.
  useEffect(() => {
    if (props.value === lastEmittedJsonRef.current) {
      return
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setJsonValue(props.value)
    parseJsonToRows(props.value)
  }, [props.value])

  const convertRowsToJson = (updatedRows: MappingRow[]): string => {
    if (updatedRows.length === 0) {
      return ''
    }
    const obj: Record<string, string | string[]> = {}
    updatedRows.forEach((row) => {
      if (!row.from.trim()) return
      const tos = row.tos.map((item) => item.trim()).filter(Boolean)
      if (tos.length === 0) {
        obj[row.from.trim()] = ''
      } else if (tos.length === 1) {
        obj[row.from.trim()] = tos[0]
      } else {
        obj[row.from.trim()] = tos
      }
    })
    return JSON.stringify(obj, null, 2)
  }

  const syncRows = (updatedRows: MappingRow[]) => {
    setRows(updatedRows)
    const duplicates = getDuplicateSources(updatedRows)
    if (duplicates.length > 0) {
      setJsonError(t('Duplicate source model mappings are not allowed'))
      setJsonValue(DUPLICATE_MAPPING_SENTINEL)
      lastEmittedJsonRef.current = DUPLICATE_MAPPING_SENTINEL
      props.onChange(DUPLICATE_MAPPING_SENTINEL)
      return
    }

    const json = convertRowsToJson(updatedRows)
    setJsonError(null)
    setJsonValue(json)
    lastEmittedJsonRef.current = json
    props.onChange(json)
  }

  const handleAddRow = () => {
    const newRow: MappingRow = {
      id: createRowId(),
      from: '',
      tos: [],
    }
    syncRows([...rows, newRow])
  }

  const handleDeleteRow = (id: string) => {
    syncRows(rows.filter((row) => row.id !== id))
  }

  const handleSourceChange = (id: string, newValue: string) => {
    const updatedRows = rows.map((row) =>
      row.id === id ? { ...row, from: newValue } : row
    )
    syncRows(updatedRows)
  }

  const handleCandidateChange = (id: string, index: number, value: string) => {
    const updatedRows = rows.map((row) => {
      if (row.id !== id) return row
      const tos = [...row.tos]
      tos[index] = value
      return { ...row, tos }
    })
    syncRows(updatedRows)
  }

  const handleAddCandidate = (id: string) => {
    const updatedRows = rows.map((row) =>
      row.id === id ? { ...row, tos: [...row.tos, ''] } : row
    )
    syncRows(updatedRows)
  }

  const handleRemoveCandidate = (id: string, index: number) => {
    const updatedRows = rows.map((row) => {
      if (row.id !== id) return row
      const tos = row.tos.filter((_, itemIndex) => itemIndex !== index)
      // 至少要保留一个输入框，清空最后一个候选等同于取消该映射
      return { ...row, tos: tos.length > 0 ? tos : [''] }
    })
    syncRows(updatedRows)
  }

  const handleJsonChange = (newJson: string) => {
    setJsonValue(newJson)
    lastEmittedJsonRef.current = newJson
    props.onChange(newJson)
    parseJsonToRows(newJson)
  }

  const handleFillTemplate = () => {
    const template = JSON.stringify(
      {
        'gpt-3.5-turbo': 'gpt-3.5-turbo-0125',
        'claude-3-5-sonnet': [
          'claude-3-5-sonnet-20241022',
          'claude-3-5-sonnet-latest',
        ],
      },
      null,
      2
    )
    setJsonValue(template)
    lastEmittedJsonRef.current = template
    props.onChange(template)
    parseJsonToRows(template)
  }

  const handleModeChange = (nextMode: string) => {
    if (nextMode !== 'visual' && nextMode !== 'json') return
    if (nextMode === 'json') {
      const duplicates = getDuplicateSources(rows)
      if (duplicates.length === 0) {
        const json = convertRowsToJson(rows)
        setJsonValue(json)
        lastEmittedJsonRef.current = json
        props.onChange(json)
      }
      setMode('json')
      return
    }
    parseJsonToRows(jsonValue)
    setMode('visual')
  }

  const tooManyCandidates = useMemo(
    () =>
      rows.some(
        (row) =>
          row.tos.filter((item) => item.trim()).length >
          MAX_CANDIDATE_MODELS_PER_SOURCE
      ),
    [rows]
  )

  return (
    <div className='space-y-2'>
      <Tabs value={mode} onValueChange={handleModeChange} className='space-y-2'>
        <div className='flex items-center justify-between gap-3'>
          <TabsList>
            <TabsTrigger value='visual'>
              <Table className='h-4 w-4' aria-hidden='true' />
              {t('Visual')}
            </TabsTrigger>
            <TabsTrigger value='json'>
              <Code className='h-4 w-4' aria-hidden='true' />
              {t('JSON')}
            </TabsTrigger>
          </TabsList>
          <Button
            type='button'
            variant='link'
            size='sm'
            className='h-auto p-0'
            onClick={handleFillTemplate}
            disabled={props.disabled}
          >
            {t('Fill Template')}
          </Button>
        </div>

        {jsonError && (
          <Alert variant='destructive'>
            <AlertDescription>{jsonError}</AlertDescription>
          </Alert>
        )}

        {duplicateSources.length > 0 && (
          <Alert>
            <AlertDescription>
              {t('Duplicate source model(s): {{models}}', {
                models: duplicateSources.join(', '),
              })}
            </AlertDescription>
          </Alert>
        )}

        {tooManyCandidates && (
          <Alert variant='destructive'>
            <AlertDescription>
              {t('Use at most {{count}} candidate models per source', {
                count: MAX_CANDIDATE_MODELS_PER_SOURCE,
              })}
            </AlertDescription>
          </Alert>
        )}

        <TabsContent value='visual' className='space-y-3'>
          {rows.length > 0 ? (
            <div className='space-y-3'>
              {rows.map((row) => {
                const candidates = row.tos.length > 0 ? row.tos : ['']
                const atCandidateLimit =
                  candidates.filter((item) => item.trim()).length >=
                  MAX_CANDIDATE_MODELS_PER_SOURCE
                return (
                  <div
                    key={row.id}
                    className='bg-muted/30 space-y-2 rounded-lg border p-3'
                  >
                    <div className='flex items-center gap-2'>
                      <Input
                        value={row.from}
                        onChange={(e) =>
                          handleSourceChange(row.id, e.target.value)
                        }
                        placeholder={t('Original Model')}
                        disabled={props.disabled}
                        list={sourceListId}
                        className='max-w-xs'
                        aria-label={t('Original Model')}
                      />
                      <ArrowRight
                        className='text-muted-foreground h-4 w-4 shrink-0'
                        aria-hidden='true'
                      />
                      <span className='text-muted-foreground text-xs'>
                        {candidates.filter((item) => item.trim()).length > 1
                          ? t('{{count}} models, tried in order', {
                              count: candidates.filter((item) => item.trim())
                                .length,
                            })
                          : t('Replacement Model')}
                      </span>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        onClick={() => handleDeleteRow(row.id)}
                        disabled={props.disabled}
                        className='ml-auto h-8 w-8'
                        aria-label={t('Delete mapping')}
                      >
                        <Trash2 className='h-4 w-4' aria-hidden='true' />
                      </Button>
                    </div>

                    <div className='space-y-2'>
                      {candidates.map((candidate, index) => (
                        <div
                          key={`${row.id}-${index}`}
                          className='flex items-center gap-2'
                        >
                          <Badge
                            variant={index === 0 ? 'default' : 'secondary'}
                            className='w-6 shrink-0 justify-center px-0 tabular-nums'
                          >
                            {index + 1}
                          </Badge>
                          <Input
                            value={candidate}
                            onChange={(e) =>
                              handleCandidateChange(
                                row.id,
                                index,
                                e.target.value
                              )
                            }
                            placeholder='gpt-3.5-turbo-0125'
                            disabled={props.disabled}
                            list={targetListId}
                            aria-label={t('Replacement Model')}
                          />
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon'
                            onClick={() => handleRemoveCandidate(row.id, index)}
                            disabled={props.disabled}
                            className='h-9 w-9 shrink-0'
                            aria-label={t('Remove model')}
                          >
                            <X className='h-4 w-4' aria-hidden='true' />
                          </Button>
                        </div>
                      ))}
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => handleAddCandidate(row.id)}
                        disabled={props.disabled || atCandidateLimit}
                        className='ml-8'
                      >
                        <Plus className='mr-2 h-3.5 w-3.5' />
                        {t('Add Fallback Model')}
                      </Button>
                    </div>
                  </div>
                )
              })}
              <p className='text-muted-foreground text-xs'>
                {t(
                  'When the first model is unavailable, the request automatically retries the same channel with the next model in order.'
                )}
              </p>
            </div>
          ) : (
            <div className='text-muted-foreground flex h-24 items-center justify-center rounded-md border border-dashed text-sm'>
              {t(
                'No model mappings configured. Click "Add Mapping" to get started.'
              )}
            </div>
          )}
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={handleAddRow}
            disabled={props.disabled}
            className='w-full'
          >
            <Plus className='mr-2 h-4 w-4' />
            {t('Add Mapping')}
          </Button>
        </TabsContent>
        <TabsContent value='json'>
          <JsonCodeEditor
            value={jsonValue}
            onChange={handleJsonChange}
            placeholder={t('{"original-model": "replacement-model"}')}
            disabled={props.disabled}
            className={jsonError ? 'border-destructive' : undefined}
            aria-invalid={Boolean(jsonError)}
            ariaLabel={t('Model Mapping')}
          />
        </TabsContent>
      </Tabs>

      {props.sourceModelOptions && props.sourceModelOptions.length > 0 && (
        <datalist id={sourceListId}>
          {props.sourceModelOptions.map((model) => (
            <option key={model} value={model} />
          ))}
        </datalist>
      )}
      {props.targetModelOptions && props.targetModelOptions.length > 0 && (
        <datalist id={targetListId}>
          {props.targetModelOptions.map((model) => (
            <option key={model} value={model} />
          ))}
        </datalist>
      )}
    </div>
  )
}
