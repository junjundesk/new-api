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

import type { UsageLog } from '../../data/schema'
import {
  formatCacheHitRate,
  getCacheHitRateLevel,
  getLogCacheUsage,
  getPageCacheRate,
} from '../cache-rate'

function makeLog(overrides: Partial<UsageLog> = {}): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1700000000,
    type: 2,
    content: '',
    username: 'tester',
    token_name: 'tok',
    model_name: 'gpt-4o',
    quota: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    use_time: 0,
    is_stream: false,
    channel: 0,
    channel_name: '',
    token_id: 0,
    group: '',
    ip: '',
    other: '',
    request_id: '',
    upstream_request_id: '',
    ...overrides,
  }
}

function withOther(values: Record<string, unknown>): string {
  return JSON.stringify(values)
}

describe('page cache rate', () => {
  test('returns null when the page has no input tokens', () => {
    assert.equal(getPageCacheRate([]), null)
    assert.equal(getPageCacheRate([makeLog(), makeLog({ type: 5 })]), null)
  })

  test('weights OpenAI-style rows by total input tokens', () => {
    const logs = [
      makeLog({ prompt_tokens: 1000, other: withOther({ cache_tokens: 500 }) }),
      makeLog({ prompt_tokens: 500, other: withOther({ cache_tokens: 250 }) }),
    ]

    assert.equal(getPageCacheRate(logs), 50)
  })

  test('counts Claude cache reads and writes in the input total', () => {
    const logs = [
      makeLog({
        prompt_tokens: 100,
        other: withOther({ claude: true, cache_tokens: 900 }),
      }),
    ]

    assert.equal(getPageCacheRate(logs), 90)
  })

  test('weights mixed provider rows by their rebuilt input totals', () => {
    const logs = [
      makeLog({ prompt_tokens: 1000, other: withOther({ cache_tokens: 500 }) }),
      makeLog({
        prompt_tokens: 100,
        other: withOther({ claude: true, cache_tokens: 900 }),
      }),
    ]

    assert.equal(getPageCacheRate(logs), 70)
  })

  test('uses the larger of the cache-creation total and the 5m/1h split', () => {
    const splitRows = [
      makeLog({
        prompt_tokens: 100,
        other: withOther({
          claude: true,
          cache_tokens: 100,
          cache_creation_tokens: 0,
          cache_creation_tokens_5m: 200,
          cache_creation_tokens_1h: 300,
        }),
      }),
    ]
    const totalRows = [
      makeLog({
        prompt_tokens: 100,
        other: withOther({
          claude: true,
          cache_tokens: 100,
          cache_creation_tokens: 600,
          cache_creation_tokens_5m: 200,
          cache_creation_tokens_1h: 300,
        }),
      }),
    ]

    assert.equal(getPageCacheRate(splitRows)?.toFixed(4), '14.2857')
    assert.equal(getPageCacheRate(totalRows), 12.5)
  })

  test('ignores malformed and negative cache values', () => {
    const logs = [
      makeLog({ prompt_tokens: 100, other: 'not-json' }),
      makeLog({
        prompt_tokens: 100,
        other: withOther({ cache_tokens: -20, cache_creation_tokens: '800' }),
      }),
    ]

    const rate = getPageCacheRate(logs)
    assert.ok(rate !== null)
    assert.equal(rate, 0)
  })

  test('clamps a rate above 100 percent when upstream data is inconsistent', () => {
    const logs = [
      makeLog({ prompt_tokens: 100, other: withOther({ cache_tokens: 150 }) }),
    ]

    assert.equal(getPageCacheRate(logs), 100)
  })
})

