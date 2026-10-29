import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

const kit = new URL('../kit', import.meta.url).pathname;

export default defineConfig({
  plugins: [sveltekit({ adapter: adapter() })],
  resolve: { alias: { '#kit': kit } },
  server: { fs: { allow: ['.', kit] } }
});
