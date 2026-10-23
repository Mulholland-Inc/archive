import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

const api = process.env.ARCHIVE_API ?? 'http://localhost:8080';
const proxy = Object.fromEntries(['/api', '/auth', '/files', '/healthz', '/mcp'].map((p) => [p, { target: api, changeOrigin: false }]));

export default defineConfig({
  plugins: [svelte()],
  base: '/',
  build: { outDir: '../../src/web/dist', emptyOutDir: true },
  server: { port: 5180, proxy }
});
