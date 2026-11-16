import {test,expect} from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import {catalogueAPI} from './catalogue-fixture.ts'
test('privacy and reporting pages have labelled controls, keyboard access and no axe violations',async({page})=>{
 await catalogueAPI(page)
 await page.route('**/api/v1/me/privacy-requests*',route=>route.fulfill({json:[]}))
 await page.setViewportSize({width:320,height:740})
 for(const path of ['/account/privacy','/report-content','/manage/trader','/manage/financial','/','/search','/signin']){
  await page.goto(path);await page.getByRole('heading',{level:1}).waitFor()
  await page.keyboard.press('Tab');await expect(page.getByRole('link',{name:'Skip to main content'})).toBeFocused();await page.keyboard.press('Enter');await expect(page.locator('#main-content')).toBeFocused()
  const result=await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa','wcag21a','wcag21aa','wcag22aa']).analyze();expect(result.violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>({html:n.html,summary:n.failureSummary}))}))).toEqual([])
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
 }
 await page.goto('/account/privacy');await page.screenshot({path:'/tmp/restringing-privacy-mobile.png',fullPage:true})
})
