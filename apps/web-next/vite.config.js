import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

const kit = new URL('../kit', import.meta.url).pathname;

export default defineConfig({
  plugins: [svelte()],
  resolve: { alias: { '#kit': kit } },
  server: { port: 5174, strictPort: true, fs: { allow: ['.', kit] } }
});
