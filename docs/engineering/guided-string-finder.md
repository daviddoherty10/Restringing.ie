# Guided string setups and real-product previews

The finder asks about experience, frequency and a plain-language goal, with an
optional softer-feel preference. It returns complete **editable starting setups**
from the selected stringer's available offerings. Customers do not need to choose
a knot pattern, pre-stretch percentage or mains/crosses tension difference.

The goal choices are illustrated radio cards, with short descriptions and a
brief drawing animation when selected. Recommendation cards show a racket diagram
explaining vertical mains and horizontal crosses, including which product goes
in each direction. Hybrid crosses use a dashed line as well as a different colour.
These are explanatory sketches, not product colours or performance measurements.
Keyboard selection works with the arrow keys; reduced-motion mode keeps the
illustrations static. Loading and completion messages are announced to screen readers.

## Try the real-string preview

From the repository root, run (or rerun):

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
choose **Tennis**, then **Help me choose a complete setup**. No racket
catalogue selection is required. Add the brand/model in the optional racket-details
field if you know it. This business uses only the real-product catalogue, so old fictional
strings do not crowd its results. A public recommendation needs no login.
For a final quote/order, use `bash scripts/preview.sh session customer` as usual.
The existing stringer/staff preview accounts can also manage the new demo shop.

The seed offers 21 real tennis variants at every tennis preview stringer, plus
three Yonex badminton variants and two Tecnifibre squash variants where those
sports are offered. The separate Real String Demo business remains available.
There are 11 active real products / 26 active real variants; the 40 original
fictional catalogue records are archived rather than deleted. Their stock and
immutable historical order snapshots remain intact. Prices are illustrative EUR material prices;
labour is €18. Ratings are explicitly editorial 1 to 5 fixtures, not manufacturer
measurements, clinical evidence or promises of performance. Existing real offerings, prices, availability, business edits, stock balances
and order history are preserved. A separate once-only extension retires only
unchanged seed-owned fictional product records and adds the new offerings.

The catalogue extension runs once on marked preview databases. Existing
`dataset=…fictional-preview, version=1` installations are supported: no reset or
production migration is needed. The existing loopback/database/metadata guards,
advisory lock and transaction still apply. After installation, reseeding never
resets edits or reactivates products/offers. The extension does not run in the
production HTTP server or against normal `DATABASE_URL`.

## Complete-setup rules

1. Rank using the existing weighted catalogue attributes, new-player comfort
   emphasis, advanced-play control, optional comfort
   emphasis and current-material context. Explain the score under a disclosure. Tennis players
   can use their league class in place of the general experience choices.
   Classes 1–3 use competitive guidance and Classes 4–8 use club guidance.
2. For new tennis players, prefer synthetic gut when stocked. New players
   and players who check the softer-feel preference receive full beds from
   the softer-material categories.
3. A spin goal can suggest a polyester mains hybrid for regular or competitive
   players. For club and advanced adult tennis players choosing Balanced,
   lead with polyester mains and synthetic gut crosses when both can be
   booked. If the hybrid is unavailable, compare eligible full beds.
   Older API clients supplying current polyester for a comfort goal can keep that model in the
   mains with synthetic gut crosses, and a full bed of higher-comfort polyester.
   A control goal with current multifilament can suggest keeping that model in
   the mains with polyester crosses. A durability goal can prioritise thicker
   gauges of the same multifilament and round polyester when changing from a
   shaped polyester.
   For tennis players under 13, synthetic gut comes first and multifilament
   is the fallback. For competitive players aged 13–17 who regularly break
   synthetic gut and pick Balanced, the finder can
   offer round polyester mains (catalogue comfort 4 or 5) with synthetic gut
   crosses. Unknown shape does not qualify. Junior recommendations leave
   tension for the stringer to confirm when appropriate. Competitive teenagers
   choosing Spin who already use a full bed of round polyester can receive
   a spin-friendly full bed. A known current product requires a higher
   editorial comfort rating. With only the construction known, require
   comfort 4 or 5.
4. New bookings use a sport and the stringer's per-sport service limits, rather
   than requiring a model catalogue. **Help me decide** is the default tension
   preference for manual bookings. The finder honours a valid known tension
   in the original units or a request for stringer help. It otherwise suggests
   48 lb for tennis players confirmed to be 15 or older when supported.
   A lower, middle or higher tension guide also leaves tension pending and
   records where the customer would like the stringer to aim within the
   racket's suitable range. Junior Spin replacements leave an unknown
   tension pending. A customer who knows their
   preferred tension can enter it within the service's range and increments.
   These service limits do not assert manufacturer approval for a particular frame.
   Older model-based requests still intersect verified racket and service limits.
5. Choose the cheaper supported knot pattern (two on a tie); a hybrid always
   requires four. Leave pre-stretch, stencil, express and grip off. Do not invent
   machine pre-stretch limits from factory string treatment.
6. Validate each proposal through the same server pricing/technical rules as a
   booking. Return an estimated total and breakdown, including labour/method
   charges. Offer distinct full-bed product alternatives, preferring a
   representative tennis gauge (1.30 mm soft material / 1.25 mm polyester).
   The durability checkbox uses the thickest stocked gauge of each product,
   including individual hybrid components. Lower material price and UUID
   break remaining ties. Cross-string ranking puts comfort and power first.

Full-bed and hybrid candidate queries use their corresponding tracking modes.
Hybrid suggestions check combined allocations; no recommendation reserves stock.
A fresh authenticated quote and normal transactional order remain required.
Current price, service, catalogue, quota and stock checks run again there.
No client amount is accepted. The stringer checks frame condition, tension and stringing pattern before work.

`POST /api/v1/stringers/{id}/recommendations` accepts `sport` instead of a
required `racket_model_id`. The optional model ID remains supported for older clients. Each result
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

Choose **Pounds (lb)** before typing tension. Pound entries are stored and shown
in pounds to the customer and stringer, with up to two decimal places. The
server uses the exact conversion to check service and racket limits. Kilogram
entries still use the service's kilogram increments. Switching units clears the
entries so the customer can enter new values and get a fresh quote.

Name a racket in the booking form to reuse its most recent non-cancelled order
setup on a future booking. Choose a distinct label for each racket. Reusing a
setup checks the current stringer's offerings and requires a fresh quote.

Two- and four-knot methods are both available when the service **and** verified
racket permit them. A four-knot-only model now explains why two knots is absent.
Hybrids always use four and hide the knot selector. Setting a surcharge in the
stringer portal does not itself enable a capability: use its checkbox. Regression tests cover both knot choices for compatible full beds, quotations and four-only restrictions.

## Sport-based bookings

Quotes accept `sport`, optional `racket_details` (up to 160 characters) and
`tension_pending`. When tension is pending, both tension amounts must be zero;
they are not stringing instructions. Customer and management order views display
that the tension needs agreement. With an explicit tension preference, normal
service bounds, increments and difference checks apply. Supplying a catalogue ID
continues to apply its verified limits; a supplied sport must match it.

Racket details and the unresolved-tension preference are saved in the normal draft,
quote and immutable order configuration. Finder answers remain transient. Existing
pending submissions retain their original quote and idempotency key.

Full-bed stock accounting reserves a standard allowance of 12 m for tennis,
10 m for badminton and 11 m for squash. These are inventory allowances, not
model-specific specifications. Hybrid stock continues to reserve one half-racket
portion for each component. Exact-length model-based quotes keep their existing
reservation rules. No database migration or racket-catalogue reseeding is needed.
