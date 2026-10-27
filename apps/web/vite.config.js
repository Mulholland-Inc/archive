import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

const archive = process.env.ARCHIVE ?? 'http://127.0.0.1:8080';

export default defineConfig({
  plugins: [svelte()],
  server: { port: 5173, proxy: { '/api': archive, '/auth': archive, '/mcp': archive } }
});
