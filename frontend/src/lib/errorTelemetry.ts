import type { CaptureResult } from 'posthog-js'

// Allowlist error locations; free-text messages and SDK/browser properties can contain personal data.
export function sanitizeException(event: CaptureResult | null, origin: string, release: string, anonymousID: string): CaptureResult | null {
  if (!event || event.event !== '$exception') return event
  const exceptions = event.properties.$exception_list as { stacktrace?: { frames?: { filename?: string; lineno?: number; colno?: number }[] } }[] | undefined
  if (!Array.isArray(exceptions)) return null
  return {
    ...event,
    properties: {
      token: event.properties.token,
      $lib: event.properties.$lib,
      $lib_version: event.properties.$lib_version,
      $exception_level: 'error',
      distinct_id: anonymousID,
      $process_person_profile: false,
      $geoip_disable: true,
      $exception_release: release,
      $exception_list: exceptions.map(exception => ({
        type: 'Error',
        value: 'Application error (message redacted)',
        stacktrace: { type: 'raw', frames: (Array.isArray(exception?.stacktrace?.frames) ? exception.stacktrace.frames : []).flatMap(frame => {
          try {
            const url = new URL(frame?.filename ?? '', origin)
            if (url.origin !== origin || !/^\/assets\/[\w.-]+\.js$/.test(url.pathname)) return []
            return [{ platform: 'web:javascript', function: '[redacted]', in_app: true, filename: origin + url.pathname,
              lineno: Number.isInteger(frame.lineno) ? frame.lineno : undefined,
              colno: Number.isInteger(frame.colno) ? frame.colno : undefined }]
          } catch { return [] }
        }) },
      })),
    },
  }
}
