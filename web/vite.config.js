import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
export default defineConfig({
  plugins: [sveltekit()],
  server: { proxy: { '/api': process.env.DOKYR_API_PROXY || 'http://localhost:8080' } }
});
