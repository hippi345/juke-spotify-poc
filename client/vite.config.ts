import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const backend = 'http://127.0.0.1:8080'

const proxy = {
  '/api': {
    target: backend,
    changeOrigin: true,
  },
  '/health': {
    target: backend,
    changeOrigin: true,
  },
} as const

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy,
  },
  // `vite preview` does not inherit `server.proxy` — without this, /api/* returns 404.
  preview: {
    port: 5173,
    proxy,
  },
})
