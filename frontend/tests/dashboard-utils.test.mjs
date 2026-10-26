import test from 'node:test'
import assert from 'node:assert/strict'
import { decimalUnits, signedUnits, nextOrderStatus, cancellable } from '../src/features/management/dashboard-utils.ts'
test('entered euro prices and reel metres convert exactly to integers', () => {
  assert.equal(decimalUnits('10.99', 2), 1099)
  assert.equal(decimalUnits('0.29', 2), 29)
  assert.equal(decimalUnits('200.125', 3), 200125)
  assert.equal(signedUnits('-12.5', 3), -12500)
  for (const bad of ['', '1e3', '1,25', '1.234', '-5', 'NaN', 'Infinity']) assert.equal(decimalUnits(bad, 2), null)
})
test('fulfilment controls stop at terminal states and never reopen orders', () => {
  assert.equal(nextOrderStatus('received'), 'in_progress')
  assert.equal(nextOrderStatus('ready'), 'completed')
  assert.equal(nextOrderStatus('completed'), null)
  assert.equal(nextOrderStatus('cancelled'), null)
  assert.equal(cancellable('received'), true)
  assert.equal(cancellable('in_progress'), false)
})
