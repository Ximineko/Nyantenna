/**
 * 复制文本。Clipboard API 在非安全上下文/被策略拦截时会失败，
 * 逐级回落到 execCommand，最后弹 prompt 让用户手动复制。
 * 返回是否真的复制成功——提示交给调用方，这里不依赖任何 UI 框架。
 */
export async function copyToClipboard(text: unknown): Promise<boolean> {
  const val = String(text ?? '').trim()
  if (!val) return false

  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(val)
      return true
    }
  } catch {
    // Clipboard API 被拦截，走下面的兜底
  }

  try {
    const ta = document.createElement('textarea')
    ta.value = val
    ta.setAttribute('readonly', 'true')
    ta.style.position = 'fixed'
    ta.style.left = '0'
    ta.style.top = '0'
    ta.style.opacity = '0'
    ta.style.pointerEvents = 'none'
    ta.style.width = '1px'
    ta.style.height = '1px'
    document.body.appendChild(ta)
    ta.focus()
    ta.select()
    ta.setSelectionRange(0, ta.value.length)
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    if (ok) return true
  } catch {
    // 继续回落
  }

  try {
    window.prompt('复制以下内容', val)
  } catch {
    // 忽略
  }
  return false
}
