import { useState } from 'react'
import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ChannelModelEditor, { createModelDraft, type ModelDraft } from './ChannelModelEditor'

afterEach(cleanup)

function Harness() {
  const [models, setModels] = useState<ModelDraft[]>([
    createModelDraft({ client_model: 'client', upstream_model: 'upstream', rewrite_model: true, enabled: true }),
  ])
  return <ChannelModelEditor models={models} onChange={setModels} />
}

describe('ChannelModelEditor stable editable rows', () => {
  it.each([
    ['客户端模型', 'client-xyz'],
    ['上游模型', 'upstream-xyz'],
  ])('keeps focus while typing in %s', async (placeholder, expected) => {
    const user = userEvent.setup()
    render(<Harness />)
    const input = screen.getByPlaceholderText(placeholder) as HTMLInputElement
    input.focus()
    await user.type(input, '-xyz')
    expect(document.activeElement).toBe(input)
    expect(screen.getByPlaceholderText(placeholder)).toBe(input)
    expect(input.value).toBe(expected)
  })
})
