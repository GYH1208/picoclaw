---
name: ticket-monitor
description: Build, adapt, debug, and operate ticket monitoring workflows for train tickets and other ticketing scenarios such as concerts, sports events, exhibitions, and platform-based ticket drops. Use when the user wants to monitor ticket availability, poll a booking/query page, detect inventory changes, deduplicate alerts, capture evidence with screenshots/snapshots, or convert an existing one-off crawler into a reusable monitoring workflow. Especially useful for browser-driven sites where normal form fill may fail and DOM/JS injection, OCR fallback, or scheduler-based notification delivery may be needed.
---

# Ticket Monitor

Create or adapt a ticket monitoring workflow as a pipeline:

1. Define target
2. Acquire data
3. Parse availability
4. Decide alert condition
5. Deduplicate
6. Notify
7. Run continuously with single-instance protection

Keep the implementation modular. Separate:

- `crawler`: one query run, returns structured JSON
- `monitor loop`: repeats crawler, deduplicates, triggers notification
- `startup wrapper`: enforces single-instance execution
- `artifacts`: logs, screenshot, snapshot, parsed JSON, state

## Recommended output contract

Make the crawler return JSON like:

```json
{
  "status": "has_tickets | no_tickets | error",
  "message": "human readable summary",
  "screenshot": "/abs/path/result.png",
  "snapshot": "/abs/path/snapshot.txt",
  "result_json": "/abs/path/last_result.json",
  "details": {
    "summary": "parsed summary",
    "available_count": 0,
    "available_items": []
  }
}
```

Use this contract so the monitor loop stays generic.

## Core workflow

### 1) Build a one-shot crawler first

Do not start with a forever loop. First make a single run work reliably.

Preferred acquisition order:

1. Official/open API if stable and allowed
2. Browser DOM extraction
3. Browser-side JS injection to set real fields and trigger query
4. OCR fallback from screenshot only when DOM data is incomplete

For browser-driven sites, use `agent-browser` skill if needed. If normal click/fill does not work, inspect real hidden fields and inject values with JS, then dispatch `input`/`change` and click the true query button.

### 2) Save artifacts every run

Persist at least:

- screenshot
- page snapshot/text dump
- structured parsed JSON
- crawler log

These artifacts are essential for debugging fragile ticketing pages.

### 3) Parse into normalized availability

Normalize site-specific data into a compact schema:

```json
{
  "item_id": "train no / event session id / sku id",
  "title": "display title",
  "segment": "route or event session",
  "date": "YYYY-MM-DD or source text",
  "time": "HH:MM or source text",
  "price": "optional",
  "inventory_text": "raw text",
  "is_available": true
}
```

For trains, `item_id` can be train number. For concerts, it can be event session + price tier. For ticketing platforms, it can be SKU/spec ID.

### 4) Deduplicate alerts

Hash only the meaningful available set, not the whole page. Example fields:

- item_id
- date/time/session
- inventory_text
- price tier

Store the last signature in a state file. If unchanged, skip notification.

### 5) Notify through a reliable channel

Prefer the actually verified delivery path in the environment. If direct CLI send is unreliable, write pending content to a file and let scheduler or another trusted delivery path forward it.

### 6) Run continuously

Use a wrapper script with pidfile protection. Avoid multiple monitor instances competing for the same browser or files.

## Single-instance rule

Always provide a wrapper like `run_monitor_forever.sh` that:

- writes a pidfile
- checks whether the old pid is alive
- exits if already running
- cleans up pidfile on exit

Do not recommend launching the monitor loop directly if the workflow is meant to stay resident.

## Browser automation heuristics

When a ticket site is JS-heavy:

- open the target page fresh
- wait for `networkidle`
- inspect actual input/hidden fields
- set both visible text and hidden code/value fields when applicable
- dispatch events after setting values
- click the real submit/query button
- wait again, then probe the DOM with JS

If snapshot lacks result rows but the page visually changes, use:

- browser `eval` to inspect `document.querySelectorAll(...)`
- screenshot for evidence
- OCR only as fallback

## Notification heuristics

Alert only on meaningful positive conditions, for example:

- train: first class / second class available
- concert: target date/session has purchasable price tier
- sports: target section has seats available
- platform drop: add-to-cart / buy-now becomes enabled

Include concise evidence in the message:

- what became available
- when checked
- key attributes
- screenshot path if local debugging is needed

## Adapting beyond train tickets

This pattern is reusable for concerts and other ticketing, but the parsing layer must change.

Reusable parts:

- one-shot crawler architecture
- JS injection strategy
- artifact capture
- deduplication by signature
- notification pipeline
- single-instance loop

Parts that usually need rewriting per site:

- login/session handling
- anti-bot and CAPTCHA handling
- selectors and hidden fields
- result parsing logic
- availability criteria
- SKU/session/price-tier normalization

Read `references/adaptation-guide.md` before adapting to concerts or other platforms.
Read `references/train-12306-pattern.md` for the concrete 12306 implementation pattern.
Read `references/template-usage.md` when you want to bootstrap a new monitor from the generic templates.
Read `references/adapter-config-example.json` for the config shape.

## Bundled templates

Use these bundled files as the default scaffold:

- `scripts/generic_monitor_loop.py`: generic polling + dedup + notify loop
- `scripts/site_adapter_example.py`: example one-shot crawler adapter returning the standard JSON contract
- `scripts/run_monitor_forever.sh`: pidfile-based single-instance wrapper

When creating a new site monitor, prefer copying the example adapter into a project directory and changing only the site-specific acquisition/parsing logic.
