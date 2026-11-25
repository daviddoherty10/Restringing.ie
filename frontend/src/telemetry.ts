import posthog from 'posthog-js'
import 'posthog-js/dist/exception-autocapture'
import { sanitizeException } from './lib/errorTelemetry'

const token = import.meta.env.VITE_POSTHOG_PROJECT_TOKEN
const host = import.meta.env.VITE_POSTHOG_HOST
export let telemetryEnabled = false
let identifiedUserID: string | undefined

export function identifyUser(userID: string): void {
  if (!telemetryEnabled || !userID || identifiedUserID === userID) return
  if (identifiedUserID) posthog.reset()
  posthog.identify(userID)
  identifiedUserID = userID
}

export function resetUser(): void {
  if (telemetryEnabled) posthog.reset()
  identifiedUserID = undefined
}

const posthogLogExporter = {
  info(message: string, attributes: Record<string, string | number | boolean>): void {
    if (telemetryEnabled) posthog.logger.info(message, attributes)
  },
}

export function logBookingCreated(paymentMethod: 'cash' | 'onsite' | 'online', isHybrid: boolean, express: boolean): void {
  posthogLogExporter.info('booking created', {
    event: 'booking_created',
    payment_method: paymentMethod,
    is_hybrid: isHybrid,
    express,
  })
}

if (token && host) {
  try {
    const anonymousID = crypto.randomUUID()
    posthog.init(token, {
      api_host: host,
      defaults: '2026-08-30',
      capture_exceptions: {
        capture_unhandled_errors: true,
        capture_unhandled_rejections: true,
        capture_console_errors: false,
      },
      logs: {
        serviceName: 'restringing-web',
        environment: import.meta.env.MODE,
        serviceVersion: import.meta.env.VITE_RELEASE || 'development',
      },
      before_send: event => sanitizeException(event, window.location.origin, import.meta.env.VITE_RELEASE || 'development', anonymousID),
    })
    telemetryEnabled = true
    posthogLogExporter.info('browser telemetry initialized', { event: 'browser_telemetry_initialized' })
  } catch {
    console.error('Error tracking could not start; the application will continue.')
  }
} else if (import.meta.env.DEV) {
  const missing = !token ? 'VITE_POSTHOG_PROJECT_TOKEN' : 'VITE_POSTHOG_HOST'
  console.error(new Error(`${missing} variable required by PostHog is missing or un-configured, this causes events to be silently missed. This error stops appearing once ${missing} is configured`))
}
