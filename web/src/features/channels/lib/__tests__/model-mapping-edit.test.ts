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

import {
  getExposedNames,
  parseExposedNameInput,
  parseModelMappingEntries,
  setUpstreamExposedName,
} from '../model-mapping-edit'

describe('parseModelMappingEntries', () => {
  it('returns an empty map for a blank mapping', () => {
    expect(parseModelMappingEntries('')).toEqual({})
    expect(parseModelMappingEntries('   ')).toEqual({})
  })

  it('normalizes single names, lists, and blank candidates', () => {
    const mapping = JSON.stringify({
      ' gpt-4o ': 'upstream-a',
      'gpt-4o-mini': ['upstream-b', ' ', 'upstream-c'],
      empty: [],
    })

    expect(parseModelMappingEntries(mapping)).toEqual({
      'gpt-4o': ['upstream-a'],
      'gpt-4o-mini': ['upstream-b', 'upstream-c'],
    })
  })

  it('rejects values that are not a mapping object', () => {
    expect(parseModelMappingEntries('{')).toBeNull()
    expect(parseModelMappingEntries('[]')).toBeNull()
    expect(parseModelMappingEntries('"gpt-4o"')).toBeNull()
    expect(parseModelMappingEntries('{"gpt-4o":1}')).toBeNull()
  })
})

describe('getExposedNames', () => {
  it('lists every public name that forwards to the upstream model', () => {
    const mapping = JSON.stringify({
      'public-a': 'gpt-4o',
      'public-b': 'gpt-4o',
      'public-c': ['gpt-4o', 'other'],
      unrelated: 'gpt-4o-mini',
    })

    expect(getExposedNames(mapping, 'gpt-4o')).toEqual([
      'public-a',
      'public-b',
      'public-c',
    ])
    expect(getExposedNames(mapping, ' gpt-4o ')).toEqual([
      'public-a',
      'public-b',
      'public-c',
    ])
  })

  it('returns an empty list when the upstream model is not exposed', () => {
    expect(getExposedNames('{}', 'gpt-4o')).toEqual([])
  })

  it('returns null when the stored mapping is invalid', () => {
    expect(getExposedNames('not-json', 'gpt-4o')).toBeNull()
  })
})

describe('setUpstreamExposedName', () => {
  it('exposes an upstream model under a new public name', () => {
    const result = setUpstreamExposedName('', 'gpt-4o', '', 'public-a')

    expect(result.conflict).toBe(false)
    expect(result.mapping && JSON.parse(result.mapping)).toEqual({
      'public-a': 'gpt-4o',
    })
  })

  it('renames the public name of an upstream model', () => {
    const initial = JSON.stringify({ 'public-a': 'gpt-4o', keep: 'other' })
    const result = setUpstreamExposedName(
      initial,
      'gpt-4o',
      'public-a',
      'public-b'
    )

    expect(result.mapping && JSON.parse(result.mapping)).toEqual({
      keep: 'other',
      'public-b': 'gpt-4o',
    })
  })

  it('removes the public name when the input is cleared', () => {
    const initial = JSON.stringify({ 'public-a': 'gpt-4o', keep: 'other' })
    const result = setUpstreamExposedName(initial, 'gpt-4o', 'public-a', '')

    expect(result.mapping && JSON.parse(result.mapping)).toEqual({
      keep: 'other',
    })
    expect(
      setUpstreamExposedName(
        JSON.stringify({ 'public-a': 'gpt-4o' }),
        'gpt-4o',
        'public-a',
        ''
      ).mapping
    ).toBe('')
  })

  it('keeps the other upstream candidates of a shared public name', () => {
    const initial = JSON.stringify({ shared: ['gpt-4o', 'other'] })
    const result = setUpstreamExposedName(
      initial,
      'gpt-4o',
      'shared',
      'public-a'
    )

    expect(result.mapping && JSON.parse(result.mapping)).toEqual({
      shared: 'other',
      'public-a': 'gpt-4o',
    })
  })

  it('reports a conflict when the public name belongs to another entry', () => {
    const initial = JSON.stringify({ 'public-a': 'gpt-4o', taken: 'other' })
    const result = setUpstreamExposedName(
      initial,
      'gpt-4o',
      'public-a',
      'taken'
    )

    expect(result).toEqual({ mapping: null, conflict: true })
  })

  it('keeps the mapping unchanged when the name is untouched', () => {
    const initial = JSON.stringify({ 'public-a': 'gpt-4o' })
    const result = setUpstreamExposedName(
      initial,
      'gpt-4o',
      'public-a',
      ' public-a '
    )

    expect(result.conflict).toBe(false)
    expect(result.mapping && JSON.parse(result.mapping)).toEqual({
      'public-a': 'gpt-4o',
    })
  })

  it('refuses to edit an invalid mapping or a blank upstream model', () => {
    expect(setUpstreamExposedName('not-json', 'gpt-4o', '', 'a')).toEqual({
      mapping: null,
      conflict: false,
    })
    expect(setUpstreamExposedName('{}', '  ', '', 'a')).toEqual({
      mapping: null,
      conflict: false,
    })
  })
})

describe('parseExposedNameInput', () => {
  it('trims a single public model name', () => {
    expect(parseExposedNameInput(' public-a ')).toEqual({
      name: 'public-a',
      error: null,
    })
  })

  it('allows an empty value so the mapping can be removed', () => {
    expect(parseExposedNameInput('')).toEqual({ name: '', error: null })
    expect(parseExposedNameInput('   ')).toEqual({ name: '', error: null })
  })

  it('rejects names with whitespace, commas, or over 255 characters', () => {
    expect(parseExposedNameInput('public a').error).toBe('invalid-name')
    expect(parseExposedNameInput('public-a,public-b').error).toBe(
      'invalid-name'
    )
    expect(parseExposedNameInput('x'.repeat(256)).error).toBe('invalid-name')
    expect(parseExposedNameInput('x'.repeat(255)).error).toBeNull()
  })
})
