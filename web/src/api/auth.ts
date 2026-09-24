// 用户/鉴权接口。
import type { ApiResponse } from '@/types/api'
import type { LoginResult, Page, User } from '@/types/entities'
import { get, post, put } from './http'

export interface LoginPayload {
    name: string
    pwd: string
}

export function login(payload: LoginPayload): Promise<ApiResponse<LoginResult>> {
    return post<LoginResult>('/user/login', payload)
}

export function register(payload: {
    name: string
    pwd: string
    re_pwd: string
    email?: string
    nick_name?: string
}): Promise<ApiResponse<User>> {
    return post<User>('/user/register', payload)
}

/** 注册前查重：keyword 是否已被用户名/邮箱/昵称占用。 */
export function checkRepeat(keyword: string): Promise<ApiResponse<{ repeated: boolean }>> {
    return get<{ repeated: boolean }>('/user/check-repeat', { keyword })
}

export function me(): Promise<ApiResponse<User>> {
    return get<User>('/user/me')
}

export function listUsers(params: { keyword?: string; page_num?: number; page_size?: number }): Promise<ApiResponse<Page<User>>> {
    return get<Page<User>>('/user/list', params)
}

export function updateProfile(id: number, payload: Record<string, unknown>): Promise<ApiResponse<User>> {
    return put<User>(`/user/profile?id=${id}`, payload)
}

export function updatePwd(payload: { old_pwd: string; pwd: string; re_pwd: string }): Promise<ApiResponse<null>> {
    return put<null>('/user/password', payload)
}
