import test from 'node:test'
import assert from 'node:assert/strict'
import { centsToEuro,kilogramsToCentiKg,inTensionRange,pageCount } from '../src/lib/marketplace-units.ts'
test('integer EUR cents stay exact',()=>{assert.equal(centsToEuro(2450),24.5);assert.equal(centsToEuro(0),0);assert.throws(()=>centsToEuro(2.5))})
test('centi-kilogram conversion',()=>{assert.equal(kilogramsToCentiKg('24.5'),2450);assert.equal(kilogramsToCentiKg(''),null);assert.equal(kilogramsToCentiKg('24.555'),null)})
test('capability-driven tension increments',()=>{assert.equal(inTensionRange('24.5',2000,2600,50),true);assert.equal(inTensionRange('24.3',2000,2600,50),false);assert.equal(inTensionRange('27',2000,2600,50),false)})
test('server pagination calculation',()=>{assert.equal(pageCount(21,10),3);assert.equal(pageCount(0,10),0)})

import { enteredTension,tensionDisplay } from '../src/lib/marketplace-units.ts'
test('pound entry rounds explicitly to supported kg increments without exceeding limits',()=>{
 assert.equal(enteredTension('48','lb',1800,3000,50),2200)
 assert.equal(enteredTension('53','lb',1800,3000,50),2400)
 assert.equal(enteredTension('200','lb',1800,3000,50),null)
 assert.equal(enteredTension('1','lb',1800,3000,50),null)
 assert.equal(enteredTension('','lb',1800,3000,50),null)
 assert.equal(enteredTension('NaN','lb',1800,3000,50),null)
 for(const n of [1800,2200,3000])assert.equal(enteredTension(tensionDisplay(n,'lb'),'lb',1800,3000,50),n)
 assert.equal(tensionDisplay(2200,'lb'),'48.50')
})
