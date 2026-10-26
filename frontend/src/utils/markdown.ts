// 轻量 Markdown 渲染：聊天消息用，输出安全 HTML（先转义再转换，模型输出无法注入 HTML）
const esc = (t: string) => t.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')

export function renderMarkdown(src: string): string {
  // 1) 先抽出代码块，换成不会被任何行内规则误吃的占位符，避免内容被其它规则影响
  const codes: string[] = []
  let s = src.replace(/```[^\n]*\n?([\s\S]*?)```/g, (_m, code: string) => {
    codes.push(code.replace(/\n+$/, ''))
    return `${codes.length - 1}`
  })

  s = esc(s)

  // 2) 标题 # ~ ######
  s = s.replace(/^(#{1,6})[ \t]+(.+)$/gm, (_m, h: string, t: string) => `<div class="md-h md-h${h.length}">${t}</div>`)

  // 3) 无序 / 有序列表（先于斜体，避免 * 列表符被当成斜体）
  s = s.replace(/(^|\n)((?:[ \t]*[-•*][ \t]+[^\n]*(?:\n|$))+)/g, (_m, pfx: string, block: string) => {
    const items = block
      .trimEnd()
      .split('\n')
      .map((l: string) => `<li>${l.replace(/^[ \t]*[-•*][ \t]+/, '')}</li>`)
      .join('')
    return `${pfx}<ul class="md-ul">${items}</ul>`
  })
  s = s.replace(/(^|\n)((?:[ \t]*\d+[.、][ \t]+[^\n]*(?:\n|$))+)/g, (_m, pfx: string, block: string) => {
    const items = block
      .trimEnd()
      .split('\n')
      .map((l: string) => `<li>${l.replace(/^[ \t]*\d+[.、][ \t]+/, '')}</li>`)
      .join('')
    return `${pfx}<ol class="md-ol">${items}</ol>`
  })

  // 4) 粗体、斜体、行内代码
  s = s.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  s = s.replace(/(^|[^*])\*([^*\n]+)\*/g, '$1<em>$2</em>')
  s = s.replace(/`([^`\n]+)`/g, '<code class="md-inline">$1</code>')

  // 5) 剩余换行转 <br>
  s = s.replace(/\n/g, '<br>')

  // 6) 还原代码块（转义后输出）
  s = s.replace(/(\d+)/g, (_m, i: string) => `<pre class="md-code"><code>${esc(codes[Number(i)] ?? '')}</code></pre>`)

  // 7) 块级元素前后的 <br> 清理
  s = s.replace(/<br>(?=<(?:div|ul|ol|pre)\b)/g, '')
  s = s.replace(/(<\/(?:div|ul|ol|pre)>)<br>/g, '$1')

  return s
}
