// 展示格式化工具。
/** ISO 时间转 `YYYY-MM-DD HH:mm:ss`；空值返回破折号。 */
export function fmtTime(iso?: string): string {
    if (!iso) return '—'
    return iso.slice(0, 19).replace('T', ' ')
}

/** ISO 时间转 `YYYY-MM-DD`。 */
export function fmtDate(iso?: string): string {
    if (!iso) return '—'
    return iso.slice(0, 10)
}
