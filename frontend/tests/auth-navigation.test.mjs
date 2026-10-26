import test from 'node:test'
import assert from 'node:assert/strict'
import { safeReturnTo, internationalPhone, signInURL } from '../src/lib/auth-navigation.ts'
import { loadBookingDraft, saveBookingDraft } from '../src/lib/booking-draft.ts'
test('sign-in redirects only to known internal routes', () => {
  const route = '/stringers/90000000-0000-4000-8000-000000000101/book'
  assert.equal(safeReturnTo(route), route)
  assert.equal(safeReturnTo('/manage'), '/manage')
  assert.equal(signInURL(route), '/signin?returnTo=' + encodeURIComponent(route))
  for (const bad of ['https://evil.example', '//evil.example', '/\\evil.example', '/%2f%2fevil.example', '/signin?returnTo=//evil', '/orders?next=evil', null]) assert.equal(safeReturnTo(bad), '/orders')
})
test('Irish and international phone entry preserves explicit country code', () => {
  assert.equal(internationalPhone('087 123 4567', '+353'), '+353871234567')
  assert.equal(internationalPhone('+44 (7700) 900-888', '+353'), '+447700900888')
  assert.equal(internationalPhone('00447700900888', '+353'), '+447700900888')
})
test('booking draft retains setup and pending idempotency identity, expires after 30 minutes', () => {
  const data = new Map()
  globalThis.sessionStorage = {getItem: k => data.get(k) ?? null, setItem: (k,v) => data.set(k,v), removeItem: k => data.delete(k)}
  const draft = {savedAt:Date.now(),racketID:'racket',variantID:'variant',variantModel:'model',stringSource:'stock',mains:'24',crosses:'23',split:true,knots:4,stencil:true,prestretch:5,notes:'Keep this setup',attempt:{quoteID:'quote',key:'same-key'}}
  assert.equal(saveBookingDraft('shop', draft), true)
  assert.deepEqual(loadBookingDraft('shop'), draft)
  assert.equal(loadBookingDraft('other-shop'), null)
  saveBookingDraft('shop', {...draft,savedAt:Date.now()-31*60*1000})
  assert.equal(loadBookingDraft('shop'), null)
  saveBookingDraft('shop', {...draft,attempt:{}})
  assert.equal(loadBookingDraft('shop'), null)
  delete globalThis.sessionStorage
})
