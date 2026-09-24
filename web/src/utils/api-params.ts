// 请求头 <-> 参数行的归一化：参数表组件只认 ApiParam，请求头借 fieldName/example 承载头名与值。
import type { ApiHeader, ApiParam } from '@/types/entities'

export function headersToParams(headers: ApiHeader[]): ApiParam[] {
  return headers.map((h) => ({
    fieldName: h.name,
    type: '',
    required: false,
    description: h.description ?? '',
    example: h.value,
  }))
}

export function paramsToHeaders(rows: ApiParam[]): ApiHeader[] {
  return rows.map((r) => ({ name: r.fieldName, value: r.example ?? '', description: r.description }))
}
