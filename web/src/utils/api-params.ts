// 请求头 <-> 参数行的归一化：参数表组件只认 ApiParam，请求头借 fieldName/example 承载头名与值。
//
// enabled 是行级启用位（客户端 KV 的同步形态）：缺省 = 启用，只有显式 false 才停用。
// 序列化时只在停用时写键，保证老数据（没有该键）保存后字节不变。
import type { ApiHeader, ApiParam } from '@/types/entities'

/** 行是否启用（缺省视为启用）。表格勾选、导出过滤、调试发送共用这一处口径。 */
export function rowEnabled(row: { enabled?: boolean }): boolean {
  return row.enabled !== false
}

export function headersToParams(headers: ApiHeader[]): ApiParam[] {
  return headers.map((h) => {
    const row: ApiParam = {
      fieldName: h.name,
      type: '',
      required: false,
      description: h.description ?? '',
      example: h.value,
    }
    if (h.enabled === false) row.enabled = false
    return row
  })
}

export function paramsToHeaders(rows: ApiParam[]): ApiHeader[] {
  return rows.map((r) => {
    const header: ApiHeader = { name: r.fieldName, value: r.example ?? '', description: r.description }
    if (r.enabled === false) header.enabled = false
    return header
  })
}

/**
 * 保存/导出前清洗行：去掉空行（编辑态末尾的占位空行）、修剪字段名、补默认值。
 * 用展开保留未知键（服务端/客户端将来加字段时 Web 不会静默洗掉）；启用位仍只在 false 时写。
 */
export function cleanParams(rows: ApiParam[]): ApiParam[] {
  return rows
    .filter((r) => (r.fieldName ?? '').trim() !== '')
    .map((r) => {
      const row: ApiParam = {
        ...r,
        fieldName: r.fieldName.trim(),
        type: r.type ?? '',
        required: r.required === true,
        description: r.description ?? '',
        example: r.example ?? '',
      }
      if (r.enabled !== false) delete row.enabled
      return row
    })
}
