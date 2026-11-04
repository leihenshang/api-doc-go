// 项目文档（独立 markdown 文档）的渲染契约。
//
// 为什么单独测：接口的「说明」字段也是 markdown，但它是**接口详情里的一段**，
// 与「项目文档」是两套数据（doc.content vs api.description）。曾经出现过
// 文档页把接口说明塞进来的疑虑，这里用同款渲染器（markdown-it，md-editor-v3 内部
// 用的就是它）把两条链路分开断言，避免回归时靠肉眼判断。
//
// 页面上标题由 DocDetailView 的 <h1> 单独渲染（正文里重复的 H1 会被
// stripDuplicateTitle 去掉），所以这里连「去重后正文是否仍可渲染」一起覆盖。

import { describe, expect, it } from 'vitest'
import MarkdownIt from 'markdown-it'
import { stripDuplicateTitle } from './docContent'

const md = new MarkdownIt({ html: true, linkify: true, breaks: true })

/** 一篇覆盖常见 markdown 语法的独立文档（不是接口说明）。 */
const DOC = `# 渲染验证文档

这是一段用于确认 **加粗**、*斜体*、\`行内代码\` 与 [外链](https://example.com) 的正文。

## 二级标题

- 列表项一
- 列表项二，含 \`code\` 片段

1. 有序第一
2. 有序第二

> 引用块

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| userId | string | 用户标识 |

\`\`\`go
func Hello(name string) string { return "hi" }
\`\`\`

---

结尾段落。
`

describe('项目文档的 markdown 渲染', () => {
    it('独立文档的各种结构都能渲染成对应标签', () => {
        const html = md.render(DOC)

        expect(html).toContain('<h1>渲染验证文档</h1>')
        expect(html).toContain('<h2>二级标题</h2>')
        expect(html).toContain('<ul>')
        expect(html).toContain('<ol>')
        expect(html).toContain('<li>')
        expect(html).toContain('<blockquote>')
        expect(html).toContain('<table>')
        expect(html).toContain('<th>字段</th>')
        expect(html).toContain('<td>userId</td>')
        expect(html).toContain('<pre><code')
        expect(html).toContain('<hr>')
        expect(html).toContain('<strong>加粗</strong>')
        expect(html).toContain('<em>斜体</em>')
        expect(html).toContain('<code>行内代码</code>')
        expect(html).toContain('href="https://example.com"')
    })

    it('标题去重后正文仍完整可渲染（页面标题已由 <h1> 承担）', () => {
        const body = stripDuplicateTitle(DOC, '渲染验证文档')
        expect(body.startsWith('# 渲染验证文档')).toBe(false)

        const html = md.render(body)
        expect(html).not.toContain('<h1>')
        // 去重只该删掉重复标题，其余结构一个不能少
        expect(html).toContain('<h2>二级标题</h2>')
        expect(html).toContain('<table>')
        expect(html).toContain('<pre><code')
    })

    it('接口说明与项目文档是两条数据，不该互相串入', () => {
        const apiDesc = '# 这是接口说明，不是文档\n\n接口的说明字段不应出现在项目文档页。'
        const docHtml = md.render(stripDuplicateTitle(DOC, '渲染验证文档'))

        expect(docHtml).not.toContain('这是接口说明')
        // 反向也要成立：接口说明的渲染结果里不该混进文档正文
        expect(md.render(apiDesc)).not.toContain('渲染验证文档')
    })
})
