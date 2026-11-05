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
import { describe, expect, test } from 'vitest'

import type { LogOtherData } from '../../types'
import { getLogChannelGroup } from '../format'

const CHANNEL_ID = 7

describe('log channel group', () => {
  test.each<{
    name: string
    group: string
    channelGroups: string[] | undefined
    other: LogOtherData | null
    expected: string
  }>([
    {
      name: 'a consume log links the recorded group when the channel belongs to it',
      group: 'vip',
      channelGroups: ['default', 'vip'],
      other: { group_ratio: 1.5 },
      expected: 'vip',
    },
    {
      name: 'a legacy log without a group column uses the group in other',
      group: '',
      channelGroups: ['default', 'legacy'],
      other: { group: 'legacy' },
      expected: 'legacy',
    },
    {
      name: 'a recorded group the channel does not belong to falls back to the channel first group',
      group: 'customer-priority',
      channelGroups: ['vip', 'backup'],
      other: null,
      expected: 'vip',
    },
    {
      name: 'a failed fallback prefers the target group of the attempt that used the channel',
      group: 'default',
      channelGroups: ['default', 'backup'],
      other: {
        admin_info: {
          fallback_attempts: [
            { index: 1, group: 'backup', model: 'gpt-4o', channel_id: 7 },
          ],
        },
      },
      expected: 'backup',
    },
    {
      name: 'a channel attempted in several fallback groups prefers the latest attempt',
      group: 'default',
      channelGroups: ['backup', 'reserve'],
      other: {
        admin_info: {
          fallback_attempts: [
            { index: 1, group: 'backup', model: 'gpt-4o', channel_id: 7 },
            { index: 2, group: 'reserve', model: 'gpt-4o', channel_id: 7 },
          ],
        },
      },
      expected: 'reserve',
    },
    {
      name: 'an auto error log whose channel came from affinity prefers the affinity group',
      group: 'auto',
      channelGroups: ['default', 'vip'],
      other: {
        admin_info: {
          channel_affinity: {
            rule_name: 'sticky-session',
            using_group: 'auto',
            selected_group: 'vip',
            channel_id: 7,
          },
        },
      },
      expected: 'vip',
    },
    {
      name: 'attempts and affinity hits naming another channel leave the recorded group in place',
      group: 'default',
      channelGroups: ['default', 'backup', 'vip'],
      other: {
        admin_info: {
          fallback_attempts: [
            { index: 1, group: 'backup', model: 'gpt-4o', channel_id: 9 },
          ],
          channel_affinity: { selected_group: 'vip', channel_id: 9 },
        },
      },
      expected: 'default',
    },
    {
      name: 'a channel whose groups are unknown resolves to no group',
      group: 'vip',
      channelGroups: undefined,
      other: null,
      expected: '',
    },
  ])('$name', (testCase) => {
    expect(
      getLogChannelGroup(
        {
          channel: CHANNEL_ID,
          channel_groups: testCase.channelGroups,
          group: testCase.group,
        },
        testCase.other
      )
    ).toBe(testCase.expected)
  })

  test('a log without a channel resolves to no group', () => {
    expect(
      getLogChannelGroup(
        { channel: 0, channel_groups: ['vip'], group: 'vip' },
        null
      )
    ).toBe('')
  })
})
