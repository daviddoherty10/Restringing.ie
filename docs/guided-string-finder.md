# Guided string setups and real-product previews

The finder asks about experience, frequency and a plain-language goal, with an
optional softer-feel preference. It returns complete **editable starting setups**
from the selected stringer's available offerings. Customers do not need to choose
a knot pattern, pre-stretch percentage or mains/crosses tension difference.

## Try the real-string preview

From this branch, run (or rerun):

```bash
bash scripts/preview.sh seed
```

Restart the preview API and frontend from this checkout in separate terminals:

```bash
AUTH_SMS_PROVIDER= BILLING_PROVIDER= bash scripts/preview.sh api
```

```bash
bash scripts/preview.sh web
```

Open the [Real String Demo booking page](http://127.0.0.1:5174/stringers/90000000-0000-4000-8000-000000000108/book),
choose **Preview Rackets — Tennis All Options**, then **Help me choose a complete
setup**. This business uses only the real-product catalogue, so old fictional
strings do not crowd its results. A public recommendation needs no login.
For a final quote/order, use `bash scripts/preview.sh session customer` as usual.
The existing stringer/staff preview accounts can also manage the new demo shop.

The seed offers 21 real tennis variants at every tennis preview stringer, plus
three Yonex badminton variants and two Tecnifibre squash variants where those
sports are offered. The separate Real String Demo business remains available.
There are 11 active real products / 26 active real variants; the 40 original
fictional catalogue records are archived rather than deleted. Their stock and
immutable historical order snapshots remain intact. Prices are illustrative EUR material prices;
labour is €18. Ratings are explicitly editorial 1–5 fixtures, not manufacturer
measurements, clinical evidence or promises of performance. Existing real offerings, prices, availability, business edits, stock balances
and order history are preserved. A separate once-only extension retires only
unchanged seed-owned fictional product records and adds the new offerings.

This is a **once-only additive extension** on marked preview databases. Existing
`dataset=…fictional-preview, version=1` installations are supported: no reset or
production migration is needed. The existing loopback/database/metadata guards,
advisory lock and transaction still apply. After installation, reseeding never
resets edits or reactivates products/offers. The extension does not run in the
production HTTP server or against normal `DATABASE_URL`.

## Complete-setup rules

1. Rank using the existing weighted catalogue attributes, new-player comfort
   emphasis, frequent-play durability, advanced-play control, optional comfort
   emphasis and current-material context. Explain the score under a disclosure.
2. For new players or the explicit softer-feel preference, restrict suggestions
   to multifilament, synthetic gut, natural gut or nylon. If none is available,
   return no suitable setup rather than silently substituting polyester.
3. Suggest one string throughout by default. For regular/advanced players playing
   at least twice weekly with spin weight ≥4 and comfort weight ≥3, an offered
   polyester/softer-material hybrid can lead. Never auto-suggest it for new
   players or the explicit softer-feel flag. It is a spin/feel compromise and
   still contains polyester; it is not an injury treatment.
4. Intersect verified racket and service tension ranges and increments. Choose
   the valid increment nearest **22 kg for tennis** (lower on an exact tie),
   with equal mains/crosses tension. Other sports retain the middle valid
   increment; the tennis preference is never applied to badminton or squash. Keep an optional supplied current tension only if it is valid in
   that intersection. This is a conservative system starting rule, not a
   string-specific manufacturer tension instruction or guaranteed optimum.
5. Choose the cheaper supported knot pattern (two on a tie); a hybrid always
   requires four. Leave pre-stretch, stencil, express and grip off. Do not invent
   machine pre-stretch limits from factory string treatment.
6. Validate each proposal through the same server pricing/technical rules as a
   booking. Return an estimated total and breakdown, including labour/method
   charges. Offer distinct full-bed product alternatives, preferring a
   representative tennis gauge (1.30 mm soft material / 1.25 mm polyester), then
   lower material price and UUID on otherwise equal scores. Apply the same gauge
   tie-break to hybrid candidates; cross-string ranking puts comfort and power first.

Full-bed and hybrid candidate queries use their corresponding tracking modes.
Hybrid suggestions check combined allocations; no recommendation reserves stock.
A fresh authenticated quote and normal transactional order remain required.
Current price, service, catalogue, quota and stock checks run again there.
No client amount is accepted. Unknown rackets still require manual review.

`POST /api/v1/stringers/{id}/recommendations` retains its request shape. Each result
adds optional `setup` with `booking`, optional `crosses_variant`, `summary`,
`explanations`, `estimated_total_cents` and `breakdown`. Algorithm is
`guided-setup-v3`. Generated OpenAPI and TypeScript include the new response.
The frontend applies all choices together, collapses manual/technical controls,
and saves the normal booking draft plus a `guided` display flag. It never saves
finder answers or the comfort flag. Optional paid extras remain explicit.
Changing answers aborts/invalidates old results; changing racket unmounts the
finder. Pending ambiguous order submissions remain locked with their original
quote/key. Sign-in/reload keeps the selected setup but requires a fresh quote.

## Product sources

Checked 2 November 2026. Product identity, material, available gauge and colour
are sourced; the demo's price and numerical scores are not copied from these
pages. Descriptions are our own. No manufacturer images or endorsements are used.

| Product | Seeded variants | Official source |
| --- | --- | --- |
| HEAD Velocity MLT | 1.25/1.30 mm, Natural and Black | [HEAD](https://www.head.com/en_US/product/velocity-mlt-4) |
| Wilson NXT | 1.30 mm, Natural | [Wilson](https://www.wilson.com/en-gb/products/nxt-16-tennis-string-set-rz94270) |
| Tecnifibre X-One Biphase | 1.24/1.30 mm, Black | [Tecnifibre](https://www.tecnifibre.com/en/products/garniture-x-one-biphase-black) |
| Tecnifibre Multifeel | 1.25/1.30/1.35 mm, Natural | [Tecnifibre](https://b2b.tecnifibre.com/en/p/02GMU135XN.html) |
| Solinco Hyper-G | 1.15/1.20/1.25/1.30 mm, Green | [Solinco](https://solincosports.com/equipment/hyper-g/) |
| Solinco Hyper-G Soft | 1.20/1.25/1.30 mm, Green | [Solinco](https://shop.solincosports.com/products/hyper-g-soft) |
| Solinco Tour Bite | 1.20/1.25/1.30 mm, Silver | [Solinco](https://solincosports.com/equipment/tour-bite/) |
| Yonex BG65 | 0.70 mm, White/Black | [Yonex](https://www.yonex.com/bg65) |
| Yonex BG80 | 0.68 mm, Yellow | [Yonex](https://www.yonex.com/bg80) |
| Tecnifibre 305 Squash | 1.10/1.20 mm, Green | [Tecnifibre](https://b2b.tecnifibre.com/en/p/06R305G.html) |
| Luxilon ALU Power | 1.25 mm, Silver | [Wilson/Luxilon](https://au.wilson.com/products/luxilon-alu-power-string-set) |

The four verified **preview rackets remain fictional test models**. Their limits
must not be used as instructions for a real racket. Production technical data
still needs sourced racket-specific verification and stringer review.

## Pounds and knot choices

Booking offers **Tension unit → Pounds (lb)**. One pound is exactly 0.45359237 kg.
API/draft tensions remain integer centi-kg. Pound input rounds to the nearest
supported shared increment only within the allowed range, with an explicit
**Applied** kg/lb readout; it never silently clamps an out-of-range input.
Changing display units preserves the physical setting and existing quote. The
unit preference survives draft restoration, while a restored setup still needs
a fresh quote. Recommendation cards show both kg and lb.

Two- and four-knot methods are both available when the service **and** verified
racket permit them. A four-knot-only model now explains why two knots is absent.
Hybrids always use four and hide the knot selector. Setting a surcharge in the
stringer portal does not itself enable a capability: use its checkbox. This
investigation did not reproduce missing two-knot choices for a compatible full
bed; regression tests cover both choices, quotations and four-only restrictions.
