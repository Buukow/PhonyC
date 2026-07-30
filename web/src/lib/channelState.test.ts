import { describe, expect, it } from 'vitest'
import { channelPresentation } from './channelState'

describe('channelPresentation', () => {
  it.each([
    [{ enabled: false, temp_disabled: false }, '停用', '启用', { enabled: true, temp_disabled: false }],
    [{ enabled: false, temp_disabled: true }, '停用', '启用', { enabled: true, temp_disabled: false }],
    [{ enabled: true, temp_disabled: false }, '启用', '停用', { enabled: false, temp_disabled: false }],
    [{ enabled: true, temp_disabled: true }, '临时禁用', '启用', { enabled: true, temp_disabled: false }],
  ])('derives one exact state and action for %o', (channel, label, actionLabel, actionPayload) => {
    const result = channelPresentation(channel)
    expect(result.label).toBe(label)
    expect(result.label).not.toBe('测活临时禁用')
    expect(result.actionLabel).toBe(actionLabel)
    expect(result.actionPayload).toEqual(actionPayload)
  })
})
