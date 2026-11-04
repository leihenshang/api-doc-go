import { describe, expect, it } from 'vitest'
import { stripDuplicateTitle } from './docContent'

describe('stripDuplicateTitle', () => {
  it('剥掉开头与标题同名的 H1', () => {
    expect(stripDuplicateTitle('# 安全规范\n\n## 必读\n\n正文', '安全规范')).toBe('## 必读\n\n正文')
  })

  it('H1 前有空行也能认出来', () => {
    expect(stripDuplicateTitle('\n\n# 安全规范\n\n正文', '安全规范')).toBe('正文')
  })

  it('标题不同就不动（正文自带的分段标题要保留）', () => {
    const md = '# 用户中心\n\n正文'
    expect(stripDuplicateTitle(md, '用户中心说明')).toBe(md)
  })

  it('大小写 / 强调符号 / 多余空白的差异视为同名', () => {
    expect(stripDuplicateTitle('#  **安全  规范**\n\n正文', '安全规范')).toBe('正文')
  })

  it('没有 H1 / 空内容时原样返回', () => {
    expect(stripDuplicateTitle('正文', '标题')).toBe('正文')
    expect(stripDuplicateTitle('', '标题')).toBe('')
  })

  it('标题为空时不做任何处理（避免误删）', () => {
    expect(stripDuplicateTitle('# 任意\n\n正文', '')).toBe('# 任意\n\n正文')
  })

  it('只剥开头那个，后面的同名 H1 保留', () => {
    expect(stripDuplicateTitle('# A\n\n正文\n\n# A\n\n尾部', 'A')).toBe('正文\n\n# A\n\n尾部')
  })
})
