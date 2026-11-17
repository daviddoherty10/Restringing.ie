import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'
import posthog from '@posthog/rollup-plugin'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), ...(process.env.POSTHOG_API_KEY && process.env.POSTHOG_PROJECT_ID && process.env.POSTHOG_HOST ? [posthog({
    personalApiKey: process.env.POSTHOG_API_KEY,
    projectId: process.env.POSTHOG_PROJECT_ID,
    host: process.env.POSTHOG_HOST,
    sourcemaps: { enabled: true, releaseName: 'restringing-ie-web', releaseVersion: process.env.VITE_RELEASE, deleteAfterUpload: true },
  })] : [])],
})
