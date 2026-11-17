import {test,expect} from '@playwright/test'
import {hybridsAPI} from './hybrids-fixture.ts'
import {dashboardAPI,shopID} from './dashboard-fixture.ts'

test('search keeps its current filter responsive and identifies the active destination',async({page})=>{
 await dashboardAPI(page)
 await page.route('**/api/v1/stringers?*',route=>route.fulfill({json:{data:[],page:1,page_size:12,total:0}}))
 await page.goto('/search')
 await expect(page.getByRole('heading',{name:'No stringers found'})).toBeVisible()
 await page.getByRole('button',{name:'All rackets',exact:true}).click()
 await expect(page.getByRole('heading',{name:'No stringers found'})).toBeVisible()
 await expect(page.getByText('Loading approved stringers…')).toHaveCount(0)
 if(await page.getByRole('button',{name:'Open menu',exact:true}).isVisible())await page.getByRole('button',{name:'Open menu',exact:true}).click()
 await expect(page.getByRole('link',{name:'Find a stringer',exact:true})).toHaveAttribute('aria-current','page')
})
test('booking keeps comfortable control sizes and its mobile layout',async({page})=>{
 await hybridsAPI(page);await page.setViewportSize({width:320,height:740});await page.goto(`/stringers/${shopID}/book`)
 await page.getByRole('heading',{level:1}).waitFor()
 await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
 const fields=page.locator('.booking-field input:visible,.booking-field select:visible');await expect(fields.first()).toBeVisible()
 for(const field of await fields.all()){
  const style=await field.evaluate(el=>({font:parseFloat(getComputedStyle(el).fontSize),height:el.getBoundingClientRect().height}))
  expect(style.font).toBeGreaterThanOrEqual(16);expect(style.height).toBeGreaterThanOrEqual(44)
 }
 await page.getByRole('heading',{level:1}).click();await page.evaluate(()=>window.scrollTo(0,0));await page.screenshot({path:`/tmp/restringing-design-booking-${test.info().project.name}.png`,fullPage:true})
})
test('dashboard remains usable with larger text and reduced motion',async({page})=>{
 await dashboardAPI(page);await page.emulateMedia({reducedMotion:'reduce'});await page.goto('/manage');await expect(page.getByRole('heading',{name:'Preview Baseline',exact:true})).toBeVisible()
 await page.addStyleTag({content:'html{font-size:125%}'});await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
 const tab=page.getByRole('button',{name:'Profile',exact:true});await tab.click();await expect(tab).toHaveAttribute('aria-pressed','true')
 await expect(page.getByLabel('Business name',{exact:true})).toBeVisible();expect(await tab.evaluate(el=>parseFloat(getComputedStyle(el).transitionDuration))).toBeLessThanOrEqual(0.001)
 await page.getByRole('heading',{level:1}).click();await page.evaluate(()=>window.scrollTo(0,0));await page.screenshot({path:`/tmp/restringing-design-dashboard-${test.info().project.name}.png`,fullPage:true})
})

test('header gives press feedback and groups account destinations',async({page})=>{
 await dashboardAPI(page);await page.goto('/')
 const mobile=await page.getByRole('button',{name:'Open menu',exact:true}).isVisible()
 const toggle=mobile?page.getByRole('button',{name:'Open menu',exact:true}):page.getByRole('button',{name:'Account',exact:true})
 await expect(toggle).toBeVisible();await toggle.hover();await page.mouse.down();await expect(toggle).not.toHaveCSS('transform','none');await page.mouse.up()
 const nav=page.getByRole('navigation',{name:'Main navigation'});await expect(nav.getByText('Your business',{exact:true})).toBeVisible();await expect(nav.getByRole('link',{name:'My orders',exact:true})).toBeVisible();await expect(nav.getByRole('link',{name:'Stringer portal',exact:true})).toBeVisible()
 await page.screenshot({path:`/tmp/restringing-design-navigation-${test.info().project.name}.png`})
 await page.keyboard.press('Escape');await expect(toggle).toBeFocused();await expect(page.getByRole('link',{name:'My orders',exact:true})).toBeHidden()
 await page.screenshot({path:`/tmp/restringing-design-home-${test.info().project.name}.png`})
})
