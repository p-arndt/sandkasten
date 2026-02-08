import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		proxy: {
			'/api': {
				target: 'http://localhost:8080',
				changeOrigin: true
			},
			'/v1': {
				target: 'http://localhost:8080',
				changeOrigin: true
			},
			'/healthz': {
				target: 'http://localhost:8080',
				changeOrigin: true
			}
		}
	}
});
