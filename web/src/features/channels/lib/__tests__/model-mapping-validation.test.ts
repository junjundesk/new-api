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
  MAX_CANDIDATE_MODELS_PER_SOURCE,
  extractRedirectModels,
  extractMappingSourceModels,
  normalizeCandidateModels,
  validateModelMappingJson,
} from '../model-mapping-validation'

describe('normalizeCandidateModels', () => {
  test('wraps a single replacement into a list', () => {
    assert.deepEqual(normalizeCandidateModels('gpt-4o'), ['gpt-4o'])
  })

  test('keeps candidate order', () => {
    assert.deepEqual(normalizeCandidateModels(['a', 'b', 'c']), ['a', 'b', 'c'])
  })

  test('rejects values that are neither string nor string list', () => {
    assert.equal(normalizeCandidateModels(1), null)
    assert.equal(normalizeCandidateModels({ model: 'a' }), null)
    assert.equal(normalizeCandidateModels(['a', 2]), null)
  })
})

describe('extractRedirectModels', () => {
  test('expands every candidate of a multi-value entry', () => {
    const mapping = JSON.stringify({
      alias: ['candidate-a', 'candidate-b'],
      legacy: 'upstream',
    })

    assert.deepEqual(extractRedirectModels(mapping).sort(), [
      'candidate-a',
      'candidate-b',
      'upstream',
    ])
  })

  test('ignores entries that are not usable model names', () => {
    const mapping = JSON.stringify({ alias: ['candidate-a', '  ', ''] })

    assert.deepEqual(extractRedirectModels(mapping), ['candidate-a'])
  })
})

describe('validateModelMappingJson', () => {
  test('accepts a single replacement per source', () => {
    assert.deepEqual(
      validateModelMappingJson('{"gpt-4o":"gpt-4o-2024-08-06"}'),
      { valid: true }
    )
  })

  test('accepts a candidate list per source', () => {
    assert.deepEqual(
      validateModelMappingJson(
        '{"claude-3-5-sonnet":["claude-3-5-sonnet-20241022","claude-3-5-sonnet-latest"]}'
      ),
      { valid: true }
    )
  })

  test('accepts an empty mapping', () => {
    assert.deepEqual(validateModelMappingJson(''), { valid: true })
    assert.deepEqual(validateModelMappingJson('{}'), { valid: true })
  })

  test('rejects a value that is not a model name or list of model names', () => {
    const result = validateModelMappingJson('{"alias":1}')
    assert.equal(result.valid, false)
    assert.match(result.error ?? '', /model name or an array of model names/)
  })

  test('rejects a candidate list longer than the backend allows', () => {
    const candidates = Array.from(
      { length: MAX_CANDIDATE_MODELS_PER_SOURCE + 1 },
      (_, index) => `candidate-${index}`
    )
    const result = validateModelMappingJson(
      JSON.stringify({ alias: candidates })
    )

    assert.equal(result.valid, false)
    assert.match(result.error ?? '', /candidate models per source/)
  })

  test('rejects model names containing spaces', () => {
    const result = validateModelMappingJson(
      '{"alias":["bad model","ok-model"]}'
    )
    assert.equal(result.valid, false)
    assert.match(result.error ?? '', /must not contain spaces/)
  })

  test('drops blank candidates instead of rejecting them', () => {
    assert.deepEqual(validateModelMappingJson('{"alias":["ok-model","  "]}'), {
      valid: true,
    })
  })
})

describe('extractMappingSourceModels', () => {
  test('lists the keys of a multi-value mapping', () => {
    const mapping = JSON.stringify({
      'alias-a': ['candidate-a', 'candidate-b'],
      'alias-b': 'upstream',
    })

    assert.deepEqual(extractMappingSourceModels(mapping).sort(), [
      'alias-a',
      'alias-b',
    ])
  })
})
