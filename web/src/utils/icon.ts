/** 文档图标工具：区分「图片 URL」与「emoji / 文本」，前者用 <img> 渲染。 */

/** 是否图片型图标（http(s) 绝对地址或上传目录相对地址）。 */
export function isImageIcon(icon?: string | null): boolean {
  const v = (icon ?? '').trim()
  if (!v) return false
  return v.startsWith('http://') || v.startsWith('https://') || v.startsWith('/uploads')
}

/** 文档默认图标（未设置时展示）。 */
export const DEFAULT_DOC_ICON = '📄'
