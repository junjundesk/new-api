import { describe, expect, it } from 'vitest'

import { channelSchema } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from '../channel-form'

const form = () => ({
  ...CHANNEL_FORM_DEFAULT_VALUES,
  name: 'Upstream',
  models: 'public-model,other-model',
  key: 'test-key',
})

function channel(setting: string) {
  return channelSchema.parse({
    id: 1,
    type: 1,
    key: '',
    status: 1,
    name: 'Upstream',
    created_time: 0,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
    models: 'public-model,other-model',
    setting,
  })
}

describe('channel model billing overrides', () => {
  it('preserves free and explicit one ratios when creating and editing', () => {
    const data = {
      ...form(),
      model_group_ratios: [
        { model: 'public-model', ratio: '0' },
        { model: 'other-model', ratio: '1' },
      ],
    }
    expect(channelFormSchema.safeParse(data).success).toBe(true)
    for (const payload of [
      transformFormDataToCreatePayload(data).channel,
      transformFormDataToUpdatePayload(data, 1),
    ]) {
      expect(JSON.parse(payload.setting!).model_group_ratios).toEqual({
        'public-model': 0,
        'other-model': 1,
      })
    }
  })

  it('clears sparse overrides on blank or deleted rows without losing other settings', () => {
    const defaults = transformChannelToFormDefaults(
      channel(
        JSON.stringify({
          proxy: 'https://proxy.example',
          custom_flag: true,
          model_group_ratios: { 'public-model': 0.8 },
        })
      )
    )
    expect(defaults.model_group_ratios).toEqual([
      { model: 'public-model', ratio: '0.8' },
    ])
    for (const rows of [[], [{ model: 'public-model', ratio: ' ' }]]) {
      const payload = transformFormDataToUpdatePayload(
        { ...defaults, model_group_ratios: rows },
        1
      )
      const setting = JSON.parse(payload.setting!)
      expect(setting).not.toHaveProperty('model_group_ratios')
      expect(setting.proxy).toBe('https://proxy.example')
      expect(setting.custom_flag).toBe(true)
      expect(
        transformChannelToFormDefaults(channel(payload.setting!))
          .model_group_ratios
      ).toEqual([])
    }
  })

  it.each(['-1', '1000.01', 'NaN', 'Infinity', 'null', 'abc'])(
    'rejects invalid ratio %s',
    (ratio) => {
      expect(
        channelFormSchema.safeParse({
          ...form(),
          model_group_ratios: [{ model: 'public-model', ratio }],
        }).success
      ).toBe(false)
    }
  )

  it('rejects duplicate or missing model names and accepts the maximum ratio', () => {
    expect(
      channelFormSchema.safeParse({
        ...form(),
        model_group_ratios: [{ model: '', ratio: '0.8' }],
      }).success
    ).toBe(false)
    expect(
      channelFormSchema.safeParse({
        ...form(),
        model_group_ratios: [
          { model: 'public-model', ratio: '0.8' },
          { model: 'public-model', ratio: '1' },
        ],
      }).success
    ).toBe(false)
    expect(
      channelFormSchema.safeParse({
        ...form(),
        model_group_ratios: [{ model: 'public-model', ratio: '1000' }],
      }).success
    ).toBe(true)
  })

  it('keeps legacy settings inherited and never silently turns null into free', () => {
    expect(
      transformChannelToFormDefaults(channel('{}')).model_group_ratios
    ).toEqual([])
    const defaults = transformChannelToFormDefaults(
      channel('{"model_group_ratios":{"public-model":null}}')
    )
    expect(channelFormSchema.safeParse(defaults).success).toBe(false)
  })
})
