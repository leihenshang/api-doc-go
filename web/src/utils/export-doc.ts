// 文档导出（T14）：把接口/项目导出为 Markdown 或**自包含单文件 HTML**，用于对外交付与归档。
// 说明：纯前端生成（数据来自既有接口），Markdown 正文用 markdown-it 渲染；不新增后端依赖。
import { headersToParams } from '@/utils/api-params'
import type { ApiHeader, ApiParam, DocItem, Group } from '@/types/entities'
import MarkdownIt from 'markdown-it'

/** 关闭原始 HTML 透传：导出物可能对外分享，避免把文档里的 <script> 带出去。 */
const md = new MarkdownIt({ html: false, linkify: true, breaks: false })

/** 导出所需的项目信息（完整 Project 与本页兜底对象都满足）。 */
export interface ProjectExportInput {
  title: string
  version?: string
  type?: string
  description?: string
}

/**
 * 导出所需的最小接口字段集：接口详情页可用「编辑态表单」直接构造，
 * 接口列表返回的 `ApiDoc` 也天然满足该结构。
 */
export interface ApiExportInput {
  id?: number
  api_name: string
  group_id: number
  protocol_type?: string
  http_method_type: string
  url: string
  description?: string
  http_request_header?: string
  http_request_params?: string
  http_return_type?: string
  http_return_params?: string
  http_return_sample?: string
  develop_language?: string
  update_time?: string
  status?: number
  version?: string
  deprecated_at?: string | null
}

// ---- 数据整理 ----

/** 请求头 JSON → 参数行。 */
export function apiHeaders(api: ApiExportInput): ApiParam[] {
  try {
    return headersToParams(JSON.parse(api.http_request_header || '[]') as ApiHeader[])
  } catch {
    return []
  }
}

/** 参数 JSON → 参数行。 */
export function apiParams(raw: string): ApiParam[] {
  try {
    return JSON.parse(raw || '[]') as ApiParam[]
  } catch {
    return []
  }
}

/** 分组路径（从顶层到当前分组），找不到时返回空数组。 */
export function groupPath(groups: Group[], groupId: number): string[] {
  const byId = new Map(groups.map((g) => [g.id, g]))
  const path: string[] = []
  let cur = byId.get(groupId)
  let guard = 0
  while (cur && guard++ < 20) {
    path.unshift(cur.title)
    cur = cur.p_id ? byId.get(cur.p_id) : undefined
  }
  return path
}

/** 旧值 → 单元格文本（空值显示为 —）。 */
function cell(v: unknown): string {
  const s = v === undefined || v === null ? '' : String(v)
  return s.trim() === '' ? '—' : s.replace(/\|/g, '\\|').replace(/\n/g, ' ')
}

/** 渲染参数表（Markdown）。 */
function paramTableMd(rows: ApiParam[]): string {
  if (!rows.length) return '_（无）_\n'
  const head = '| 参数名 | 必填 | 类型 | 示例 | 说明 |\n| :-- | :-- | :-- | :-- | :-- |\n'
  const body = rows
    .map((r) => `| ${cell(r.fieldName)} | ${r.required ? '是' : '否'} | ${cell(r.type)} | ${cell(r.example)} | ${cell(r.description)} |`)
    .join('\n')
  return head + body + '\n'
}

/** 渲染参数表（HTML）。 */
function paramTableHtml(rows: ApiParam[]): string {
  if (!rows.length) return '<p class="muted">（无）</p>'
  const head = '<table><thead><tr><th>参数名</th><th>必填</th><th>类型</th><th>示例</th><th>说明</th></tr></thead><tbody>'
  const body = rows
    .map(
      (r) =>
        `<tr><td>${esc(r.fieldName)}</td><td>${r.required ? '是' : '否'}</td><td>${esc(r.type)}</td>` +
        `<td>${esc(r.example)}</td><td>${esc(r.description)}</td></tr>`,
    )
    .join('')
  return head + body + '</tbody></table>'
}

