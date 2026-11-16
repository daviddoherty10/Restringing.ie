import {expect,test} from '@playwright/test'
import {catalogueAPI,catalogueID} from './catalogue-fixture.ts'
test('public unlawful-content notice needs accurate grounds and gives a private receipt',async({page})=>{
 await catalogueAPI(page);let sent=0
 await page.route('**/api/v1/content-notices',route=>{const body=route.request().postDataJSON();expect(body.good_faith).toBe(true);expect(body.kind).toBe('unlawful_content');sent++;return route.fulfill({json:{id:catalogueID(81),status:'submitted'}})})
 await page.goto('/report-content');await page.getByLabel('Exact content location').fill('https://example.invalid/listing');await page.getByLabel('Reasons and relevant facts').fill('TEST grounds for human review');await page.getByLabel('Contact email').fill('test@example.invalid');await page.getByLabel('I believe in good faith').check();await page.getByRole('button',{name:'Submit report'}).click();await expect(page.getByRole('status').filter({hasText:'Report received'})).toBeVisible();expect(sent).toBe(1)
})
