import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')
    }
  },
  server: {
    // 开发时前端单独端口，避免与正式服务 8021 混淆；正式使用只开 go run main.go，访问 8021 即可
    port: 8022,
    proxy: {
      '/api': {
        target: 'http://localhost:8021',
        changeOrigin: true
      },
      '/v1': {
        target: 'http://localhost:8021',
        changeOrigin: true
      },
      '/pool': {
        target: 'http://localhost:8021',
        changeOrigin: true
      }
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true
  }
})
