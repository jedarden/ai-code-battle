import { defineConfig } from 'vite'
import { resolve } from 'path'

export default defineConfig({
  root: '.',
  build: {
    outDir: 'dist',
    sourcemap: true,
    rollupOptions: {
      input: {
        main: resolve(__dirname, 'index.html'),
        app: resolve(__dirname, 'app.html'),
        embed: resolve(__dirname, 'embed.html'),
      },
      output: {
        manualChunks(id) {
          // Agentation: React + agentation library (lazy-loaded)
          if (id.includes('react') || id.includes('agentation')) {
            return 'agentation';
          }
          // Replay viewer chunk (includes canvas rendering, charts)
          if (id.includes('replay-viewer') || id.includes('win-probability')) {
            return 'replay-viewer';
          }
          // Sandbox chunk (includes engine orchestration)
          if (id.includes('pages/sandbox')) {
            return 'sandbox';
          }
          // Evolution page (large, complex visualizations)
          if (id.includes('pages/evolution')) {
            return 'evolution';
          }
          // Blog pages (markdown parsing)
          if (id.includes('pages/blog')) {
            return 'blog';
          }
          // Clip maker (video processing)
          if (id.includes('pages/clip-maker')) {
            return 'clip-maker';
          }
          // Series/predictions (chart-heavy)
          if (id.includes('pages/series') || id.includes('pages/predictions')) {
            return 'charts';
          }
          // Feedback page (includes its own replay viewer)
          if (id.includes('pages/feedback')) {
            return 'feedback';
          }
        },
      },
    },
  },
  server: {
    port: 3000,
  },
})
