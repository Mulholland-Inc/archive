import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

const archive = 'http://127.0.0.1:8080';
const kit = new URL('../kit', import.meta.url).pathname;

export default defineConfig({
  plugins: [svelte()],
  resolve: { alias: { '#kit': kit } },
  server: { port: 5173, strictPort: true, fs: { allow: ['.', kit] }, proxy: { '/api': archive, '/auth': archive } }
});
