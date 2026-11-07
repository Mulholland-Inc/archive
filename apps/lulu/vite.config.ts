import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const target = 'http://localhost:8091';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5182,
    strictPort: true,
    proxy: {
      '/api': target,
      '/auth': target,
      '/live': { target, ws: true }
    }
  }
});
