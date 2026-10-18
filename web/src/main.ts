// 应用入口：装配 Pinia、Router、i18n，挂载根组件。
import { createPinia } from 'pinia'
import { createApp } from 'vue'
import App from './App.vue'
import { i18n } from './i18n'
import router from './router'
import './styles/app.css'

const app = createApp(App)

// 渲染/导航异常必须落到控制台：此前刷新或深链接进受保护页会白屏，而控制台一片安静，
// 排查只能靠猜。这里把 Vue 渲染错误与路由错误显式打出来（生产环境同样保留）。
app.config.errorHandler = (err, _inst, info) => {
  console.error('[vue]', info, err)
}

app.use(createPinia()).use(router).use(i18n)

// 注意：不要在 router.isReady() 之后才 mount —— 实测首次导航在「带登录态刷新受保护页」时
// 会一直 pending，isReady 永不 resolve，反而从「渲染空」变成「连 app 都不挂载」。
// 白屏的根因仍在首次导航本身（见 doc/web-ui-review-2026-10-18/README.md 的 P0-1）。
app.mount('#app')
