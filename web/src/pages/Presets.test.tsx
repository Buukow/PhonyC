import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { api } from '@/lib/api'
import Presets from './Presets'

const preset = {
  id: 7,
  name: 'Tree preset',
  version_label: '1.0',
  description: 'tree editor test',
  builtin: false,
  rule_json: JSON.stringify({
    schema_version: 1,
    headers: {
      'X-Test': {
        value: { session: { id: 'abc' }, version: '1.0' },
        fill_missing: false,
        children_fill_missing: {},
      },
    },
    remove_headers: [],
    generators: {},
  }),
}

vi.mock('@/lib/api', () => ({ api: vi.fn() }))

afterEach(() => { cleanup(); vi.mocked(api).mockReset() })

function useDefaultAPI() {
  vi.mocked(api).mockImplementation(async (path: string, init?: RequestInit) => {
    if (path === '/api/presets' && !init?.method) return { items: [preset] }
    if (path === '/api/presets/validate') {
      const body = JSON.parse(String(init?.body || '{}'))
      return { rule_json: body.rule_json }
    }
    if (path === '/api/presets' && init?.method === 'POST') {
      return { id: 8, ...JSON.parse(String(init.body || '{}')) }
    }
    return {}
  })
}

async function openEditor() {
  useDefaultAPI()
  const user = userEvent.setup()
  render(<Presets />)
  await user.click(await screen.findByRole('button', { name: '编辑' }))
  return user
}

function rowFor(key: string) {
  const keyNode = screen.getByTitle(key)
  return keyNode.parentElement?.parentElement as HTMLElement
}

describe('preset visual tree editor', () => {
  it('shows keys and values as an indented expandable tree', async () => {
    const user = await openEditor()

    expect(screen.getByText('headers')).toBeTruthy()
    expect(screen.getByText('对象（1项）')).toBeTruthy()
    expect(screen.getByTitle('X-Test')).toBeTruthy()
    expect(rowFor('X-Test').style.paddingLeft).toBe('24px')
    expect(screen.getByTitle('session')).toBeTruthy()
    expect(within(rowFor('version')).getByRole('textbox')).toHaveProperty('value', '1.0')
    expect(rowFor('session').style.paddingLeft).toBe('48px')
    expect(screen.queryByTitle('id')).toBeNull()

    await user.click(within(rowFor('session')).getByRole('button', { name: '展开' }))
    expect(screen.getByTitle('id')).toBeTruthy()
    expect(screen.getByDisplayValue('abc')).toBeTruthy()
    expect(rowFor('id').style.paddingLeft).toBe('72px')
  })

  it('keeps leaf input focus while typing', async () => {
    const user = await openEditor()
    const input = within(rowFor('version')).getByRole('textbox') as HTMLInputElement

    input.focus()
    await user.type(input, '.1')

    expect(document.activeElement).toBe(input)
    expect(input.value).toBe('1.0.1')
  })

  it('propagates parent fill state without changing the parent from a child edit', async () => {
    const user = await openEditor()
    const rootCheckbox = within(rowFor('X-Test')).getByRole('checkbox') as HTMLInputElement
    const sessionCheckbox = within(rowFor('session')).getByRole('checkbox') as HTMLInputElement

    await user.click(rootCheckbox)
    expect(rootCheckbox.checked).toBe(true)
    expect(sessionCheckbox.checked).toBe(true)

    await user.click(within(rowFor('session')).getByRole('button', { name: '展开' }))
    const idCheckbox = within(rowFor('id')).getByRole('checkbox') as HTMLInputElement
    expect(idCheckbox.checked).toBe(true)

    await user.click(idCheckbox)
    expect(idCheckbox.checked).toBe(false)
    expect(rootCheckbox.checked).toBe(true)

    await user.click(rootCheckbox)
    expect(rootCheckbox.checked).toBe(false)
    expect(sessionCheckbox.checked).toBe(false)
    expect(idCheckbox.checked).toBe(false)
  })

  it('keeps pasted JSON when a new preset switches to visual mode before saving', async () => {
    useDefaultAPI()
    const user = userEvent.setup()
    render(<Presets />)
    await user.click(await screen.findByRole('button', { name: '新建预设' }))

    const nameInput = screen.getAllByRole('textbox')[0]
    await user.type(nameInput, 'Nested create')
    await user.click(screen.getByRole('button', { name: 'JSON 编辑' }))

    const nested = {
      schema_version: 1,
      headers: {
        'X-New': {
          value: { level1: { level2: { value: 'saved' } } },
          fill_missing: false,
        },
      },
      remove_headers: [],
      generators: {},
    }
    const jsonEditor = screen.getAllByRole('textbox').find((element) => element.tagName === 'TEXTAREA') as HTMLTextAreaElement
    fireEvent.change(jsonEditor, { target: { value: JSON.stringify(nested, null, 2) } })
    await user.click(screen.getByRole('button', { name: '可视化编辑' }))

    expect(screen.getByTitle('X-New')).toBeTruthy()
    expect(screen.getByTitle('level1')).toBeTruthy()
    await user.click(screen.getByRole('button', { name: '保存' }))

    const createCall = vi.mocked(api).mock.calls.find(([path, init]) => path === '/api/presets' && init?.method === 'POST')
    expect(createCall).toBeTruthy()
    const payload = JSON.parse(String(createCall?.[1]?.body))
    expect(JSON.parse(payload.rule_json)).toEqual(nested)
  })

  it('configures interval and increment generators with valid defaults', async () => {
    useDefaultAPI()
    const user = userEvent.setup()
    render(<Presets />)
    await user.click(await screen.findByRole('button', { name: '新建预设' }))
    await user.click(screen.getByRole('button', { name: '添加生成器' }))

    let selects = screen.getAllByRole('combobox') as HTMLSelectElement[]
    const typeSelect = selects[0]
    const charsetSelect = selects[1]
    const modeSelect = selects[2]
    await user.selectOptions(modeSelect, 'increment')

    expect(typeSelect.value).toBe('random')
    expect(charsetSelect.value).toBe('digits')
    expect(charsetSelect.disabled).toBe(true)
    expect(screen.getByDisplayValue('1')).toBeTruthy()
    selects = screen.getAllByRole('combobox') as HTMLSelectElement[]
    expect(selects[3].value).toBe('wrap')

    await user.selectOptions(modeSelect, 'interval')
    expect(screen.getByDisplayValue('30m')).toBeTruthy()
  })
})
