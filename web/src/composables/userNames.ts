// 用户 id → 显示名的全局缓存：多处列表需展示作者/操作人，避免每页重复拉取。
import { listUsers } from '@/api/auth'
import { ErrCode } from '@/types/api'
import { ref } from 'vue'

const names = ref<Record<number, string>>({})
let loading: Promise<void> | null = null

export function useUserNames() {
    /** 首次调用时拉取用户表，后续复用同一 Promise。 */
    function ensure(): Promise<void> {
        if (!loading) {
            loading = listUsers({ page_size: 200 }).then((res) => {
                if (res.code === ErrCode.Success && res.data) {
                    const map: Record<number, string> = {}
                    for (const u of res.data.items) map[u.id] = u.nick_name || u.name
                    names.value = map
                }
            })
        }
        return loading
    }

    function nameOf(id: number): string {
        return names.value[id] ?? `用户 #${id}`
    }

    return { ensure, nameOf }
}
