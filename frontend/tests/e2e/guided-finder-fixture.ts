import type { Page } from '@playwright/test'
import { hybridsAPI } from './hybrids-fixture.ts'
import type { components } from '../../src/lib/marketplace-contract.d.ts'
type S = components['schemas']
export async function guidedFinderAPI(page: Page) {
  const f = await hybridsAPI(page)
  Object.assign(f.variants[0], {
    brand: 'HEAD',
    model: 'Velocity MLT',
    material: 'multifilament',
    material_price_cents: 1000,
  })
  Object.assign(f.variants[1], {
    brand: 'Solinco',
    model: 'Hyper-G',
    material: 'polyester',
    material_price_cents: 1200,
  })
  const answers: S['RecommendationRequest'][] = []
  let empty = false,
    fail = false,
    delay = 0
  await page.route('**/recommendations', async (route) => {
    const req = route.request().postDataJSON() as S['RecommendationRequest']
    answers.push(req)
    if (delay) await new Promise((r) => setTimeout(r, delay))
    if (fail) {
      fail = false
      return route.fulfill({
        status: 503,
        json: {
          error: {
            code: 'unavailable',
            message: 'Finder temporarily unavailable.',
          },
        },
      })
    }
    const juniorSpin = req.round_poly_full_bed === true && req.goal === 'spin'
    const hybrid = req.setup_feedback !== 'no_poly' && !req.prioritise_arm_comfort && !juniorSpin && (
      req.goal === 'balanced' && req.age_group === 'adult' && req.experience !== 'new' ||
      req.goal === 'spin' && (!req.age_group || req.age_group === 'adult') && req.experience !== 'new'
    )
    const variant = hybrid || juniorSpin ? f.variants[1] : f.variants[0]
    const lb = req.tension_help || req.tension_band ? 0 : req.current_tension_centi_lb ??
      (!req.current_tension_centi_kg && (hybrid && req.goal === 'balanced' || juniorSpin && req.age_group === '15_to_17') ? 4800 : 0)
    const kg = req.tension_help || req.tension_band ? 0 : req.current_tension_centi_kg ?? (req.racket_model_id ? 2200 : 0)
    const booking: S['Booking'] = {
      ...(req.racket_model_id ? {racket_model_id: req.racket_model_id} : {sport: req.sport ?? 'tennis'}),
      tension_pending: !lb && !kg,
      ...(req.tension_band ? { tension_band: req.tension_band } : {}),
      ...(lb ? { mains_tension_centi_lb: lb, crosses_tension_centi_lb: lb } : {}),
      string_source: 'stock',
      variant_id: variant.id,
      mains_tension_centi_kg: lb ? 0 : kg,
      crosses_tension_centi_kg: lb ? 0 : kg,
      knots: 4,
      prestretch_percent: 0,
      stencil: false,
      express: false,
      ...(hybrid ? { hybrid: true, crosses_variant_id: f.variants[0].id } : {}),
    }
    const result: S['Recommendations'] = {
      algorithm: 'guided-setup-v2',
      notice:
        'Editorial preference matching; request a fresh server quote. Answers are not saved.',
      results: empty
        ? []
        : [
            {
              variant,
              score: 80,
              reasons: ['comfort: weight 4 × catalogue rating 5'],
              setup: {
                booking,
                summary: hybrid
                  ? 'Two strings for spin and feel.'
                  : 'One string throughout, with a balanced starting tension.',
                explanations: [
                  'The stringing method is selected for this racket.',
                  'No pre-stretch or optional extras.',
                ],
                estimated_total_cents: hybrid ? 2700 : 2600,
                breakdown: [
                  { code: 'labour', label: 'Labour', amount_cents: 1500 },
                  {
                    code: 'strings',
                    label: 'Strings',
                    amount_cents: hybrid ? 1100 : 1000,
                  },
                  {
                    code: 'knots',
                    label: 'Stringing method',
                    amount_cents: 100,
                  },
                ],
                ...(hybrid ? { crosses_variant: f.variants[0] } : {}),
              },
            },
          ],
    }
    return route.fulfill({ json: result })
  })
  return {
    ...f,
    answers,
    setEmpty: (v: boolean) => {
      empty = v
    },
    fail: () => {
      fail = true
    },
    delay: (ms: number) => {
      delay = ms
    },
  }
}
