import { defineConfig } from 'vite'
import cesium from 'vite-plugin-cesium'

export default defineConfig({
  plugins: [cesium()],
  // satellite.js ships an optional wasm runtime whose worker uses top-level
  // await; the ES worker format lets it bundle (it is never loaded at runtime).
  worker: { format: 'es' },
  server: {
    port: 3000,
    proxy: {
      '/api': 'http://localhost:8090',
    },
  },
})
