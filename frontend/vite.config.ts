import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': path.resolve(import.meta.dirname, './src') },
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
      '/up': 'http://localhost:8080',
      // Claude's MCP endpoint and its OAuth routes are served by Go too.
      '/mcp': 'http://localhost:8080',
      '/oauth': 'http://localhost:8080',
      '/.well-known': 'http://localhost:8080',
      // Company pages live under /w/<slug>/; their API calls must reach Go too.
      '^/w/[^/]+/(api/|auth/|up$|mcp$|oauth/|\\.well-known/)': 'http://localhost:8080',
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
    // Screen tests are slow on WSL under load; 5 s timed out at random.
    testTimeout: 15000,
  },
})
