import posthog from 'posthog-js'
import 'posthog-js/dist/exception-autocapture'
import { sanitizeException } from './lib/errorTelemetry'

const token = import.meta.env.VITE_POSTHOG_PROJECT_TOKEN
const host = import.meta.env.VITE_POSTHOG_HOST
export let telemetryEnabled = false

if (token && host) {
  try {
    const anonymousID = crypto.randomUUID()
    posthog.init(token, {
      api_host: host,
      defaults: '2026-08-30',
      autocapture: false,
      capture_pageview: false,
      capture_pageleave: false,
      capture_exceptions: { capture_unhandled_errors: true, capture_unhandled_rejections: true, capture_console_errors: false },
      disable_session_recording: true,
      disable_surveys: true,
      disable_persistence: true,
      person_profiles: 'never',
      advanced_disable_feature_flags: true,
      disable_external_dependency_loading: true,
      before_send: event => sanitizeException(event, window.location.origin, import.meta.env.VITE_RELEASE || 'development', anonymousID),
    })
    telemetryEnabled = true
  } catch {
    console.error('Error tracking could not start; the application will continue.')
  }
} else if (import.meta.env.DEV) {
  const missing = !token ? 'VITE_POSTHOG_PROJECT_TOKEN' : 'VITE_POSTHOG_HOST'
  console.error(new Error(`${missing} variable required by PostHog is missing or un-configured, this causes events to be silently missed. This error stops appearing once ${missing} is configured`))
}
