// 应用入口：装配 Pinia、Router、i18n，挂载根组件。
import { createPinia } from 'pinia'
import { createApp } from 'vue'
import App from './App.vue'
import { i18n } from './i18n'
import router from './router'
import './styles/app.css'

createApp(App).use(createPinia()).use(router).use(i18n).mount('#app')
