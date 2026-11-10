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
import type { TFunction } from 'i18next'
import { describe, expect, it } from 'vitest'

import type { ChannelChain } from '../../types'
import {
  buildChannelChainInput,
  countChannelChainModels,
  DEFAULT_USER_CHANNEL_CHAIN_LIMITS,
  getChainFormValues,
  getItemsBytes,
  getUserChannelChainFormSchema,
  normalizeChannelChainType,
  toUserChannelChainItems,
  type UserChannelChainLimits,
} from '../user-channel-chain'

function interpolate(key: string, options?: Record<string, unknown>): string {
  if (!options) return key
  return key.replaceAll(/\{\{(\w+)\}\}/g, (_, name) =>
    String(options[name] ?? '')
  )
}

const t = interpolate as unknown as TFunction

const limits: UserChannelChainLimits = {
  maxChains: 2,
  maxGroupsPerChain: 2,
  maxModelsPerItem: 2,
  maxItemsBytes: 1024,
}

describe('buildChannelChainInput', () => {
  it('keeps the legacy group payload without a type field', () => {
    const payload = buildChannelChainInput({
      name: ' Primary ',
      type: 'groups',
      groups: ['vip', 'default'],
      items: [],
    })

    expect(payload).toEqual({ name: 'Primary', groups: ['vip', 'default'] })
    expect('type' in payload).toBe(false)
  })

  it('builds the models payload only from non-empty rows', () => {
    const payload = buildChannelChainInput({
      name: 'Routing',
      type: 'models',
      groups: [],
      items: [
        { id: 'row-1', group: ' vip ', models: ['gpt-4o', ' gpt-4o-mini '] },
        { id: 'row-2', group: 'default', models: ['claude-sonnet-4'] },
      ],
    })

    expect(payload).toEqual({
      name: 'Routing',
      type: 'models',
      items: [
        { group: 'vip', models: ['gpt-4o', 'gpt-4o-mini'] },
        { group: 'default', models: ['claude-sonnet-4'] },
      ],
    })
  })
})

describe('channel chain helpers', () => {
  it('treats a missing type as the legacy groups chain', () => {
    expect(normalizeChannelChainType(undefined)).toBe('groups')
    expect(normalizeChannelChainType('')).toBe('groups')
    expect(normalizeChannelChainType('groups')).toBe('groups')
    expect(normalizeChannelChainType('models')).toBe('models')
  })

  it('counts unique models across model rows', () => {
    const chain: ChannelChain = {
      id: 'chain_1',
      name: 'Routing',
      type: 'models',
      groups: [],
      items: [
        { group: 'vip', models: ['gpt-4o', 'gpt-4o-mini'] },
        { group: 'default', models: ['gpt-4o'] },
      ],
    }

    expect(countChannelChainModels(chain)).toBe(2)
  })

  it('measures the UTF-8 byte size of the submitted items', () => {
    const items = toUserChannelChainItems([
      { id: 'row-1', group: 'vip', models: ['中文模型'] },
    ])

    expect(getItemsBytes(items)).toBe(
      Buffer.byteLength(JSON.stringify(items), 'utf8')
    )
    expect(getItemsBytes([])).toBe(2)
  })

  it('restores a saved models chain into editable rows', () => {
    const chain: ChannelChain = {
      id: 'chain_1',
      name: 'Routing',
      type: 'models',
      groups: ['vip'],
      items: [{ group: 'vip', models: ['gpt-4o'] }],
    }

    const values = getChainFormValues(chain)

    expect(values.type).toBe('models')
    expect(values.groups).toEqual([])
    expect(values.items).toHaveLength(1)
    expect(values.items[0].group).toBe('vip')
    expect(values.items[0].models).toEqual(['gpt-4o'])
    expect(values.items[0].id).toBeTruthy()
  })

  it('restores a legacy groups chain without model rows', () => {
    const values = getChainFormValues({
      id: 'chain_1',
      name: 'Primary',
      type: 'groups',
      groups: ['vip', 'default'],
      items: [],
    })

    expect(values.type).toBe('groups')
    expect(values.groups).toEqual(['vip', 'default'])
    expect(values.items).toEqual([])
  })
})