/** HTML 转义。 */
function esc(v: unknown): string {
  const s = v === undefined || v === null ? '' : String(v)
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

/** 代码块（HTML）。 */
function codeBlock(code: string): string {
  if (!code.trim()) return '<p class="muted">（未配置）</p>'
  return `<pre class="code">${esc(code)}</pre>`
}

// ---- 单接口 ----

/** 接口元信息行（Markdown）。 */
function apiMetaMd(api: ApiExportInput, groups: Group[]): string {
  const path = groupPath(groups, api.group_id)
  const rows: string[] = [
    `- **接口名称**：${api.api_name || '—'}`,
    `- **所属分组**：${path.length ? path.join(' / ') : '未分组'}`,
    `- **请求方式**：${api.http_method_type || '—'}`,
    `- **请求协议**：${api.protocol_type || 'HTTP'}`,
    `- **返回类型**：${api.http_return_type || '—'}`,
  ]
  if (api.version) rows.push(`- **版本**：${api.version}`)
  if (api.status === 1) rows.push('- **状态**：草稿')
  if (api.status === 3) rows.push('- **状态**：已废弃')
  if (api.deprecated_at) rows.push(`- **废弃时间**：${api.deprecated_at}`)
  if (api.develop_language) rows.push(`- **开发语言**：${api.develop_language}`)
  if (api.update_time) rows.push(`- **最后更新**：${api.update_time}`)
  return rows.join('\n') + '\n'
}

/** 单个接口 → Markdown（level 为标题层级，接口详情用 2、项目导出用 3）。 */
export function apiToMarkdown(api: ApiExportInput, groups: Group[] = [], level = 2): string {
  const h = '#'.repeat(Math.min(6, Math.max(1, level)))
  const parts: string[] = [
    `${h} ${api.http_method_type || 'GET'} ${api.api_name || '未命名接口'}\n`,
    apiMetaMd(api, groups),
    `\n${h}# 请求地址\n\n\`\`\`\n${api.url || '—'}\n\`\`\`\n`,
    `\n${h}# 请求头部\n\n${paramTableMd(apiHeaders(api))}`,
    `\n${h}# 请求参数\n\n${paramTableMd(apiParams(api.http_request_params ?? ''))}`,
    `\n${h}# 返回参数\n\n${paramTableMd(apiParams(api.http_return_params ?? ''))}`,
  ]
  if (api.http_return_sample) {
    parts.push(`\n${h}# 响应样例\n\n\`\`\`json\n${api.http_return_sample}\n\`\`\`\n`)
  }
  if (api.description) parts.push(`\n${h}# 说明\n\n${api.description}\n`)
  return parts.join('')
}

/** 单个接口 → HTML 片段（不含外层文档壳）。 */
export function apiToHtml(api: ApiExportInput, groups: Group[] = [], level = 2, anchor = ''): string {
  const h = Math.min(6, Math.max(1, level))
  const path = groupPath(groups, api.group_id)
  const sample = api.http_return_sample ? codeBlock(api.http_return_sample) : '<p class="muted">（未配置）</p>'
  return `
<section class="api"${anchor ? ` id="${esc(anchor)}"` : ''}>
  <h${h} class="api-title"><span class="method m-${esc((api.http_method_type || '').toLowerCase())}">${esc(api.http_method_type)}</span> ${esc(api.api_name)}</h${h}>
  <ul class="meta">
    <li>所属分组：${esc(path.length ? path.join(' / ') : '未分组')}</li>
    <li>请求协议：${esc(api.protocol_type || 'HTTP')} · 返回类型：${esc(api.http_return_type || '—')}</li>
    ${api.version ? `<li>版本：${esc(api.version)}</li>` : ''}
    ${api.status === 1 ? '<li>状态：草稿</li>' : ''}
    ${api.status === 3 ? '<li>状态：已废弃</li>' : ''}
    ${api.deprecated_at ? `<li>废弃时间：${esc(api.deprecated_at)}</li>` : ''}
    ${api.develop_language ? `<li>开发语言：${esc(api.develop_language)}</li>` : ''}
    ${api.update_time ? `<li>最后更新：${esc(api.update_time)}</li>` : ''}
  </ul>
  <h${Math.min(6, h + 1)}>请求地址</h${Math.min(6, h + 1)}>
  ${codeBlock(api.url)}
  <h${Math.min(6, h + 1)}>请求头部</h${Math.min(6, h + 1)}>
  ${paramTableHtml(apiHeaders(api))}
  <h${Math.min(6, h + 1)}>请求参数</h${Math.min(6, h + 1)}>
  ${paramTableHtml(apiParams(api.http_request_params ?? ''))}
  <h${Math.min(6, h + 1)}>返回参数</h${Math.min(6, h + 1)}>
  ${paramTableHtml(apiParams(api.http_return_params ?? ''))}
  <h${Math.min(6, h + 1)}>响应样例</h${Math.min(6, h + 1)}>
  ${sample}
  ${
    api.description
      ? `<h${Math.min(6, h + 1)}>说明</h${Math.min(6, h + 1)}><div class="md">${md.render(api.description)}</div>`
      : ''
  }
</section>`
}

// ---- 整项目 ----

/** 稳定锚点：分组名/接口名 → id（保留中英文与数字）。 */
function slug(text: string, fallback: string): string {
  const s = text
    .trim()
    .toLowerCase()
    .replace(/[\s/\\]+/g, '-')
    .replace(/[^\p{L}\p{N}-]/gu, '')
    .replace(/-{2,}/g, '-')
    .replace(/^-|-$/g, '')
  return s || fallback
}

/** 项目 → Markdown（含目录 + 按分组排列的接口 + 项目文档）。 */
export function projectToMarkdown(project: ProjectExportInput, groups: Group[], apis: ApiExportInput[], docs: DocItem[] = []): string {
  const now = new Date().toLocaleString('zh-CN')
  const parts: string[] = [`# ${project.title}\n`]
  const meta: string[] = []
  if (project.version) meta.push(`- **版本**：${project.version}`)
  if (project.type) meta.push(`- **类型**：${project.type}`)
  meta.push(`- **导出时间**：${now}`, `- **接口数量**：${apis.length}`, `- **文档数量**：${docs.length}`)
  parts.push(meta.join('\n') + '\n')
  if (project.description) parts.push(`> ${project.description}\n`)

  // 目录（两级：分组 → 接口/文档标题）
  const apiGroups = groups.filter((g) => g.type === 1)
  const docGroups = groups.filter((g) => g.type === 3)
  const walk = (list: Group[], pid: number, depth: number, visit: (g: Group, depth: number) => void): void => {
    for (const g of list.filter((x) => x.p_id === pid)) {
      visit(g, depth)
      walk(list, g.id, depth + 1, visit)
    }
  }
  const toc: string[] = ['## 目录\n']
  walk(apiGroups, 0, 0, (g, depth) => {
    const pad = '  '.repeat(depth)
    const items = apis.filter((a) => a.group_id === g.id)
    if (!items.length && !apiGroups.some((c) => c.p_id === g.id)) return
    toc.push(`${pad}- **${g.title}**`)
    for (const a of items) toc.push(`${pad}  - ${a.http_method_type} ${a.api_name}`)
  })
  walk(docGroups, 0, 0, (g, depth) => {
    const pad = '  '.repeat(depth)
    const items = docs.filter((d) => d.group_id === g.id)
    if (!items.length && !docGroups.some((c) => c.p_id === g.id)) return
    toc.push(`${pad}- **${g.title}**（文档）`)
    for (const d of items) toc.push(`${pad}  - ${d.title}`)
  })
  parts.push(toc.join('\n') + '\n')

  // 接口正文（按分组）
  parts.push('## 接口\n')
  const loose = apis.filter((a) => !apiGroups.some((g) => g.id === a.group_id))
  const writeApis = (g: Group): void => {
    const items = apis.filter((a) => a.group_id === g.id)
    if (items.length) {
      parts.push(`### ${g.title}\n`)
      for (const a of items) parts.push(apiToMarkdown(a, groups, 4))
    }
    for (const k of groups.filter((c) => c.p_id === g.id)) writeApis(k)
  }
  for (const g of apiGroups.filter((g) => !g.p_id)) writeApis(g)
  if (loose.length) {
    parts.push('### 未分组\n')
    for (const a of loose) parts.push(apiToMarkdown(a, groups, 4))
  }

  if (docs.length) {
    parts.push('## 文档\n')
    const written = new Set<number>()
    const docBody = (d: DocItem): string => `${d.content || '（空）'}\n`
    walk(docGroups, 0, 0, (g) => {
      const items = docs.filter((d) => d.group_id === g.id)
      if (!items.length) return
      parts.push(`### ${g.title}\n`)
      for (const d of items) {
        written.add(d.id)
        parts.push(`#### ${d.icon ? d.icon + ' ' : ''}${d.title}\n`, docBody(d))
      }
    })
    const rest = docs.filter((d) => !written.has(d.id))
    if (rest.length) {
      parts.push('### 未分组文档\n')
      for (const d of rest) parts.push(`#### ${d.icon ? d.icon + ' ' : ''}${d.title}\n`, docBody(d))
    }
  }
  parts.push(`\n---\n\n_由 API-DOC-GO 于 ${now} 导出_\n`)
  return parts.join('\n')
}

/** HTML 文档壳（自包含单文件：内嵌样式 + 侧边目录，窄屏自动堆叠）。 */
export function wrapHtml(title: string, body: string, toc: { id: string; text: string; level: number }[]): string {
  const tocHtml = toc
    .map((t) => `<a class="lvl-${t.level}" href="#${esc(t.id)}">${esc(t.text)}</a>`)
    .join('')
  return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>${esc(title)}</title>
<style>
  :root { --bd:#e6e8eb; --muted:#8a9199; --accent:#18a058; --bg:#fbfbfc; }
  * { box-sizing: border-box; }
  body { margin:0; color:#1f2329; font:14px/1.7 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif; }
  header { padding:16px 24px; border-bottom:1px solid var(--bd); position:sticky; top:0; background:#fff; z-index:2; }
  header h1 { margin:0 0 4px; font-size:18px; }
  header .sub { color:var(--muted); font-size:12px; }
  .layout { display:flex; align-items:flex-start; }
  nav { flex:0 0 240px; max-height:calc(100vh - 80px); overflow:auto; padding:16px 12px; border-right:1px solid var(--bd); background:var(--bg); position:sticky; top:64px; }
  nav a { display:block; padding:3px 8px; color:#3f4750; text-decoration:none; border-radius:4px; font-size:13px; }
  nav a:hover { background:#eef2f0; color:var(--accent); }
  nav .lvl-1 { font-weight:600; margin-top:6px; }
  nav .lvl-2 { padding-left:20px; color:var(--muted); }
  main { flex:1 1 auto; min-width:0; padding:20px 28px 60px; }
  h2,h3,h4,h5 { line-height:1.4; }
  h2 { margin:28px 0 12px; padding-bottom:6px; border-bottom:1px solid var(--bd); font-size:17px; }
  h3 { margin:22px 0 10px; font-size:15px; }
  h4 { margin:18px 0 8px; font-size:14px; }
  .method { display:inline-block; margin-right:6px; padding:1px 6px; border-radius:3px; color:#fff; font-size:11px; font-weight:700; vertical-align:2px; }
  .m-get{background:#18a058}.m-post{background:#d97706}.m-put{background:#2563eb}.m-patch{background:#7c3aed}.m-delete{background:#dc2626}
  ul.meta { margin:0 0 8px; padding-left:20px; color:var(--muted); font-size:12px; }
  table { width:100%; border-collapse:collapse; margin:8px 0 16px; font-size:13px; }
  th,td { border:1px solid var(--bd); padding:6px 8px; text-align:left; vertical-align:top; }
  th { background:var(--bg); font-weight:600; }
  pre.code { margin:8px 0 16px; padding:10px 12px; overflow:auto; border:1px solid var(--bd); border-radius:4px; background:var(--bg); font:12.5px/1.6 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace; }
  .md img { max-width:100%; }
  .muted { color:var(--muted); }
  section.api { margin-bottom:28px; }
  @media (max-width: 768px) {
    .layout { flex-direction:column; }
    nav { position:static; flex:1 1 auto; width:100%; max-height:40vh; border-right:0; border-bottom:1px solid var(--bd); }
    main { padding:16px; }
  }
</style>
</head>
<body>
<header>
  <h1>${esc(title)}</h1>
  <div class="sub">由 API-DOC-GO 导出 · ${esc(new Date().toLocaleString('zh-CN'))}</div>
</header>
<div class="layout">
  <nav>${tocHtml}</nav>
  <main>${body}</main>
</div>
</body>
</html>`
}

/** 项目 → HTML（自包含单文件）。 */
export function projectToHtml(project: ProjectExportInput, groups: Group[], apis: ApiExportInput[], docs: DocItem[] = []): string {
  const apiGroups = groups.filter((g) => g.type === 1)
  const toc: { id: string; text: string; level: number }[] = []
  const body: string[] = []
  body.push(
    `<p class="muted">版本 ${esc(project.version || '—')} · 接口 ${apis.length} 个 · 文档 ${docs.length} 篇</p>` +
      (project.description ? `<div class="md">${md.render(project.description)}</div>` : ''),
  )

  // 分组按层级递归渲染：有子分组但自身无接口的父分组也会输出（保持目录结构完整）。
  const write = (g: Group, level: number): void => {
    const items = apis.filter((a) => a.group_id === g.id)
    const kids = groups.filter((c) => c.p_id === g.id)
    if (!items.length && !kids.length) return
    const id = slug(g.title, `g-${g.id}`)
    const heading = Math.min(6, level + 1)
    toc.push({ id, text: g.title, level })
    body.push(`<section id="${esc(id)}"><h${heading}>${esc(g.title)}</h${heading}>`)
    for (const a of items) {
      const aid = slug(`${a.http_method_type}-${a.api_name}`, `api-${a.id}`)
      toc.push({ id: aid, text: `${a.http_method_type} ${a.api_name}`, level: level + 1 })
      body.push(apiToHtml(a, groups, heading + 1, aid))
    }
    for (const k of kids) write(k, level + 1)
    body.push('</section>')
  }
  for (const g of apiGroups.filter((g) => !g.p_id)) write(g, 1)

  const loose = apis.filter((a) => !apiGroups.some((g) => g.id === a.group_id))
  if (loose.length) {
    const id = 'g-ungrouped'
    toc.push({ id, text: '未分组', level: 1 })
    body.push(`<section id="${id}"><h2>未分组</h2>`)
    for (const a of loose) {
      const aid = slug(`${a.http_method_type}-${a.api_name}`, `api-${a.id}`)
      toc.push({ id: aid, text: `${a.http_method_type} ${a.api_name}`, level: 2 })
      body.push(apiToHtml(a, groups, 3, aid))
    }
    body.push('</section>')
  }

  if (docs.length) {
    toc.push({ id: 'docs', text: '文档', level: 1 })
    body.push('<section id="docs"><h2>文档</h2>')
    for (const d of docs) {
      const id = slug(d.title, `doc-${d.id}`)
      toc.push({ id, text: d.title, level: 2 })
      body.push(`<section id="${esc(id)}"><h3>${esc(d.icon ? d.icon + ' ' : '')}${esc(d.title)}</h3><div class="md">${md.render(d.content || '')}</div></section>`)
    }
    body.push('</section>')
  }
  return wrapHtml(project.title, body.join('\n'), toc)
}

/** 单个接口 → HTML（自包含单文件）。 */
export function apiToStandaloneHtml(api: ApiExportInput, groups: Group[] = []): string {
  const anchor = slug(`${api.http_method_type}-${api.api_name}`, `api-${api.id}`)
  return wrapHtml(
    `${api.http_method_type} ${api.api_name}`,
    apiToHtml(api, groups, 2, anchor),
    [{ id: anchor, text: `${api.http_method_type} ${api.api_name}`, level: 1 }],
  )
}

/** 触发浏览器下载。 */
export function downloadText(filename: string, content: string, mime: string): void {
  const url = URL.createObjectURL(new Blob([content], { type: `${mime};charset=utf-8` }))
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

/** 文件名安全化（去掉路径分隔符等）。 */
export function safeName(text: string, fallback: string): string {
  const s = (text || '').trim().replace(/[\\/:*?"<>|\s]+/g, '-')
  return s || fallback
}
