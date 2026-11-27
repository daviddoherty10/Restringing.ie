# Restringing.ie business proposal

Updated 24 November 2026. This is a pre-launch planning pack for commercial partners, clubs, investors and enterprise advisers.

| Deliverable | File |
| --- | --- |
| Full proposal including locker costs and bibliography | [PDF](business-proposal.pdf) · [Editable Markdown](business-proposal.md) |
| Standalone executive summary, two pages | [PDF](executive-summary.pdf) · [Editable Markdown](executive-summary.md) |
| Editable five-year model with 60 monthly periods | [Excel workbook](financial-model/financial-model.xlsx) |
| Prioritised delivery plan | [Implementation roadmap](implementation-roadmap.md) |
| Research references and gaps | [Sources](sources.md) |
| Model inputs | [Assumptions JSON](financial-model/assumptions.json) |

The proposal adds a pre-launch four-bay locker trial and proposes a €24,000 first-year funding ceiling. Appendix D includes the itemised cash analysis and overlays it on the five-year software baseline. The base forecast does not break even within five years. It distinguishes software present in source from planned products and unverified deployment readiness. Customer bookings are cash to the stringer; current Stripe code handles business subscriptions. The implemented free tier counts accepted bookings, whereas the original handoff proposed completed jobs.

All commercial forecasts and procurement allowances are estimates. No actual revenue, customers, funding, founder legal identity or signed partnership is claimed. Legal, tax and insurance questions require professional review. The revised locker estimates are planning allowances pending written quotes. The numerical contents of Tennis Ireland's large 2025 annual-report PDF could not be verified; the pack explains the older figures it uses.

## Using the workbook

Select case 1, 2 or 3 in `Assumptions!D4` for conservative, base or optimistic. Summary and Monthly build use that case. Blue numbers on pale yellow are editable inputs. Values retain fractional forecast accounts and paid-month equivalents; displayed counts are rounded.

Case comparison is a labelled snapshot of the published assumptions. It does not recalculate when workbook cells change. Changes made in Excel do not modify the source JSON. To publish a revised pack, reconcile the workbook and JSON, regenerate the saved comparison, then update the Markdown figures and regenerate the PDFs.

The model reserves VAT immediately and does not reclaim input VAT. It includes startup cash and locker capex separately from operating surplus. Negative cash balances show unfunded requirements. No opening cash, grants, loans or investment receipts are invented. Year one excludes founder pay.

## Rebuilding

Use the Codex bundled Node/Python runtime or an environment with `@oai/artifact-tool`, `reportlab`, `pypdf`, Pillow and Poppler's `pdftoppm`. No application dependency or deployment setting is required. The scripts run from any working directory.

```sh
python3 business/financial-model/check-model.py
node business/financial-model/build-model.mjs
python3 business/build-documents.py
```

If artifact-tool is installed outside the repository, set `NODE_PATH` to that runtime's `node_modules` directory. PDF generation uses Arial from macOS `/System/Library/Fonts/Supplemental`; set `RESTRINGING_FONT_DIR` to another directory containing the named Arial font files when needed. Set `RESTRINGING_QA_DIR` to change the temporary workbook-preview destination.

`check-model.py` independently calculates monthly cash flows and annual scenario outputs from the JSON inputs. `build-model.mjs` creates the workbook, checks its scenario outputs against that calculation, tests price changes, zero paid eligibility and zero churn, restores the base case and exports Excel. It also writes `workbook-results.json`; `scenario-results.json` contains the independent annual outputs. Changing JSON without first refreshing the independent results causes the workbook consistency check to fail.

`build-documents.py` renders the current Markdown and rebuilds the revenue chart from the independent results. It does not rewrite financial figures in prose. Update those figures and the page-number contents table when assumptions or layout change. Review rendered pages after rebuilding.

## Validation performed

- Inspected repository main at `5ec1eb10d4f60601bbb39c52a9fc17f7edb8078d` and relevant local source/guides; excluded uncommitted application changes from released-capability claims.
- Researched current market, competitor, funding and regulatory sources, recording retrieval limits and data-definition differences.
- Matched workbook scenario outputs to an independent monthly model and checked representative input changes and boundaries.
- Scanned spreadsheet formula errors and reviewed all four worksheet types visually.
- Rendered all proposal and executive-summary pages, checked tables and page breaks, and confirmed the summary is two pages.
- Reviewed all 17 proposal sections and appendices using the humanizer skill; no section was omitted.

Native Microsoft Excel recalculation, live production flows, supplier quotations and legal opinions were not checked. Application code and production configurations were not changed.
