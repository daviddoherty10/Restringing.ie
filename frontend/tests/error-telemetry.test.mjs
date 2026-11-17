import test from 'node:test'
import assert from 'node:assert/strict'
import { sanitizeException } from '../src/lib/errorTelemetry.ts'

test('error telemetry drops analytics and strips personal data while preserving bundle locations', () => {
  const origin = 'https://preview.restringing.ie'
  const anonymousID = '90000000-0000-4000-8000-000000000001'
  const sanitize = event => sanitizeException(event, origin, 'test-release', anonymousID)
  assert.equal(sanitize(null), null)
  assert.equal(sanitize({ event: '$pageview', properties: {} }), null)
  const input = { event: '$exception', properties: {
    token: 'public-project-token', $lib: 'web', $lib_version: 'test',
    distinct_id: 'private-phone', $current_url: origin + '/orders/private-order?token=private-token',
    $exception_message: 'private-message', $set: { email: 'private-email' },
    $exception_list: [{ type: 'private-type', value: 'private-message', stacktrace: { frames: [
      { filename: origin + '/assets/index-12345678.js?secret=private-query#private-hash', lineno: 12, colno: 3, function: 'private-function', vars: { token: 'private-token' } },
      { filename: 'https://external.invalid/private-path.js', lineno: 1 },
      { filename: origin + '/orders/private-order' },
      { filename: origin + '/assets/index-12345678.js', lineno: 'private-line', colno: 'private-column' },
    ] } }],
  } }
  const result = sanitize(input)
  assert.equal(result.properties.distinct_id, anonymousID)
  assert.equal(result.properties.token, 'public-project-token')
  assert.equal(result.properties.$process_person_profile, false)
  assert.equal(result.properties.$exception_list[0].stacktrace.frames[0].filename, origin + '/assets/index-12345678.js')
  assert.equal(result.properties.$exception_list[0].stacktrace.frames[0].lineno, 12)
  assert.equal(result.properties.$exception_list[0].stacktrace.frames.length, 2)
  assert.ok(!JSON.stringify(result).includes('private-'))
  assert.ok(JSON.stringify(input).includes('private-message'))
})
