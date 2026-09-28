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

import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  GROUP_FILTER_ALL_VALUE,
  buildGroupFilterOptions,
} from '../group-filter'

describe('log group filter options', () => {
  test('offers only the all-groups entry while the group list is still loading', () => {
    assert.deepEqual(buildGroupFilterOptions(undefined, 'All Groups'), [
      { value: GROUP_FILTER_ALL_VALUE, label: 'All Groups' },
    ])
  })

  test('lists the model groups in stable order after the all-groups entry', () => {
    const options = buildGroupFilterOptions(
      ['VIP', 'default', 'k12', '柠檬公益'],
      'All Groups'
    )

    assert.deepEqual(
      options.map((option) => option.value),
      [GROUP_FILTER_ALL_VALUE, 'VIP', 'default', 'k12', '柠檬公益']
    )
    assert.equal(options[0].label, 'All Groups')
    assert.equal(options[1].label, 'VIP')
  })

  test('ignores blank group names coming from the group list', () => {
    const options = buildGroupFilterOptions(['default', ''], 'All Groups')

    assert.deepEqual(
      options.map((option) => option.value),
      [GROUP_FILTER_ALL_VALUE, 'default']
    )
  })

  test('still accepts the usable-group map used by the non-admin fallback', () => {
    const options = buildGroupFilterOptions(
      { vip: {}, default: {} },
      'All Groups'
    )

    assert.deepEqual(
      options.map((option) => option.value),
      [GROUP_FILTER_ALL_VALUE, 'default', 'vip']
    )
  })
})
