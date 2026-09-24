// 路由参数解析：vue-router 的 params 始终是字符串，统一在此转数字，避免各页面重复处理。
import { computed, type ComputedRef } from 'vue'
import { useRoute } from 'vue-router'

/** 取路由数字参数；缺失或非法返回 0。 */
export function useNumParam(name: string): ComputedRef<number> {
    const route = useRoute()
    return computed(() => Number(route.params[name]) || 0)
}
