import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'
import { defineConfig, loadEnv } from 'vite'

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
    const env = loadEnv(mode, process.cwd(), 'VITE_')
    // 后端代理目标：与 server/bin/env.test 的 PORT 保持一致（默认 2027）。
    const target = env.VITE_PROXY_TARGET || 'http://127.0.0.1:2027'

    return {
        plugins: [vue()],
        resolve: {
            alias: {
                '@': fileURLToPath(new URL('./src', import.meta.url)),
            },
        },
        server: {
            port: 5173,
            // 将 /api 与 /uploads 代理到后端，避免开发期跨域；统一响应见开发规范 §2.1。
            proxy: {
                '/api': { target, changeOrigin: true },
                '/uploads': { target, changeOrigin: true },
                // Mock 服务：与后端同源部署时不需要代理，dev 下转发到后端
                '/mock': { target, changeOrigin: true },
            },
        },
    }
})
