import {expect,test} from '@playwright/test'
import {catalogueAPI,catalogueID} from './catalogue-fixture.ts'
test('owner submits distinct private and public identity for review',async({page})=>{
 await catalogueAPI(page);const business=catalogueID(101);let document:unknown
 await page.route(`**/api/v1/stringers/${business}/trader`,async route=>{if(route.request().method()==='PUT'){document=route.request().postDataJSON().document;return route.fulfill({json:{stringer_id:business,document,status:'submitted',version:1,reason:'',updated_at:new Date().toISOString()}})}return route.fulfill({status:404,json:{error:{message:'Not submitted'}}})})
 await page.goto('/manage/trader?business='+business);await page.getByRole('button',{name:'Load verification'}).click()
 for(const label of ['legal name','address','email','phone'])await page.getByLabel(label,{exact:true}).fill('TEST private '+label)
 for(const label of ['name','address','email','phone','trader status'])await page.getByLabel('Public '+label,{exact:true}).fill('TEST public '+label)
 await page.getByLabel('I confirm these details').check();await page.getByRole('button',{name:'Submit for review'}).click();await expect(page.getByRole('status').filter({hasText:'submitted:'})).toBeVisible();expect(document).toMatchObject({legal_name:'TEST private legal name',public:{name:'TEST public name'}})
})
