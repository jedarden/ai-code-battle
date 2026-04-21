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
          if (id.includes('node_modules')) return;

          // Agentation: React + agentation library (lazy-loaded only on /feedback)
          if (id.includes('react') || id.includes('agentation')) {
            return 'agentation';
          }
          // Replay viewer chunk (canvas renderer + win probability)
          if (id.includes('replay-viewer') || id.includes('win-probability')) {
            return 'replay-viewer';
          }
          // Replay page (uses replay-viewer, separate from the viewer chunk itself)
          if (id.includes('pages/replay')) {
            return 'replay-page';
          }
          // Sandbox chunk (includes engine orchestration + WASM loader)
          if (id.includes('pages/sandbox')) {
            return 'sandbox';
          }
          // Evolution page (live polling, SVG lineage tree, island grid)
          if (id.includes('pages/evolution')) {
            return 'evolution';
          }
          // Blog pages (markdown parsing)
          if (id.includes('pages/blog')) {
            return 'blog';
          }
          // Clip maker (video/GIF export)
          if (id.includes('pages/clip-maker')) {
            return 'clip-maker';
          }
          // Series/predictions (chart-heavy)
          if (id.includes('pages/series') || id.includes('pages/predictions')) {
            return 'charts';
          }
          // Feedback page (includes its own replay viewer + triggers agentation load)
          if (id.includes('pages/feedback')) {
            return 'feedback';
          }
          // Home page (hero, playlists carousel, season bar, evolution mini)
          if (id.includes('pages/home')) {
            return 'home';
          }
          // Leaderboard (rating table)
          if (id.includes('pages/leaderboard')) {
            return 'leaderboard';
          }
        },
      },
    },
  },
  server: {
    port: 3000,
  },
})
