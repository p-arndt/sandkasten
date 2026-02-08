import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	kit: {
		adapter: adapter({
			pages: '../internal/web/dist',
			assets: '../internal/web/dist',
			fallback: 'index.html',
			precompress: false,
			strict: true
		}),
		alias: {
			'$lib': './src/lib',
			'$lib/*': './src/lib/*'
		}
	}
};

export default config;
