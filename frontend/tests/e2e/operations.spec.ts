import { expect,test } from '@playwright/test'
import { dashboardAPI,shopID } from './dashboard-fixture.ts'
const inviteID='00000000-0000-4000-8000-000000000099'
test('owner invites and revokes staff while invitee can accept',async({page})=>{
 await dashboardAPI(page)
 let pending=true
 await page.route('**/api/v1/**',async route=>{const req=route.request(),path=new URL(req.url()).pathname,method=req.method();const json=(body:unknown)=>route.fulfill({json:body})
 if(path.endsWith('/me/staff-invitations'))return json([{id:inviteID,stringer_id:shopID,business_name:'Colleague workshop',phone_number:'+353800000001',state:'pending',expires_at:'2026-12-01T00:00:00Z'}])
 if(path.endsWith('/accept'))return json({read:true})
 if(path.endsWith('/team'))return json({members:[{user_id:inviteID,phone_number:'+353800000002',role:'owner'}],invitations:pending?[{id:inviteID,stringer_id:shopID,business_name:'Preview Baseline',phone_number:'+353800000001',state:'pending',expires_at:'2026-12-01T00:00:00Z'}]:[]})
 if(path.includes('/staff-invitations')){if(method==='POST')expect(req.postDataJSON()).toEqual({phone_number:'+353800000001'});if(method==='DELETE')pending=false;return json({read:true})}
 return route.fallback()
 })
 await page.goto('/manage');await page.getByRole('button',{name:'Accept invitation to Colleague workshop'}).click()
 await page.getByRole('button',{name:'Team',exact:true}).click();await page.getByLabel('Phone number',{exact:true}).fill('+353800000001');await page.getByRole('button',{name:'Invite staff',exact:true}).click();await expect(page.getByRole('status')).toContainText('Invitation saved')
 await page.getByRole('button',{name:'Revoke invitation for +353800000001'}).click();await expect(page.getByRole('button',{name:'Revoke invitation for +353800000001'})).toHaveCount(0)
})
test('staff do not see owner team and image controls',async({page})=>{await dashboardAPI(page,'staff');await page.goto('/manage');await expect(page.getByRole('button',{name:'Team',exact:true})).toHaveCount(0);await page.getByRole('button',{name:'Profile',exact:true}).click();await expect(page.getByLabel('Profile photo',{exact:true})).toHaveCount(0)})
test('owner can upload and remove a business image',async({page})=>{
 await dashboardAPI(page);let uploaded=false,removed=false
 await page.route('**/api/v1/stringers/*/images/profile',async route=>{if(route.request().method()==='PUT'){expect(route.request().postDataJSON().data).toBeTruthy();uploaded=true;return route.fulfill({json:{version:inviteID}})}removed=true;return route.fulfill({json:{read:true}})})
 await page.goto('/manage');await page.getByRole('button',{name:'Profile',exact:true}).click()
 await page.getByLabel('Profile photo',{exact:true}).setInputFiles({name:'profile.png',mimeType:'image/png',buffer:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=','base64')})
 await expect(page.getByText('Profile photo saved.',{exact:true})).toBeVisible();expect(uploaded).toBe(true);await page.getByRole('button',{name:'Remove profile photo',exact:true}).click();await expect(page.getByText('Image removed.',{exact:true})).toBeVisible();expect(removed).toBe(true)
})
test('SMS alerts default off and can be enabled and disabled',async({page})=>{
 let enabled=false
 await page.route('**/api/v1/**',async route=>{const path=new URL(route.request().url()).pathname;if(path.endsWith('/me'))return route.fulfill({json:{id:inviteID,admin:false}});if(path.endsWith('/notification-preferences')){if(route.request().method()==='PUT')enabled=route.request().postDataJSON().sms_enabled;return route.fulfill({json:{sms_enabled:enabled}})}return route.fulfill({json:{data:[],total:0,unread_count:0,page:1,page_size:20}})})
 await page.goto('/notifications');const toggle=page.getByRole('checkbox',{name:'Send me SMS order alerts'});await expect(toggle).not.toBeChecked();await toggle.check();await expect(toggle).toBeChecked();await expect.poll(()=>enabled).toBe(true);await expect(toggle).toBeEnabled();await toggle.uncheck();await expect(toggle).not.toBeChecked();await expect.poll(()=>enabled).toBe(false)
})
