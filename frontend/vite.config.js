import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // Keeps `npm run dev` working now that api.js uses a relative `/api`
    // path — the dev server forwards those requests to the Go backend.
    // `host: true` also lets you still hit this from your phone on the
    // LAN, same as before.
    host: true,
    proxy: {
      '/api': 'http://localhost:8080',
      '/health': 'http://localhost:8080',
    },
  },
})
