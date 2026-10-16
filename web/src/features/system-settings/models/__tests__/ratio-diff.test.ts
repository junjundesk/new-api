import { describe, expect, it } from 'vitest'

import { collectChangedOptionEntries } from '../utils'

describe('collectChangedOptionEntries', () => {
  it('returns nothing when every value matches the saved baseline', () => {
    const baseline = {
      GroupRatio: '{"vip":1}',
      MaxTokenAutoGroups: 5,
      DefaultUseAutoGroup: false,
    }
    expect(collectChangedOptionEntries({ ...baseline }, baseline)).toEqual([])
  })

  it('maps changed form fields to their option keys', () => {
    const entries = collectChangedOptionEntries(
      {
        GroupRatio: '{"vip":0.5}',
        GroupSpecialUsableGroup: '{"vip":{}}',
        MaxTokenAutoGroups: 6,
        DefaultUseAutoGroup: true,
      },
      {
        GroupRatio: '{"vip":1}',
        GroupSpecialUsableGroup: '{}',
        MaxTokenAutoGroups: 5,
        DefaultUseAutoGroup: false,
      },
      {
        GroupSpecialUsableGroup:
          'group_ratio_setting.group_special_usable_group',
      }
    )

    expect(entries).toEqual([
      { key: 'GroupRatio', value: '{"vip":0.5}' },
      {
        key: 'group_ratio_setting.group_special_usable_group',
        value: '{"vip":{}}',
      },
      { key: 'MaxTokenAutoGroups', value: 6 },
      { key: 'DefaultUseAutoGroup', value: true },
    ])
  })

  it('keeps boolean and numeric changes that are falsy', () => {
    expect(
      collectChangedOptionEntries(
        { DefaultUseAutoGroup: false, MaxTokenAutoGroups: 0 },
        { DefaultUseAutoGroup: true, MaxTokenAutoGroups: 5 }
      )
    ).toEqual([
      { key: 'DefaultUseAutoGroup', value: false },
      { key: 'MaxTokenAutoGroups', value: 0 },
    ])
  })
})
