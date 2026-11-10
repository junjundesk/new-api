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
import { nanoid } from 'nanoid'
import { z } from 'zod'

import type {
  ChannelChain,
  ChannelChainInput,
  UserChannelChainItem,
  UserChannelChainType,
} from '../types'

export const DEFAULT_MAX_CHAINS = 10
export const DEFAULT_MAX_GROUPS_PER_CHAIN = 10
export const DEFAULT_MAX_MODELS_PER_ITEM = 100
export const DEFAULT_MAX_ITEMS_BYTES = 60 * 1024

export type UserChannelChainLimits = {
  maxChains: number
  maxGroupsPerChain: number
  maxModelsPerItem: number
  maxItemsBytes: number
}

export const DEFAULT_USER_CHANNEL_CHAIN_LIMITS: UserChannelChainLimits = {
  maxChains: DEFAULT_MAX_CHAINS,
  maxGroupsPerChain: DEFAULT_MAX_GROUPS_PER_CHAIN,
  maxModelsPerItem: DEFAULT_MAX_MODELS_PER_ITEM,
  maxItemsBytes: DEFAULT_MAX_ITEMS_BYTES,
}

export type UserChannelChainItemRow = {
  id: string
  group: string
  models: string[]
}

export function createUserChannelChainRowId() {
  if (typeof globalThis.crypto?.randomUUID === 'function') {
    return globalThis.crypto.randomUUID()
  }
  return `${nanoid()}`
}

// Older responses do not carry a type; they are legacy group-based chains.
export function normalizeChannelChainType(
  type: string | undefined
): UserChannelChainType {
  return type === 'models' ? 'models' : 'groups'
}

export function createUserChannelChainItemRow(
  item?: Partial<UserChannelChainItem>
): UserChannelChainItemRow {
  return {
    id: createUserChannelChainRowId(),
    group: item?.group ?? '',
    models: item?.models ? [...item.models] : [],
  }
}

export function getDefaultUserChannelChainFormValues() {
  return {
    name: '',
    type: 'groups' as UserChannelChainType,
    groups: [] as string[],
    items: [] as UserChannelChainItemRow[],
  }
}

export function getChainFormValues(chain: ChannelChain) {
  const type = normalizeChannelChainType(chain.type)
  if (type === 'groups') {
    return {
      name: chain.name,
      type,
      groups: [...(chain.groups ?? [])],
      items: [] as UserChannelChainItemRow[],
    }
  }
  return {
    name: chain.name,
    type,
    groups: [] as string[],
    items: (chain.items ?? []).map((item) =>
      createUserChannelChainItemRow(item)
    ),
  }
}

export function countChannelChainModels(chain: ChannelChain) {
  const models = new Set<string>()
  for (const item of chain.items ?? []) {
    for (const model of item.models ?? []) models.add(model)
  }
  return models.size
}

export function getUtf8ByteLength(value: string) {
  return new TextEncoder().encode(value).length
}

export function getItemsBytes(items: UserChannelChainItem[]) {
  return getUtf8ByteLength(JSON.stringify(items))
}

export function toUserChannelChainItems(
  rows: UserChannelChainItemRow[]
): UserChannelChainItem[] {
  return rows.map((row) => ({
    group: row.group.trim(),
    models: row.models.map((model) => model.trim()).filter(Boolean),
  }))
}

export function buildChannelChainInput(
  values: UserChannelChainFormValues
): ChannelChainInput {
  const name = values.name.trim()
  if (values.type === 'groups') {
    return { name, groups: values.groups }
  }
  return {
    name,
    type: 'models',
    items: toUserChannelChainItems(values.items),
  }
}

const userChannelChainItemRowSchema = z.object({
  id: z.string(),
  group: z.string(),
  models: z.array(z.string()),
})

export function getUserChannelChainFormSchema(
  t: TFunction,
  limits: UserChannelChainLimits = DEFAULT_USER_CHANNEL_CHAIN_LIMITS
) {
  return z
    .object({
      name: z
        .string()
        .trim()
        .min(1, { error: t('Name is required') })
        .refine((value) => getUtf8ByteLength(value) <= 64, {
          error: t('Name must be at most 64 bytes'),
        }),
      type: z.enum(['groups', 'models']),
      groups: z.array(z.string()),
      items: z
        .array(userChannelChainItemRowSchema)
        .max(limits.maxGroupsPerChain, {
          error: t('A chain can contain at most {{count}} rows', {
            count: limits.maxGroupsPerChain,
          }),
        }),
    })
    .superRefine((values, ctx) => {
      if (values.type === 'groups') {
        if (values.groups.length === 0) {
          ctx.addIssue({
            code: 'custom',
            path: ['groups'],
            message: t('Select at least one group for the chain'),
          })
          return
        }
        if (values.groups.length > limits.maxGroupsPerChain) {
          ctx.addIssue({
            code: 'custom',
            path: ['groups'],
            message: t('A chain can contain at most {{count}} groups', {
              count: limits.maxGroupsPerChain,
            }),
          })
        }
        return
      }

      if (values.items.length === 0) {
        ctx.addIssue({
          code: 'custom',
          path: ['items'],
          message: t('Add at least one model row to the chain'),
        })
        return
      }

      const usedGroups = new Set<string>()
      values.items.forEach((item, index) => {
        const group = item.group.trim()
        if (!group) {
          ctx.addIssue({
            code: 'custom',
            path: ['items', index, 'group'],
            message: t('Select a group for every model row'),
          })
        } else if (usedGroups.has(group)) {
          ctx.addIssue({
            code: 'custom',
            path: ['items', index, 'group'],
            message: t('This group is already used in another row'),
          })
        } else {
          usedGroups.add(group)
        }

        if (item.models.length === 0) {
          ctx.addIssue({
            code: 'custom',
            path: ['items', index, 'models'],
            message: t('Select at least one model for each row'),
          })
        }
        if (item.models.length > limits.maxModelsPerItem) {
          ctx.addIssue({
            code: 'custom',
            path: ['items', index, 'models'],
            message: t('A row can contain at most {{count}} models', {
              count: limits.maxModelsPerItem,
            }),
          })
        }
      })

      const bytes = getItemsBytes(toUserChannelChainItems(values.items))
      if (bytes > limits.maxItemsBytes) {
        ctx.addIssue({
          code: 'custom',
          path: ['items'],
          message: t(
            'The model list is too large. Remove some models and try again.'
          ),
        })
      }
    })
}

export type UserChannelChainFormValues = z.infer<
  ReturnType<typeof getUserChannelChainFormSchema>
>
