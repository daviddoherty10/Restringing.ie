import {expect,test} from '@playwright/test'
import {catalogueAPI,catalogueID} from './catalogue-fixture.ts'
test('privacy request submits once and shows the recorded response deadline',async({page})=>{
 await catalogueAPI(page);const rows:object[]=[]
 await page.route('**/api/v1/me/privacy-requests*',async route=>{if(route.request().method()==='POST'){expect(route.request().headers()['idempotency-key']).toBeTruthy();const row={...route.request().postDataJSON(),id:catalogueID(99),user_id:catalogueID(1),status:'submitted',outcome:'',created_at:'2026-11-08T12:00:00Z',due_at:'2026-12-08T12:00:00Z',version:1};rows.push(row);return route.fulfill({json:row})}return route.fulfill({json:rows})})
 await page.goto('/account/privacy');await page.getByLabel('Details',{exact:true}).fill('Please provide my account records');await page.getByRole('button',{name:'Submit request',exact:true}).click();await expect(page.getByRole('heading',{name:'access: submitted'})).toBeVisible();expect(rows).toHaveLength(1);await expect(page.getByText('Response due:')).toBeVisible()
})
