/**
 * 文档正文渲染前的清洗。
 *
 * 服务端的文档页头部已经渲染了标题（还有作者/分组/时间），正文若再写一个同名 H1，
 * 视觉上就是同一个标题连着出现两次。这里只在**预览**里剥掉开头那个同名 H1，
 * 原文（编辑态、数据库）保持不动 —— 用户可能就是想用 H1 分段，不能替他改内容。
 */
export function stripDuplicateTitle(md: string, title: string): string {
  const want = normalize(title)
  if (!want) return md

  const lines = md.split('\n')
  let i = 0
  while (i < lines.length && lines[i].trim() === '') i++
  if (i >= lines.length) return md

  const m = /^#\s+(.*)$/.exec(lines[i].trim())
  if (!m || normalize(m[1]) !== want) return md

  let j = i + 1
  while (j < lines.length && lines[j].trim() === '') j++
  // 连同 H1 前面的空行一起丢掉：否则正文顶部会多出一截空白
  return lines.slice(j).join('\n')
}

/** 比较用的归一化：忽略强调/行内代码符号与多余空白、大小写差异。 */
function normalize(s: string): string {
  return s
    .replace(/[*_`~]/g, '')
    .replace(/\s+/g, '')
    .trim()
    .toLowerCase()
}
