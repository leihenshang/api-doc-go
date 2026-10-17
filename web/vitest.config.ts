// 前端单元测试配置：只跑 src 下的 *.test.ts，环境用 happy-dom（store 需要 localStorage）。
// 与 vite.config.ts 分开：构建配置不该被测试插件影响，反之亦然。
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    environment: 'happy-dom',
    include: ['src/**/*.test.ts'],
    // 不允许只跑部分用例就报绿：CI 上用 vitest run 全量执行。
    reporters: 'default',
  },
})
