// 退出登录：清空用户与项目缓存并跳转登录页（顶栏用户菜单与个人中心共用）。
import { useProjectStore } from '@/stores/project'
import { useUserStore } from '@/stores/user'
import { useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

export function useLogout(): () => void {
  const router = useRouter()
  const message = useMessage()
  const user = useUserStore()
  const project = useProjectStore()
  const { t } = useI18n()

  return function logout(): void {
    user.clear()
    project.clear()
    message.info(t('user.logoutDone'))
    void router.push('/login')
  }
}
