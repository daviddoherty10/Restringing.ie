import test from 'node:test'
import assert from 'node:assert/strict'
import { safeReturnTo, internationalPhone, signInURL } from '../src/lib/auth-navigation.ts'
test('sign-in redirects only to known internal routes', () => {
  const route = '/stringers/90000000-0000-4000-8000-000000000101/book'
  assert.equal(safeReturnTo(route), route)
  assert.equal(safeReturnTo('/manage'), '/manage')
  for (const adminRoute of ['/admin', '/admin/catalogue', '/admin/complaints']) assert.equal(safeReturnTo(adminRoute), adminRoute)
  assert.equal(safeReturnTo('/admin/unknown'), '/orders')
  assert.equal(safeReturnTo('/admin/catalogue?next=//evil.example'), '/orders')
  assert.equal(signInURL(route), '/signin?returnTo=' + encodeURIComponent(route))
  for (const bad of ['https://evil.example', '//evil.example', '/\\evil.example', '/%2f%2fevil.example', '/signin?returnTo=//evil', '/orders?next=evil', null]) assert.equal(safeReturnTo(bad), '/orders')
})
test('Irish and international phone entry preserves explicit country code', () => {
  assert.equal(internationalPhone('087 123 4567', '+353'), '+353871234567')
  assert.equal(internationalPhone('+44 (7700) 900-888', '+353'), '+447700900888')
  assert.equal(internationalPhone('00447700900888', '+353'), '+447700900888')
})
test('booking draft retains setup and pending idempotency identity, expires after 30 minutes and invalidates across logout', async () => {
  const shared = new Map()
  let quotaFull = false
  globalThis.localStorage = {
    getItem: k => shared.get(k) ?? null,
    setItem: (k,v) => { if (quotaFull) throw Error('quota'); shared.set(k,v) },
    removeItem: k => shared.delete(k),
  }
  const { loadBookingDraft, saveBookingDraft, clearBookingDrafts, watchForLogout } = await import('../src/lib/booking-draft.ts')
  globalThis.sessionStorage = {
    getItem(k) { return this[k] ?? null },
    setItem(k,v) { this[k] = v },
    removeItem(k) { delete this[k] },
  }
  const key = 'restringing:booking:shop'
  const draft = {savedAt:Date.now(),racketID:'racket',variantID:'variant',variantModel:'model',stringSource:'stock',mains:'24',crosses:'23',split:true,knots:4,stencil:true,prestretch:5,notes:'Keep this setup',attempt:{quoteID:'quote',key:'same-key'}}
  sessionStorage.setItem(key, JSON.stringify(draft))
  assert.equal(loadBookingDraft('shop'), null, 'legacy draft predating account binding')
  assert.equal(saveBookingDraft('shop', draft), true)
  assert.deepEqual(loadBookingDraft('shop'), draft)
  assert.equal(loadBookingDraft('other-shop'), null)
  saveBookingDraft('shop', {...draft,savedAt:Date.now()-31*60*1000})
  assert.equal(loadBookingDraft('shop'), null)
  saveBookingDraft('shop', {...draft,attempt:{}})
  assert.equal(loadBookingDraft('shop'), null)
  saveBookingDraft('shop', draft)
  const otherTabData = sessionStorage.getItem(key)
  const afterSignIn = await import('../src/lib/booking-draft.ts?after-sign-in')
  assert.deepEqual(afterSignIn.loadBookingDraft('shop'), draft, 'preserve guest setup through sign-in')
  const listeners = new Map()
  let reloads = 0
  globalThis.window = {
    addEventListener: (name, listener) => listeners.set(name,listener),
    location: {reload: () => { reloads++ }},
  }
  watchForLogout()
  quotaFull = true
  clearBookingDrafts()
  assert.equal(sessionStorage.getItem(key), null)
  sessionStorage.setItem(key, otherTabData)
  assert.equal(saveBookingDraft('shop', draft), false)
  assert.equal(loadBookingDraft('shop'), null)
  listeners.get('storage')({key:'restringing:booking-generation'})
  assert.equal(reloads, 1)
  assert.equal(sessionStorage.getItem(key), null, 'clear old bytes in the other tab')
  listeners.get('pageshow')({persisted:true})
  assert.equal(reloads, 2, 'recheck a restored document')
  const unavailable = await import('../src/lib/booking-draft.ts?quota-full')
  assert.equal(unavailable.saveBookingDraft('shop', draft), false)
  quotaFull = false
  const fresh = await import('../src/lib/booking-draft.ts?new-document')
  sessionStorage.setItem(key, otherTabData)
  assert.equal(fresh.loadBookingDraft('shop'), null)
  assert.equal(fresh.saveBookingDraft('shop', draft), true)
  assert.deepEqual(fresh.loadBookingDraft('shop'), draft)
  delete globalThis.window
  delete globalThis.localStorage
  delete globalThis.sessionStorage
})
