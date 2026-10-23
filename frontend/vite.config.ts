import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

// https://vite.dev/config/
export default defineConfig({
  // 桌面应用：构建产物由 Go 内嵌托管，相对路径保证在 wails 自定义协议下可用
  base: './',
  plugins: [
    vue(),
    ...(process.env.NODE_ENV === 'production' ? [] : [vueDevTools()]),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    outDir: '../web/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5174,
    proxy: {
      '/api': {
        target: 'http://localhost:8530',
        changeOrigin: true,
      },
    },
  },
})
