// 变量占位符切片：把文本按 {{name}} 拆成「纯文本 / 变量」段，供 VarText 渲染。
// 正则与后端 service/variable.go 的 varPlaceholderPattern 保持一致（变量名规则同 §2.1）。
import { translate } from '@/i18n'
import type { ResolveItem } from '@/types/entities'

/** 变量段状态：有值 / 空值 / 敏感值（脱敏）/ 未定义。 */
export type VarState = 'value' | 'empty' | 'secret' | 'undefined'

export interface VarSegment {
  /** 段文本：变量段为占位符原文（如 `{{baseUrl}}`），便于保持文档原貌。 */
  text: string
  /** 变量名（纯文本段为空）。 */
  name?: string
  /** 解析出的取值（敏感值为掩码，未定义时不填）。 */
  value?: string
  /** 生效来源：request / project / global。 */
  source?: string
  state?: VarState
}

const VAR_RE = /\{\{\s*([A-Za-z_][A-Za-z0-9_]{0,63})\s*\}\}/g

/** 把文本切成渲染段；未提供解析表时全部按纯文本返回。 */
export function splitVarSegments(text: string, map: Record<string, ResolveItem> = {}): VarSegment[] {
  const out: VarSegment[] = []
  if (!text) return out
  VAR_RE.lastIndex = 0
  let last = 0
  for (let m = VAR_RE.exec(text); m; m = VAR_RE.exec(text)) {
    if (m.index > last) out.push({ text: text.slice(last, m.index) })
    const name = m[1]
    const hit = map[name]
    out.push({
      text: m[0],
      name,
      value: hit?.value ?? '',
      source: hit?.source,
      state: !hit ? 'undefined' : hit.is_secret === 1 ? 'secret' : hit.has_value ? 'value' : 'empty',
    })
    last = m.index + m[0].length
  }
  if (last < text.length) out.push({ text: text.slice(last) })
  return out
}

/** 变量来源的显示名（用于 tooltip，随界面语言切换）。 */
export function sourceLabel(source?: string): string {
  if (source === 'request') return translate('varSource.request')
  if (source === 'project') return translate('varSource.project')
  if (source === 'global') return translate('varSource.global')
  return translate('varSource.undefined')
}
