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

Adult tennis players choose one of three playing levels and one goal. The preferred constructions are:

| Goal | Class 1–4 | Class 5–7 | Beginner |
| --- | --- | --- | --- |
| Balanced | Hybrid | Hybrid | Hybrid |
| Comfort | Soft polyester mains hybrid | Multifilament | Synthetic gut |
| Easy depth | Power-focused polyester mains / synthetic gut crosses | Multifilament | Synthetic gut |
| Spin | Full bed of shaped polyester | Full bed of shaped polyester | Hybrid |

Hybrid crosses favour stocked synthetic gut, then multifilament. The softer-feel checkbox excludes polyester throughout. When the preferred hybrid cannot be booked, the finder explains the soft full-bed fallback. Migration 26 converts nylon to synthetic gut and retires packaged hybrids while preserving inventory and order references. Product shape is stored on the catalogue product and shared by its gauges and colours. Spin prefers a confirmed shaped polyester. When only round or unclassified polyester is stocked, the result identifies it as an alternative. A hybrid combines two individually offered strings.

Players aged 15–17 use the same recommendations as adults.

| Goal | 12–14, club or advanced | 12–14, beginner | 11 and under |
| --- | --- | --- | --- |
| Balanced | Round polyester mains / synthetic gut crosses | Multifilament | Synthetic gut |
| Comfort | Synthetic gut | Synthetic gut | Synthetic gut |
| Easy depth | Multifilament | Multifilament | Synthetic gut |
| Spin | Round polyester mains / synthetic gut crosses | Multifilament | Multifilament |

For players aged 14 and under, the softer-feel checkbox prioritises synthetic gut. The 12–14 hybrid uses round polyester with a catalogue comfort rating of at least four. If the hybrid is unavailable, offer a softer full bed. When the preferred full-bed material is unavailable, try the other of synthetic gut and multifilament. The optional thicker-gauge preference still applies.

Tension belongs under Stringing instructions. Lower, middle and higher are qualitative guides. Estimate asks the stringer to assess the current tension on arrival. Each travels to the order with tension pending. Customers who know the actual tension can still enter it in kg or lb.

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
