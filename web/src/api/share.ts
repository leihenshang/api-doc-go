// 分享链接（免登录只读）接口。
import type { ApiResponse } from '@/types/api'
import type { ShareLink, ShareView } from '@/types/entities'
import { del, get, post } from './http'

export function createShareLink(payload: {
  project_id: number
  remark?: string
  /** 有效期天数，0/缺省 = 永久有效。 */
  expire_days?: number
}): Promise<ApiResponse<ShareLink>> {
  return post<ShareLink>('/share', payload)
}
/**
 * 分享链接列表。默认令牌脱敏（防截屏/旁观泄露）；
 * reveal=true 时回原文 —— 仅供复制链接使用，否则复制出来的是星号。
 */
export function listShareLinks(projectId: number, reveal = false): Promise<ApiResponse<ShareLink[]>> {
  return get<ShareLink[]>('/share', reveal ? { project_id: projectId, reveal: 1 } : { project_id: projectId })
}
export function deleteShareLink(id: number): Promise<ApiResponse<null>> {
  return del<null>('/share', { id })
}
/** 免登录读取分享数据（分享页使用；令牌无效/过期返回 code 14）。 */
export function getSharePayload(token: string): Promise<ApiResponse<ShareView>> {
  return get<ShareView>(`/share/${token}`)
}
