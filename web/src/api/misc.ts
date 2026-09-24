// Bug / 属性 / 消息 / 通知 / 日志 / 外部库 / 文件 接口。
import type { ApiResponse } from '@/types/api';
import type { Bug, BugAssign, Conn, Notification, OpLog, Page } from '@/types/entities';
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
export function updateBug(payload: { id: number; title?: string; content?: string; status?: number; level?: number; comment?: string }): Promise<ApiResponse<Bug>> {
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

// ---- 外部数据库连接 ----
export function listConns(projectId: number): Promise<ApiResponse<Conn[]>> {
    return get<Conn[]>('/database', { project_id: projectId })
}
export function createConn(payload: Record<string, unknown>): Promise<ApiResponse<Conn>> {
    return post<Conn>('/database', payload)
}
export function updateConn(payload: Record<string, unknown>): Promise<ApiResponse<Conn>> {
    return put<Conn>('/database', payload)
}
export function deleteConn(id: number): Promise<ApiResponse<null>> {
    return del<null>('/database', { id })
}

// ---- Bug 指派记录 ----
export function listBugAssign(bugId: number): Promise<ApiResponse<BugAssign[]>> {
    return get<BugAssign[]>('/bug/assign', { bug_id: bugId })
}

// ---- 外部库元数据（introspect） ----
export interface TableInfo {
    name: string
    comment: string
}
export interface ColumnInfo {
    name: string
    data_type: string
    comment: string
}
export function connTables(connId: number): Promise<ApiResponse<TableInfo[]>> {
    return get<TableInfo[]>('/database/tables', { conn_id: connId })
}
export function connColumns(connId: number, table: string): Promise<ApiResponse<ColumnInfo[]>> {
    return get<ColumnInfo[]>('/database/columns', { conn_id: connId, table })
}
