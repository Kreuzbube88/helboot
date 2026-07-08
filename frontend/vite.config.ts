import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // The dev server proxies API calls to a locally running backend so
    // the frontend never needs its own data access (ADR-0010).
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
