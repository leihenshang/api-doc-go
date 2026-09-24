// 响应/请求 JSON → 结构化参数表：扁平化解析与既有行合并（保留人工填写的说明）。
import type { ApiParam } from '@/types/entities'

/** JSON 值的类型名；整数与浮点区分，便于文档表达。 */
function jsonType(v: unknown): string {
  if (v === null || v === undefined) return 'null'
  if (Array.isArray(v)) return 'array'
  switch (typeof v) {
    case 'string':
      return 'string'
    case 'boolean':
      return 'boolean'
    case 'number':
      return Number.isInteger(v) ? 'integer' : 'number'
    default:
      return 'object'
  }
}

/** 标量与基本类型数组取示例值；对象/对象数组不填示例。 */
function exampleOf(v: unknown): string {
  if (v === null || v === undefined) return ''
  if (Array.isArray(v)) {
    const first = v[0]
    return first !== undefined && first !== null && typeof first !== 'object' ? String(first) : ''
  }
  return typeof v === 'object' ? '' : String(v)
}

/**
 * 递归展开为 `data.list[].id` 形式的路径行。
 * 对象逐层展开；数组按首个元素推断结构并以 `[]` 标记。
 */
export function flattenJson(value: unknown, prefix = '', out: ApiParam[] = []): ApiParam[] {
  if (Array.isArray(value)) {
    const path = `${prefix}[]`
    const first = value[0]
    if (first !== undefined && first !== null && typeof first === 'object') flattenJson(first, path, out)
    return out
  }
  if (value !== null && typeof value === 'object') {
    for (const [key, child] of Object.entries(value as Record<string, unknown>)) {
      const path = prefix ? `${prefix}.${key}` : key
      out.push({ fieldName: path, type: jsonType(child), required: false, description: '', example: exampleOf(child) })
      if (child !== null && typeof child === 'object') flattenJson(child, path, out)
    }
  }
  return out
}

/** 解析 JSON 文本为参数行；空文本或非法 JSON 返回 error。 */
export function jsonToParams(raw: string): { rows: ApiParam[]; error?: string } {
  const text = raw.trim()
  if (!text) return { rows: [], error: '请输入 JSON 内容' }
  try {
    return { rows: flattenJson(JSON.parse(text) as unknown) }
  } catch (e) {
    return { rows: [], error: 'JSON 解析失败：' + (e as Error).message }
  }
}

/**
 * 合并生成行与既有行：同名字段沿用已填写的说明与必填标记，
 * 新字段按 JSON 顺序追加，JSON 中不存在的人工补充行予以保留。
 * replace 为真时直接以生成结果覆盖。
 */
export function mergeParams(existing: ApiParam[], generated: ApiParam[], replace = false): ApiParam[] {
  if (replace) return generated.map((g) => ({ ...g }))
  const byName = new Map(existing.map((p) => [p.fieldName, p]))
  const matched = new Set<string>()
  const rows = generated.map((g) => {
    const old = byName.get(g.fieldName)
    if (!old) return { ...g }
    matched.add(g.fieldName)
    return { ...g, description: old.description || g.description, required: old.required ?? g.required }
  })
  return [...rows, ...existing.filter((p) => !matched.has(p.fieldName)).map((p) => ({ ...p }))]
}
