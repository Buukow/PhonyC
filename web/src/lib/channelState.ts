export type ChannelStateInput = {
  enabled: boolean
  temp_disabled?: boolean
}

export function channelPresentation(ch: ChannelStateInput) {
  if (!ch.enabled) {
    return {
      label: '停用' as const,
      tone: 'warn' as const,
      actionLabel: '启用' as const,
      actionPayload: { enabled: true, temp_disabled: false },
    }
  }
  if (ch.temp_disabled) {
    return {
      label: '临时禁用' as const,
      tone: 'warn' as const,
      actionLabel: '启用' as const,
      actionPayload: { enabled: true, temp_disabled: false },
    }
  }
  return {
    label: '启用' as const,
    tone: undefined,
    actionLabel: '停用' as const,
    actionPayload: { enabled: false, temp_disabled: false },
  }
}