describe('getUserChannelChainFormSchema', () => {
  const schema = getUserChannelChainFormSchema(t, limits)

  function validateGroups(values: Record<string, unknown>) {
    return schema.safeParse({
      name: 'Primary',
      type: 'groups',
      groups: [],
      items: [],
      ...values,
    })
  }

  it('accepts a valid group chain', () => {
    const result = validateGroups({ groups: ['vip'] })

    expect(result.success).toBe(true)
  })

  it('reports a required name', () => {
    const result = validateGroups({ name: '   ', groups: ['vip'] })

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues[0]?.path).toEqual(['name'])
    expect(result.error.issues[0]?.message).toBe('Name is required')
  })

  it('reports a name longer than 64 bytes', () => {
    const result = validateGroups({ name: 'a'.repeat(65), groups: ['vip'] })

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues[0]?.message).toBe(
      'Name must be at most 64 bytes'
    )
  })

  it('limits the name by UTF-8 bytes instead of characters', () => {
    // 22 Chinese characters are 66 UTF-8 bytes: valid by character count, but
    // rejected by the backend, so the form must reject it too.
    const result = validateGroups({
      name: '链'.repeat(22),
      groups: ['vip'],
    })

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues[0]?.path).toEqual(['name'])
    expect(result.error.issues[0]?.message).toBe(
      'Name must be at most 64 bytes'
    )
  })

  it('accepts a name whose UTF-8 encoding is exactly 64 bytes', () => {
    const result = validateGroups({
      name: `${'链'.repeat(21)}a`,
      groups: ['vip'],
    })

    expect(result.success).toBe(true)
  })

  it('requires at least one group', () => {
    const result = validateGroups({ groups: [] })

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues[0]?.path).toEqual(['groups'])
    expect(result.error.issues[0]?.message).toBe(
      'Select at least one group for the chain'
    )
  })

  it('rejects more groups than the limit', () => {
    const result = validateGroups({ groups: ['vip', 'default', 'svip'] })

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues[0]?.message).toBe(
      'A chain can contain at most 2 groups'
    )
  })

  function validateModels(items: unknown[]) {
    return schema.safeParse({
      name: 'Routing',
      type: 'models',
      groups: [],
      items,
    })
  }

  it('requires at least one model row', () => {
    const result = validateModels([])

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues[0]?.path).toEqual(['items'])
    expect(result.error.issues[0]?.message).toBe(
      'Add at least one model row to the chain'
    )
  })

  it('rejects duplicate groups across model rows', () => {
    const result = validateModels([
      { id: 'row-1', group: 'vip', models: ['gpt-4o'] },
      { id: 'row-2', group: 'vip', models: ['gpt-4o-mini'] },
    ])

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues[0]?.path).toEqual(['items', 1, 'group'])
    expect(result.error.issues[0]?.message).toBe(
      'This group is already used in another row'
    )
  })

  it('rejects a model row without a group', () => {
    const result = validateModels([
      { id: 'row-1', group: '  ', models: ['gpt-4o'] },
    ])

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues[0]?.path).toEqual(['items', 0, 'group'])
    expect(result.error.issues[0]?.message).toBe(
      'Select a group for every model row'
    )
  })

  it('rejects a model row without any model', () => {
    const result = validateModels([{ id: 'row-1', group: 'vip', models: [] }])

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues[0]?.path).toEqual(['items', 0, 'models'])
    expect(result.error.issues[0]?.message).toBe(
      'Select at least one model for each row'
    )
  })

  it('rejects more models than the per-row limit', () => {
    const result = validateModels([
      { id: 'row-1', group: 'vip', models: ['a', 'b', 'c'] },
    ])

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues[0]?.path).toEqual(['items', 0, 'models'])
    expect(result.error.issues[0]?.message).toBe(
      'A row can contain at most 2 models'
    )
  })

  it('rejects an items payload larger than the byte budget', () => {
    const tightSchema = getUserChannelChainFormSchema(t, {
      ...limits,
      maxItemsBytes: 40,
    })
    const result = tightSchema.safeParse({
      name: 'Routing',
      type: 'models',
      groups: [],
      items: [
        { id: 'row-1', group: 'vip', models: ['gpt-4o', 'claude-sonnet-4'] },
      ],
    })

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues[0]?.path).toEqual(['items'])
    expect(result.error.issues[0]?.message).toBe(
      'The model list is too large. Remove some models and try again.'
    )
  })

  it('keeps the default limits aligned with the backend contract', () => {
    expect(DEFAULT_USER_CHANNEL_CHAIN_LIMITS).toEqual({
      maxChains: 10,
      maxGroupsPerChain: 10,
      maxModelsPerItem: 100,
      maxItemsBytes: 60 * 1024,
    })
  })
})
