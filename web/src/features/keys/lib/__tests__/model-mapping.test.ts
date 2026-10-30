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
import { describe, expect, it } from 'vitest'

import type { TokenModelMapping } from '../../types'
import { getModelMappingNotices } from '../model-mapping'

function mapping(source: string, target: string): TokenModelMapping {
  return { source_model: source, target_model: target }
}

describe('getModelMappingNotices', () => {
  it('flags a source name that the selected group also serves', () => {
    expect(
      getModelMappingNotices([mapping('gpt-4', 'gpt-4o')], ['gpt-4', 'gpt-4o'])
    ).toEqual([
      {
        index: 0,
        code: 'shadows-group-model',
        model: 'gpt-4',
        target: 'gpt-4o',
      },
    ])
  })

  it('ignores alias names that are not served by the group', () => {
    expect(
      getModelMappingNotices([mapping('client-alias', 'gpt-4o')], ['gpt-4o'])
    ).toEqual([])
  })

  it('ignores self-mappings because they only override reasoning effort', () => {
    expect(
      getModelMappingNotices([mapping('gpt-4', 'gpt-4')], ['gpt-4'])
    ).toEqual([])
  })

  it('reports one notice per row with the row index and target', () => {
    expect(
      getModelMappingNotices(
        [mapping('alias', 'gpt-4o'), mapping('gpt-4', 'claude-sonnet-4-5')],
        ['gpt-4', 'gpt-4o', 'claude-sonnet-4-5']
      )
    ).toEqual([
      {
        index: 1,
        code: 'shadows-group-model',
        model: 'gpt-4',
        target: 'claude-sonnet-4-5',
      },
    ])
  })

  it('compares trimmed names', () => {
    expect(
      getModelMappingNotices([mapping(' gpt-4 ', ' gpt-4o ')], ['gpt-4'])
    ).toEqual([
      {
        index: 0,
        code: 'shadows-group-model',
        model: 'gpt-4',
        target: 'gpt-4o',
      },
    ])
  })

  it('stays quiet while group models are unknown', () => {
    expect(getModelMappingNotices([mapping('gpt-4', 'gpt-4o')])).toEqual([])
    expect(getModelMappingNotices([mapping('gpt-4', 'gpt-4o')], [])).toEqual([])
  })

  it('ignores rows that are still incomplete', () => {
    expect(getModelMappingNotices([mapping('gpt-4', '')], ['gpt-4'])).toEqual(
      []
    )
  })
})
