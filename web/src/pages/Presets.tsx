import { FormEvent, useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { Badge, Button, Card, Input, Label, PageHeader, Select, Table, Textarea } from '@/components/ui'

type NodeRule = { value: any; fill_missing?: boolean; children_fill_missing?: Record<string, boolean> }
type PresetDoc = { schema_version: number; headers: Record<string, NodeRule>; remove_headers: string[]; generators: Record<string, any> }

const emptyDoc: PresetDoc = { schema_version: 1, headers: {}, remove_headers: [], generators: {} }

const builtinDisplay: Record<string, { name: string; description: string }> = {
  'codex-tui': {
    name: 'Codex 基础',
    description: '模拟 Codex 客户端的基础请求头',
  },
  'claude-cli': {
    name: 'Claude 基础',
    description: '模拟 Claude Code 客户端的基础请求头',
  },
  'codex-enhanced': {
    name: 'Codex 增强',
    description: '模拟 Codex 客户端的完整请求头与动态会话信息，强制覆盖同名请求头',
  },
  'claude-enhanced': {
    name: 'Claude 增强',
    description: '模拟 Claude Code 客户端的完整请求头与动态会话信息，强制覆盖同名请求头',
  },
}

function presetDisplay(preset: any) {
  return preset.builtin ? builtinDisplay[preset.name] || { name: preset.name, description: preset.description } : { name: preset.name, description: preset.description }
}

function legacyToDoc(p: any): PresetDoc {
  try {
    if (p.rule_json) return JSON.parse(p.rule_json)
    const values = JSON.parse(p.headers_json || '{}')
    return { schema_version: 1, headers: Object.fromEntries(Object.entries(values).map(([key, value]) => [key, { value, fill_missing: false }])), remove_headers: JSON.parse(p.remove_headers || '[]'), generators: {} }
  } catch {
    return emptyDoc
  }
}

function TreeNode({
  keyName,
  value,
  path,
  depth,
  fillMissing,
  explicit,
  parentFillMissing,
  fillMap,
  onValueChange,
  onFillChange,
  onResetFill,
  onDelete,
}: {
  keyName: string
  value: any
  path: string
  depth: number
  fillMissing: boolean
  explicit: boolean
  parentFillMissing: boolean
  fillMap: Record<string, boolean>
  onValueChange: (path: string, value: any) => void
  onFillChange: (path: string, value: boolean) => void
  onResetFill: (path: string) => void
  onDelete?: () => void
}) {
  const isObject = value !== null && typeof value === 'object'
  const entries = isObject ? (Array.isArray(value) ? value.map((v, i) => [String(i), v] as const) : Object.entries(value)) : []
  const [open, setOpen] = useState(depth < 2)
  const summary = Array.isArray(value) ? `数组（${value.length}项）` : isObject ? '对象' : ''
  const hasExplicitDescendant = Object.keys(fillMap).some((key) => path ? key.startsWith(`${path}.`) : true)
  return (
    <div className="space-y-1">
      <div className="grid grid-cols-[minmax(0,1fr)_minmax(180px,1.5fr)_auto_auto] gap-2 items-center min-h-9" style={{ paddingLeft: `${depth * 24}px` }}>
        <div className="flex items-center gap-1 min-w-0">
          {isObject ? (
            <button type="button" aria-label={open ? '收起' : '展开'} className="w-5 h-5 shrink-0 text-gray-500 hover:text-primary" onClick={() => setOpen(!open)}>{open ? '▾' : '▸'}</button>
          ) : <span className="w-5 shrink-0" />}
          <span className="font-mono text-xs text-gray-700 truncate" title={keyName}>{keyName}</span>
        </div>
        <div className="min-w-0">
          {isObject ? <span className="text-xs text-gray-400">{summary}</span> : <Input value={value == null ? '' : String(value)} onChange={(e) => onValueChange(path, e.target.value)} className="font-mono text-xs" />}
        </div>
        <div className="flex items-center gap-1 whitespace-nowrap">
          <button type="button" role="switch" aria-checked={fillMissing} aria-label={`${keyName} ${fillMissing ? '缺失补全' : '强制覆盖'}`} className={`relative inline-flex h-6 w-12 items-center rounded-full transition-colors ${fillMissing ? 'bg-accent' : 'bg-primary'}`} onClick={() => onFillChange(path, !fillMissing)}>
            <span className={`inline-block h-4 w-4 rounded-full bg-white shadow transition-transform ${fillMissing ? 'translate-x-1' : 'translate-x-7'}`} />
          </button>
          <span className="text-[11px] text-gray-500">{explicit ? (fillMissing ? '缺失补全' : '强制覆盖') : `继承：${parentFillMissing ? '缺失补全' : '强制覆盖'}`}</span>
          {hasExplicitDescendant && <span className="text-[11px] text-accent">含自定义子项</span>}
          {explicit && path && <button type="button" className="text-[11px] text-primary hover:underline" onClick={() => onResetFill(path)}>恢复继承</button>}
        </div>
        {onDelete ? <Button type="button" variant="danger" onClick={onDelete}>删除</Button> : <span />}
      </div>
      {open && entries.map(([childKey, childValue]) => {
        const childPath = path ? `${path}.${childKey}` : childKey
        const childExplicit = Object.prototype.hasOwnProperty.call(fillMap, childPath)
        const childFill = childExplicit ? !!fillMap[childPath] : fillMissing
        return <TreeNode key={childPath} keyName={childKey} value={childValue} path={childPath} depth={depth + 1} fillMissing={childFill} explicit={childExplicit} parentFillMissing={fillMissing} fillMap={fillMap} onValueChange={onValueChange} onFillChange={onFillChange} onResetFill={onResetFill} />
      })}
    </div>
  )
}

function collectPaths(value: any, base = ''): string[] {
  if (!value || typeof value !== 'object') return []
  const entries = Array.isArray(value) ? value.map((v, i) => [String(i), v] as const) : Object.entries(value)
  return entries.flatMap(([key, child]) => {
    const path = base ? `${base}.${key}` : key
    return [path, ...collectPaths(child, path)]
  })
}

function PresetEditor({ initial, onClose, onSaved }: { initial: any; onClose: () => void; onSaved: () => Promise<void> }) {
  const builtIn = !!initial?.builtin
  const [name, setName] = useState(initial?.name || '')
  const [description, setDescription] = useState(initial?.description || '')
  const [versionLabel, setVersionLabel] = useState(initial?.version_label || '')
  const [doc, setDoc] = useState<PresetDoc>(() => initial ? legacyToDoc(initial) : emptyDoc)
  const [jsonText, setJsonText] = useState(() => JSON.stringify(initial ? legacyToDoc(initial) : emptyDoc, null, 2))
  const [view, setView] = useState<'visual' | 'json'>('visual')
  const [error, setError] = useState('')
  const [preview, setPreview] = useState<any>(null)
  const [headersOpen, setHeadersOpen] = useState(true)

  function setHeaderValue(header: string, value: any, path: string) {
    const root = doc.headers[header]
    if (!root) return
    const parts = path ? path.split('.') : []
    const setNested = (current: any, index: number): any => {
      if (index === parts.length) return value
      const key = parts[index]
      return Array.isArray(current) ? current.map((item, i) => (String(i) === key ? setNested(item, index + 1) : item)) : { ...current, [key]: setNested(current[key], index + 1) }
    }
    const nextValue = setNested(root.value, 0)
    const next = { ...doc, headers: { ...doc.headers, [header]: { ...root, value: nextValue } } }
    setDoc(next); setJsonText(JSON.stringify(next, null, 2))
  }

  function setFillMissing(header: string, path: string, checked: boolean) {
    const rule = doc.headers[header]
    if (!rule) return
    const parts = path ? path.split('.') : []
    const childFill = { ...(rule.children_fill_missing || {}) }
    const nextRule = path ? { ...rule, children_fill_missing: { ...childFill, [path]: checked } } : { ...rule, fill_missing: checked, children_fill_missing: childFill }
    const next = { ...doc, headers: { ...doc.headers, [header]: nextRule } }
    setDoc(next); setJsonText(JSON.stringify(next, null, 2))
  }

  function resetFillMissing(header: string, path: string) {
    const rule = doc.headers[header]
    if (!rule) return
    const childFill = { ...(rule.children_fill_missing || {}) }
    delete childFill[path]
    const next = { ...doc, headers: { ...doc.headers, [header]: { ...rule, children_fill_missing: childFill } } }
    setDoc(next); setJsonText(JSON.stringify(next, null, 2))
  }

  function addHeader() {
    let name = 'New-Header'; let i = 1
    while (doc.headers[name]) name = `New-Header-${i++}`
    const next = { ...doc, headers: { ...doc.headers, [name]: { value: '', fill_missing: false } } }
    setDoc(next); setJsonText(JSON.stringify(next, null, 2))
  }

  function addGenerator() {
    let name = 'random_code'; let i = 1
    while (doc.generators[name]) name = `random_code_${i++}`
    const next = { ...doc, generators: { ...doc.generators, [name]: { type: 'random', charset: 'alnum', length: 16, mode: 'request', exclude_ambiguous: false } } }
    setDoc(next); setJsonText(JSON.stringify(next, null, 2))
  }

  async function save(e: FormEvent) {
    e.preventDefault(); setError('')
    let nextDoc = doc
    if (view === 'json') {
      try { nextDoc = JSON.parse(jsonText) } catch (err: any) { setError(err.message || 'JSON 无效'); return }
    }
    try {
      const checked = await api<{ rule_json: string }>('/api/presets/validate', { method: 'POST', body: JSON.stringify({ rule_json: JSON.stringify(nextDoc) }) })
      const payload = { name, description, version_label: versionLabel, rule_json: checked.rule_json, headers_json: '{}', remove_headers: '[]' }
      const saved = builtIn
        ? await api<any>('/api/presets', { method: 'POST', body: JSON.stringify(payload) })
        : initial?.id
          ? await api<any>(`/api/presets/${initial.id}`, { method: 'PATCH', body: JSON.stringify(payload) })
          : await api<any>('/api/presets', { method: 'POST', body: JSON.stringify(payload) })
      if (!saved?.rule_json) throw new Error('服务未返回已保存的规则，请重试')
      if (saved.rule_json !== checked.rule_json) throw new Error('服务保存后的规则与提交内容不一致，请重试')
      setDoc(nextDoc)
      setJsonText(JSON.stringify(nextDoc, null, 2))
      await onSaved(); onClose()
    } catch (err: any) { setError(err.message || '保存失败') }
  }

  async function doPreview() {
    setError(''); try { const res = await api('/api/presets/preview', { method: 'POST', body: JSON.stringify({ rule_json: view === 'json' ? jsonText : JSON.stringify(doc), version_label: versionLabel, client_headers: {} }) }); setPreview(res) } catch (err: any) { setError(err.message || '预览失败') }
  }

  return (
    <Card className="p-6 mb-6">
      <form className="space-y-4" onSubmit={save}>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
          <div><Label>{builtIn ? '新预设名称' : '名称'}</Label><Input value={name} onChange={(e) => setName(e.target.value)} required /></div>
          <div><Label>版本标签</Label><Input value={versionLabel} onChange={(e) => setVersionLabel(e.target.value)} placeholder="例如 1.0" /></div>
          <div><Label>描述</Label><Input value={description} onChange={(e) => setDescription(e.target.value)} /></div>
        </div>
        <div className="flex gap-2 items-center border-b border-gray-100 pb-2">
          <Button type="button" variant={view === 'visual' ? 'primary' : 'secondary'} onClick={() => {
            if (view === 'json') {
              try {
                const parsed = JSON.parse(jsonText)
                setDoc(parsed)
                setJsonText(JSON.stringify(parsed, null, 2))
                setError('')
              } catch (err: any) {
                setError(err.message || 'JSON 无效，无法切换到可视化编辑')
                return
              }
            }
            setView('visual')
          }}>可视化编辑</Button>
          <Button type="button" variant={view === 'json' ? 'primary' : 'secondary'} onClick={() => { setView('json'); setJsonText(JSON.stringify(doc, null, 2)); setError('') }}>JSON 编辑</Button>
          <Button type="button" variant="ghost" onClick={addHeader}>添加 Header</Button><Button type="button" variant="ghost" onClick={addGenerator}>添加生成器</Button>
          {builtIn && <Badge tone="accent">保存为新预设</Badge>}
        </div>
        {view === 'visual' ? (
          <div className="space-y-3">
            <div className="rounded-2xl border border-gray-100 overflow-hidden">
              <div className="grid grid-cols-[minmax(0,1fr)_minmax(180px,1.5fr)_auto_auto] gap-2 items-center min-h-11 px-3 bg-canvas/70 border-b border-gray-100">
                <div className="flex items-center gap-1 min-w-0">
                  <button type="button" aria-label={headersOpen ? '收起 headers' : '展开 headers'} className="w-5 h-5 shrink-0 text-gray-500 hover:text-primary" onClick={() => setHeadersOpen(!headersOpen)}>{headersOpen ? '▾' : '▸'}</button>
                  <span className="font-mono text-xs font-semibold text-gray-700">headers</span>
                </div>
                <span className="text-xs text-gray-400">对象（{Object.keys(doc.headers).length}项）</span>
                <span className="text-[11px] text-gray-400 whitespace-nowrap">默认模式</span>
                <span className="w-14" />
              </div>
              {headersOpen && <div className="p-3 divide-y divide-gray-50">
                {Object.entries(doc.headers).map(([header, rule]) => (
                  <TreeNode
                    key={header}
                    keyName={header}
                    value={rule.value}
                    path=""
                    depth={1}
                    fillMissing={!!rule.fill_missing}
                    explicit
                    parentFillMissing={!!rule.fill_missing}
                    fillMap={rule.children_fill_missing || {}}
                    onValueChange={(path, value) => setHeaderValue(header, value, path)}
                    onFillChange={(path, checked) => setFillMissing(header, path, checked)}
                    onResetFill={(path) => resetFillMissing(header, path)}
                    onDelete={() => { const headers = { ...doc.headers }; delete headers[header]; const next = { ...doc, headers }; setDoc(next); setJsonText(JSON.stringify(next, null, 2)) }}
                  />
                ))}
                {!Object.keys(doc.headers).length && <div className="py-3 pl-6 text-sm text-gray-400">暂无 Header，点击“添加 Header”。</div>}
              </div>}
            </div>
            <div className="rounded-2xl border border-gray-100 p-3 space-y-3">
              <div className="font-medium text-sm text-gray-700">生成器</div>
              {Object.entries(doc.generators).map(([generatorName, generator]) => (
                <div key={generatorName} className="rounded-xl border border-gray-100 p-3 space-y-3">
                  <div className="grid grid-cols-1 md:grid-cols-6 gap-2 items-end">
                    <div><Label>名称</Label><Input value={generatorName} readOnly className="font-mono text-xs" /></div>
                    <div><Label>类型</Label><Select value={generator.type} onChange={(e) => { const next = { ...doc, generators: { ...doc.generators, [generatorName]: { ...generator, type: e.target.value } } }; setDoc(next); setJsonText(JSON.stringify(next, null, 2)) }}><option value="random">随机字符</option><option value="uuid" disabled={generator.mode === 'increment'}>UUID</option></Select></div>
                    <div><Label>{generator.type === 'uuid' ? 'UUID 版本' : '字符集'}</Label>{generator.type === 'uuid' ? <Select value={String(generator.version || 4)} onChange={(e) => { const next = { ...doc, generators: { ...doc.generators, [generatorName]: { ...generator, version: Number(e.target.value) } } }; setDoc(next); setJsonText(JSON.stringify(next, null, 2)) }}><option value="4">UUID v4</option><option value="7">UUID v7</option></Select> : <Select disabled={generator.mode === 'increment'} value={generator.charset || 'alnum'} onChange={(e) => { const next = { ...doc, generators: { ...doc.generators, [generatorName]: { ...generator, charset: e.target.value } } }; setDoc(next); setJsonText(JSON.stringify(next, null, 2)) }}><option value="digits">数字</option><option value="lowercase">小写字母</option><option value="uppercase">大写字母</option><option value="letters">字母</option><option value="alnum">字母数字</option></Select>}</div>
                    <div><Label>长度</Label><Input type="number" min={1} max={256} disabled={generator.type === 'uuid'} value={generator.length || 16} onChange={(e) => { const next = { ...doc, generators: { ...doc.generators, [generatorName]: { ...generator, length: Number(e.target.value) } } }; setDoc(next); setJsonText(JSON.stringify(next, null, 2)) }} /></div>
                    <div><Label>刷新模式</Label><Select value={generator.mode} onChange={(e) => { const mode = e.target.value; const changed = { ...generator, mode, type: mode === 'increment' ? 'random' : generator.type, charset: mode === 'increment' ? 'digits' : generator.charset, step: mode === 'increment' ? (generator.step || 1) : generator.step, overflow: mode === 'increment' ? (generator.overflow || 'wrap') : generator.overflow, interval: mode === 'interval' ? (generator.interval || '30m') : generator.interval }; const next = { ...doc, generators: { ...doc.generators, [generatorName]: changed } }; setDoc(next); setJsonText(JSON.stringify(next, null, 2)) }}><option value="request">每请求随机</option><option value="fixed">运行期固定</option><option value="interval">定时刷新</option><option value="increment">递增序列</option></Select></div>
                    <Button type="button" variant="danger" onClick={() => { const generators = { ...doc.generators }; delete generators[generatorName]; const next = { ...doc, generators }; setDoc(next); setJsonText(JSON.stringify(next, null, 2)) }}>删除</Button>
                  </div>
                  {generator.mode === 'interval' && <div className="max-w-xs"><Label>刷新间隔</Label><Input value={generator.interval || '30m'} onChange={(e) => { const next = { ...doc, generators: { ...doc.generators, [generatorName]: { ...generator, interval: e.target.value } } }; setDoc(next); setJsonText(JSON.stringify(next, null, 2)) }} placeholder="例如 30m、1h、1d" /></div>}
                  {generator.mode === 'increment' && <div className="grid grid-cols-1 md:grid-cols-2 gap-2 max-w-xl">
                    <div><Label>递增步长</Label><Input type="number" min={1} value={generator.step || 1} onChange={(e) => { const next = { ...doc, generators: { ...doc.generators, [generatorName]: { ...generator, step: Number(e.target.value) } } }; setDoc(next); setJsonText(JSON.stringify(next, null, 2)) }} /></div>
                    <div><Label>溢出处理</Label><Select value={generator.overflow || 'wrap'} onChange={(e) => { const next = { ...doc, generators: { ...doc.generators, [generatorName]: { ...generator, overflow: e.target.value } } }; setDoc(next); setJsonText(JSON.stringify(next, null, 2)) }}><option value="wrap">归零循环</option><option value="regenerate">重新随机</option><option value="expand">自动扩位</option><option value="error">返回错误</option></Select></div>
                  </div>}
                </div>
              ))}
              {!Object.keys(doc.generators).length && <div className="text-xs text-gray-400">暂无生成器。</div>}
            </div>
            <div className="rounded-2xl border border-gray-100 p-3 space-y-2"><div className="font-medium text-sm text-gray-700">屏蔽 Headers</div><Input value={doc.remove_headers.join(', ')} onChange={(e) => { const next = { ...doc, remove_headers: e.target.value.split(',').map((v) => v.trim()).filter(Boolean) }; setDoc(next); setJsonText(JSON.stringify(next, null, 2)) }} placeholder="逗号分隔，例如 X-Legacy, X-Debug" /></div>
          </div>
        ) : <Textarea className="min-h-[420px] font-mono text-xs" value={jsonText} onChange={(e) => setJsonText(e.target.value)} />}
        {error && <div className="text-sm text-warn whitespace-pre-wrap">{error}</div>}
        {preview && <pre className="rounded-xl bg-canvas p-3 text-xs overflow-auto max-h-64">{JSON.stringify(preview, null, 2)}</pre>}
        <div className="flex gap-2"><Button type="button" variant="secondary" onClick={doPreview}>预览结果</Button><Button type="submit">保存</Button><Button type="button" variant="secondary" onClick={onClose}>取消</Button></div>
      </form>
    </Card>
  )
}

export default function Presets() {
  const [items, setItems] = useState<any[]>([])
  const [editing, setEditing] = useState<any | null>(null)
  const [creating, setCreating] = useState(false)
  async function load() { const res = await api<{ items: any[] }>('/api/presets'); setItems(res.items || []) }
  useEffect(() => { load().catch(console.error) }, [])
  async function remove(id: number) { if (!confirm('确认删除？')) return; await api(`/api/presets/${id}`, { method: 'DELETE' }); await load() }
  return (
    <div>
      <PageHeader title="客户端预设" subtitle="Codex / Claude Code 等指纹模板，可视化编辑或 JSON 编辑" actions={<Button onClick={() => { setCreating(true); setEditing(null) }}>新建预设</Button>} />
      {(creating || editing) && <PresetEditor initial={editing} onClose={() => { setCreating(false); setEditing(null) }} onSaved={load} />}
      <Card className="p-2"><Table headers={['名称', '版本', '类型', '描述', '操作']}>
        {items.map((p) => {
          const display = presetDisplay(p)
          return <tr key={p.id} className="border-b border-gray-50 hover:bg-canvas"><td className="px-4 py-3 font-medium">{display.name}</td><td className="px-4 py-3">{p.version_label}</td><td className="px-4 py-3">{p.builtin ? <Badge>内置</Badge> : <Badge tone="muted">自定义</Badge>}</td><td className="px-4 py-3 text-gray-500">{display.description}</td><td className="px-4 py-3 space-x-2"><Button variant="ghost" onClick={() => { setEditing(p); setCreating(false) }}>编辑</Button>{!p.builtin && <Button variant="danger" onClick={() => remove(p.id)}>删除</Button>}</td></tr>
        })}
      </Table></Card>
    </div>
  )
}
