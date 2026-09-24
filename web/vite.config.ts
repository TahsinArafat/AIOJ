import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

export default defineConfig({
    plugins: [react(), tailwindcss()],
    resolve: {
        alias: { '@': path.resolve(__dirname, './src') },
    },
    build: {
        // The setter's VisualEditor (tiptap + prosemirror + lowlight) is the
        // one chunk that still crosses 500 kB. Shard its vendor stack into
        // separate on-demand chunks so no single file trips the size warning.
        rolldownOptions: {
            output: {
                codeSplitting: {
                    groups: [
                        { name: 'editor-highlight', test: /node_modules[\\/](highlight\.js|lowlight)[\\/]/ },
                        { name: 'editor-tiptap', test: /node_modules[\\/](@tiptap|prosemirror)[\\/]/ },
                        { name: 'editor-turndown', test: /node_modules[\\/](turndown|@mixmark-io)[\\/]/ },
                    ],
                },
            },
        },
    },
    server: {
        proxy: {
            '/api': 'http://localhost:8080',
            '/ws': { target: 'ws://localhost:8080', ws: true },
        },
    },
})