describe('log cache usage', () => {
  test('rebuilds the complete input volume for a Claude row', () => {
    const log = makeLog({
      prompt_tokens: 17176,
      other: withOther({ claude: true, cache_tokens: 274623 }),
    })

    const usage = getLogCacheUsage(log)
    assert.ok(usage !== null)
    assert.equal(usage.inputTokens, 291799)
    assert.equal(usage.cachedTokens, 274623)
    assert.equal(usage.rate.toFixed(2), '94.11')
  })

  test('treats prompt tokens as the whole input for a non-Claude row', () => {
    const log = makeLog({
      prompt_tokens: 1000,
      other: withOther({ cache_tokens: 500 }),
    })

    const usage = getLogCacheUsage(log)
    assert.ok(usage !== null)
    assert.equal(usage.inputTokens, 1000)
    assert.equal(usage.cachedTokens, 500)
    assert.equal(usage.rate, 50)
  })

  test('counts cache writes in the input volume without counting them as hits', () => {
    const log = makeLog({
      prompt_tokens: 100,
      other: withOther({
        claude: true,
        cache_tokens: 900,
        cache_creation_tokens_5m: 200,
        cache_creation_tokens_1h: 300,
      }),
    })

    const usage = getLogCacheUsage(log)
    assert.ok(usage !== null)
    assert.equal(usage.inputTokens, 1500)
    assert.equal(usage.cachedTokens, 900)
    assert.equal(usage.rate, 60)
  })

  test('returns null when the row carries no input tokens', () => {
    assert.equal(getLogCacheUsage(makeLog()), null)
    assert.equal(
      getLogCacheUsage(
        makeLog({ prompt_tokens: 0, other: withOther({ cache_tokens: 10 }) })
      ),
      null
    )
  })

  test('returns null when cache reads exceed the input volume', () => {
    const log = makeLog({
      prompt_tokens: 100,
      other: withOther({ cache_tokens: 150 }),
    })

    assert.equal(getLogCacheUsage(log), null)
  })

  test('returns null for unusable counts', () => {
    assert.equal(
      getLogCacheUsage(
        makeLog({
          prompt_tokens: 100,
          other: withOther({ cache_tokens: 10.5 }),
        })
      ),
      null
    )
    assert.equal(
      getLogCacheUsage(
        makeLog({
          prompt_tokens: 2000000000,
          other: withOther({ cache_tokens: 10 }),
        })
      ),
      null
    )
    assert.equal(
      getLogCacheUsage(
        makeLog({ prompt_tokens: 100, other: withOther({ cache_tokens: -10 }) })
      )?.cachedTokens,
      0
    )
  })
})

describe('cache hit rate tone', () => {
  test('greens above 90 percent', () => {
    assert.equal(getCacheHitRateLevel(90.01), 'success-subtle')
    assert.equal(getCacheHitRateLevel(100), 'success-subtle')
  })

  test('yellows from 50 to 90 percent', () => {
    assert.equal(getCacheHitRateLevel(90), 'warning-subtle')
    assert.equal(getCacheHitRateLevel(50), 'warning-subtle')
  })

  test('reds below 50 percent', () => {
    assert.equal(getCacheHitRateLevel(49.99), 'danger-subtle')
    assert.equal(getCacheHitRateLevel(0), 'danger-subtle')
  })
})

describe('cache hit rate formatting', () => {
  test('prints one decimal', () => {
    const usage = { cachedTokens: 274623, inputTokens: 291799, rate: 94.1394 }
    assert.equal(formatCacheHitRate(usage, 'en'), '94.1%')
  })

  test('prints a below-floor rate as less than 0.1 percent', () => {
    const usage = { cachedTokens: 1, inputTokens: 100000, rate: 0.001 }
    assert.equal(formatCacheHitRate(usage, 'en'), '<0.1%')
  })

  test('prints a rate above a floored 90 percent as greater than 90.0 percent', () => {
    const usage = { cachedTokens: 90040, inputTokens: 100000, rate: 90.04 }
    assert.equal(formatCacheHitRate(usage, 'en'), '>90.0%')
  })

  test('keeps an exact 90 percent rate unqualified', () => {
    const usage = { cachedTokens: 900, inputTokens: 1000, rate: 90 }
    assert.equal(formatCacheHitRate(usage, 'en'), '90.0%')
  })
})
