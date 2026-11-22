import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// В dev-режиме API/WebSocket/медиа проксируются на локальный backend.
export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/media': { target: 'http://localhost:8080', changeOrigin: true },
      '/ws': { target: 'ws://localhost:8080', ws: true },
    },
  },
})
