/** Copy text in both secure (HTTPS/localhost) and plain HTTP contexts. */
export async function copyText(text: string): Promise<void> {
  const value = String(text ?? '')
  if (!value) {
    throw new Error('没有可复制的内容')
  }

  // Preferred API (requires secure context in most browsers)
  if (typeof navigator !== 'undefined' && window.isSecureContext && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(value)
      return
    } catch {
      // fall through to legacy path
    }
  }

  // Legacy fallback works on http://public-ip as long as it runs in a user gesture
  const ta = document.createElement('textarea')
  ta.value = value
  ta.setAttribute('readonly', '')
  ta.style.position = 'fixed'
  ta.style.top = '0'
  ta.style.left = '0'
  ta.style.width = '1px'
  ta.style.height = '1px'
  ta.style.padding = '0'
  ta.style.border = 'none'
  ta.style.outline = 'none'
  ta.style.boxShadow = 'none'
  ta.style.background = 'transparent'
  ta.style.opacity = '0'
  document.body.appendChild(ta)

  const selection = document.getSelection()
  const selected = selection && selection.rangeCount > 0 ? selection.getRangeAt(0) : null

  ta.focus()
  ta.select()
  ta.setSelectionRange(0, value.length)

  let ok = false
  try {
    ok = document.execCommand('copy')
  } catch {
    ok = false
  }

  document.body.removeChild(ta)
  if (selected && selection) {
    selection.removeAllRanges()
    selection.addRange(selected)
  }

  if (!ok) {
    throw new Error('复制失败，请手动选择文本复制')
  }
}
