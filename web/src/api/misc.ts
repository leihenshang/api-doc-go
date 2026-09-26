// Bug / 属性 / 消息 / 通知 / 日志 / 外部库 / 文件 接口。
import type { ApiResponse } from '@/types/api';
import type { Bug, BugAssign, Notification, OpLog, Page } from '@/types/entities';
import http, { del, get, post, put } from './http';

// ---- Bug ----
export function createBug(payload: { project_id: number; title: string; content: string; level?: number; comment?: string }): Promise<ApiResponse<Bug>> {
    return post<Bug>('/bug', payload)
}
export function listBugs(params: { project_id: number; status?: number; page_num?: number; page_size?: number }): Promise<ApiResponse<Page<Bug>>> {
    return get<Page<Bug>>('/bug', params)
}
export function getBug(id: number): Promise<ApiResponse<Bug>> {
    return get<Bug>('/bug/detail', { id })
}
export function updateBug(payload: { id: number; title?: string; content?: string; status?: number; level?: number; comment?: string; is_top?: number }): Promise<ApiResponse<Bug>> {
    return put<Bug>('/bug', payload)
}
export function deleteBug(id: number): Promise<ApiResponse<null>> {
    return del<null>('/bug', { id })
}
export function assignBug(payload: { bug_id: number; to_user_id: number; comment?: string }): Promise<ApiResponse<null>> {
    return post<null>('/bug/assign', payload)
}

// ---- 公共属性 ----
export function listProperties(): Promise<ApiResponse<Array<Record<string, unknown>>>> {
    return get<Array<Record<string, unknown>>>('/property')
}
export function saveProperty(payload: Record<string, unknown>): Promise<ApiResponse<unknown>> {
    return post<unknown>('/property', payload)
}
export function deleteProperty(id: number): Promise<ApiResponse<null>> {
    return del<null>('/property', { id })
}

// ---- 文件上传 ----
export function uploadImg(file: File): Promise<ApiResponse<{ id: number; url: string }>> {
    const fd = new FormData()
    fd.append('file', file)
    return http.post<ApiResponse<{ id: number; url: string }>>('/file/upload', fd).then((r) => r.data)
}

// ---- 消息 / 通知 ----
export function sendMessage(payload: { receive_source: string; send_type?: number }): Promise<ApiResponse<null>> {
    return post<null>('/message/send', payload)
}
export function listNotifications(params: { page_num?: number; page_size?: number }): Promise<ApiResponse<Page<Notification>>> {
    return get<Page<Notification>>('/notification', params)
}
export function markNotificationRead(id: number): Promise<ApiResponse<null>> {
    return post<null>('/notification/read', { id })
}

// ---- 操作日志 ----
export function listLogs(params: { project_id: number; object_id?: number; type?: number; page_num?: number; page_size?: number }): Promise<ApiResponse<Page<OpLog>>> {
    return get<Page<OpLog>>('/operation-log', params)
}

// ---- Bug 指派记录 ----
export function listBugAssign(bugId: number): Promise<ApiResponse<BugAssign[]>> {
    return get<BugAssign[]>('/bug/assign', { bug_id: bugId })
}
