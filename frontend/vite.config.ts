import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// The Go API this dev server forwards to; ZIV_API points it at a test Hub
// (for example http://localhost:8092) when 8080 is taken.
const api = process.env.ZIV_API ?? 'http://localhost:8080'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': path.resolve(import.meta.dirname, './src') },
  },
  server: {
    proxy: {
      '/api': api,
      '/auth': api,
      '/up': api,
      // Claude's MCP endpoint and its OAuth routes are served by Go too.
      '/mcp': api,
      '/oauth': api,
      '/.well-known': api,
      // Company pages live under /w/<slug>/; their API calls must reach Go too.
      '^/w/[^/]+/(api/|auth/|up$|mcp$|oauth/|\\.well-known/)': api,
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
