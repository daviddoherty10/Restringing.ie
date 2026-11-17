import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import './features/compliance/Compliance.css'
import App from './App.tsx'
import posthog from 'posthog-js'
import { PostHogProvider, PostHogErrorBoundary } from '@posthog/react'
import { telemetryEnabled } from './telemetry'

const app = <><a className="skip-link" href="#main-content">Skip to main content</a><App /></>

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    {telemetryEnabled ? <PostHogProvider client={posthog}>
      <PostHogErrorBoundary fallback={<main id="main-content"><h1>Something went wrong</h1><p>Please reload the page and try again.</p><button onClick={() => window.location.reload()}>Reload page</button></main>}>
        {app}
      </PostHogErrorBoundary>
    </PostHogProvider> : app}
  </StrictMode>,
)
