import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import inertia from '@inertiajs/vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig(({ command }) => ({
  plugins: [react(), inertia({ ssr: { entry: 'resources/js/ssr.tsx' } }), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./resources/js', import.meta.url)),
    },
  },
  base: command === 'build' ? './' : '/assets/dist/',
  // Bundle dependencies into the production SSR build so the embedded
  // assets/dist/ssr/ssr.js runs without node_modules. The dev server keeps them
  // external so Vite can serve SSR under `andurel run`.
  ssr: command === 'build' ? { noExternal: true } : undefined,
  build: {
    manifest: "vite/manifest.json",
    assetsDir: "",
    outDir: 'assets/dist',
    rollupOptions: {
      input: 'resources/js/app.tsx',
    },
  },
  server: {
    port: 5173,
    strictPort: true,
  },
}))
