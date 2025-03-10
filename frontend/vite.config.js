import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import svgr from 'vite-plugin-svgr';
import { resolve } from 'path';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react({
      // Include JSX processing in .js files, not just .jsx
      include: "**/*.{jsx,js}",
    }),
    svgr(), // Transforms SVGs into React components
  ],
  resolve: {
    alias: {
      // Add any path aliases your project uses
      '@': resolve(__dirname, 'src'),
    },
    extensions: ['.js', '.jsx', '.json'],
  },
  server: {
    port: 3000, // Use the same port as CRA for familiarity
    proxy: {
      // Configure proxy for your API requests (as in your package.json)
      '/api': {
        target: 'http://localhost:9989',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'build', // Match CRA's output directory
    chunkSizeWarningLimit: 600, // Increase from default 500kb for Patternfly
    rollupOptions: {
      output: {
        manualChunks: {
          // Group Patternfly components into a separate chunk
          'patternfly': [
            '@patternfly/react-core',
            '@patternfly/react-icons',
            '@patternfly/react-styles',
            '@patternfly/react-tokens'
          ],
          // Vendor chunk for other major dependencies
          'vendor': [
            'react',
            'react-dom',
            'react-router-dom'
          ]
        }
      }
    }
  },
});
