// 统一 HTTP 客户端：注入凭证、集中处理业务码。页面不裸调 axios（见开发规范 §4.3）。
import router from '@/router'
import { useUserStore } from '@/stores/user'
import type { ApiResponse } from '@/types/api'
import { ErrCode } from '@/types/api'
import axios, { type AxiosInstance } from 'axios'

const http: AxiosInstance = axios.create({
    // 统一走版本化前缀（P2-27）：新增破坏性接口时用 /api/v2，旧版继续跑 /api/v1。
    baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
    timeout: 15000,
})

// 请求拦截：自动携带 token（沿用旧约定，终态可切 Authorization 头）。
http.interceptors.request.use((config) => {
    const user = useUserStore()
    if (user.token) {
        config.headers.set('Authorization', `Bearer ${user.token}`)
    }
    return config
})

// 响应拦截：集中处理 34（未登录/过期）跳登录；其余原样返回，由 get/post 解包。
http.interceptors.response.use(
    (response) => {
        const body = response.data as ApiResponse
        if (body && body.code === ErrCode.Unauthorized) {
            useUserStore().clear()
            void router.push('/login')
        }
        return response
    },
    (error) => Promise.reject(error),
)

/** 发起 GET 并返回已解包的统一响应体。 */
export async function get<T>(url: string, params?: Record<string, unknown>): Promise<ApiResponse<T>> {
    const res = await http.get<ApiResponse<T>>(url, { params })
    return res.data
}

/** 发起 POST 并返回已解包的统一响应体。 */
export async function post<T>(url: string, data?: unknown): Promise<ApiResponse<T>> {
    const res = await http.post<ApiResponse<T>>(url, data)
    return res.data
}

/** 发起 PUT 并返回已解包的统一响应体。 */
export async function put<T>(url: string, data?: unknown): Promise<ApiResponse<T>> {
    const res = await http.put<ApiResponse<T>>(url, data)
    return res.data
}

/** 发起 DELETE（可带查询串）并返回已解包的统一响应体。 */
export async function del<T>(url: string, params?: Record<string, unknown>): Promise<ApiResponse<T>> {
    const res = await http.delete<ApiResponse<T>>(url, { params })
    return res.data
}

export default http
