// 应用入口：装配 Pinia、Router、i18n，挂载根组件。
import { createPinia } from 'pinia'
import { createApp } from 'vue'
import App from './App.vue'
import { setUnauthorizedHandler } from './api/http'
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

// 注入「未登录跳登录页」：http 不再静态依赖 router（见 api/http.ts 的说明），
// 否则 router → 视图 → http → router 成环，首次导航会死锁成白屏。
setUnauthorizedHandler(() => {
    void router.push('/login')
})

// 直接 mount：不要在 router.isReady() 之后才挂载——首次导航若被卡住，isReady 永不 resolve，
// 会从「渲染空」退化成「连 app 都不挂载」，更难判断（P0-1 排查时踩过）。
app.mount('#app')
