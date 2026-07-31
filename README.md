# Ledger — a personal expense tracker

A standalone full-stack app for tracking income and expenses: Go backend,
React frontend. Built as a learning project for Go, with the frontend
serving as the client instead of Go's templates.

## Stack

- **Backend:** Go 1.22, standard library only for everything except
  storage. Data persists to local JSON files by default — no database
  server required to get started. Postgres is supported as an opt-in swap
  (see "Using Postgres" below) via a single driver dependency
  (`jackc/pgx`), still with no ORM.
- **Frontend:** React + Vite, charts via Recharts.

## Why no database or web framework, by default?

This is deliberate, and worth understanding if you're learning Go from a
Django background:

- **No ORM** — by default the whole "database" is a `map[string]*Transaction`
  in memory, guarded by a `sync.RWMutex`, flushed to `data.json` on every
  write. This is the single most useful thing to study in this repo if you're
  coming from Django: compare `store.go` to how DRF/the ORM handles the same
  concerns (thread-safety, persistence, lookups) and you'll see what an ORM is
  actually doing for you under the hood.
- **No router framework** (no gin/chi/mux) — routing is done with Go 1.22's
  built-in `http.ServeMux`. Good enough for a handful of routes, and you get
  to see exactly how a request maps to a handler function with zero magic.

The `TransactionStore`/`CategoryStore`/`BudgetStore`/`RecurringStore`
interfaces (`stores.go`) are what make it possible to swap the storage
backend without touching a single handler — `handlers.go` only ever talks
to the interface, never to `store.go` or `pg_store.go` directly.

## Using Postgres instead of JSON files

Set the `DATABASE_URL` environment variable before starting the backend:

```bash
export DATABASE_URL="postgres://user:password@localhost:5432/expensetracker?sslmode=disable"
cd backend
go run .
```

What happens:

- If `DATABASE_URL` is unset (the default), nothing changes — you get the
  original `data.json`/`categories.json`/etc. behavior.
- If it's set, the backend connects to that Postgres instance, creates the
  `transactions`, `categories`, `budgets`, and `recurring_rules` tables on
  startup if they don't already exist (`schema.go` — plain `CREATE TABLE IF
  NOT EXISTS`, no migration framework), and uses Postgres for all of that
  data instead.
- **The PIN and session store (`pin.json`/`sessions.json`) always stay as
  local JSON files, regardless of `DATABASE_URL`.** That's a deliberate
  choice, not an oversight — auth is a separate concern from the app's
  business data, small enough that a database doesn't buy you anything, and
  keeping it host-local matches the app's existing "casual local-network
  access" threat model described in the PIN lock section below.
- One-time setup: `cd backend && go get github.com/jackc/pgx/v5/stdlib &&
  go mod tidy` to pull down the driver and populate `go.sum` (not
  pre-vendored in this repo).

If you're moving data you already have in `data.json` etc. over to
Postgres, there's no automatic import — write your own one-off script, or
ask for help with one.

## Running it

### 1. Backend

```bash
cd backend
go run .
```

Starts the API on `http://localhost:8080`. On first run (JSON mode) it
creates `data.json` in the `backend/` folder — that file *is* your
database, so back it up like you would any other data file. In Postgres
mode (`DATABASE_URL` set), your Postgres instance is the database instead.

### 2. Frontend

In a second terminal:

```bash
cd frontend
npm install
npm run dev
```

Opens the app on `http://localhost:5173`. It talks to the API via a
relative `/api` path (`BASE_URL` in `frontend/src/api.js`) — during `npm
run dev`, Vite's dev-server proxy (`vite.config.js`) forwards those
requests to the Go backend on port 8080. This also still works from your
phone over LAN (`host: true` in the Vite config exposes it on your
network IP, e.g. `http://192.168.1.42:5173`) without any extra setup.

## Running it as a single server (for tunneling / sharing a link)

For sharing the app over the internet — e.g. with `cloudflared` or
`ngrok` — run it as **one process on one port** instead of two:

```bash
# 1. Build the frontend once (repeat this after any frontend code change)
cd frontend
npm install
npm run build
cd ..

# 2. Start the backend — it now also serves the built frontend
cd backend
go run .
```

Look for `serving frontend from ../frontend/dist` in the startup log —
that confirms it found the build. If you see `no frontend build found`
instead, the `npm run build` step above didn't run or didn't finish.

The whole app (UI + API) is now on `http://localhost:8080`. In a
**separate terminal, left running alongside it**:

```bash
cloudflared tunnel --url http://localhost:8080
```

(Install with `sudo apt-get install cloudflared` or the `.deb` from
Cloudflare's GitHub releases if you don't have it.) The very first thing
it prints is a boxed `https://....trycloudflare.com` URL — scroll up if
you miss it in the rest of the log. That link works from any device,
anywhere, for as long as **both** the `go run .` tab and the
`cloudflared tunnel` tab stay running on your machine — closing either
one, sleeping the laptop, etc. kills the link immediately.

Restarting `cloudflared tunnel` generates a **new random URL** every
time — if you've shared a link before and restart the tunnel, the old
link stops working and you need to grab and resend the new one.

`FRONTEND_DIST` env var overrides where the backend looks for the
built frontend, if you ever move `dist/` somewhere other than the
default `../frontend/dist`.

## API reference

| Method | Path                     | Description                          |
|--------|--------------------------|---------------------------------------|
| GET    | `/api/transactions`      | List all transactions                |
| POST   | `/api/transactions`      | Create a transaction                 |
| GET    | `/api/transactions/{id}` | Get one transaction                  |
| PUT    | `/api/transactions/{id}` | Replace a transaction (full object)  |
| PATCH  | `/api/transactions/{id}` | Partially update a transaction       |
| DELETE | `/api/transactions/{id}` | Delete a transaction                 |
| GET    | `/api/reports`           | Category + monthly aggregates        |
| GET    | `/api/categories`        | List categories                      |
| POST   | `/api/categories`        | Create a category                    |
| DELETE | `/api/categories/{id}`   | Delete a category                    |
| GET    | `/api/budgets`           | List budgets                         |
| POST   | `/api/budgets`           | Create a budget                      |
| PUT    | `/api/budgets/{id}`      | Update a budget (category/limit)     |
| DELETE | `/api/budgets/{id}`      | Delete a budget                      |
| GET    | `/api/budgets/status`    | Spend vs. limit for a month (`?month=2026-05`, defaults to current) |
| GET    | `/api/budgets/history`   | Spend vs. limit trend across recent months (`?months=`, default 6, max 24) |
| GET    | `/api/recurring`         | List recurring rules                 |
| POST   | `/api/recurring`         | Create a recurring rule              |
| DELETE | `/api/recurring/{id}`    | Delete a recurring rule              |
| GET    | `/api/goals`             | List savings goals                   |
| POST   | `/api/goals`             | Create a savings goal                |
| PUT    | `/api/goals/{id}`        | Edit a goal's name/target/date       |
| DELETE | `/api/goals/{id}`        | Delete a goal                        |
| POST   | `/api/goals/{id}/contribute` | Add (or subtract) from a goal's saved amount |
| GET    | `/api/export/csv`        | Download all transactions as CSV     |
| GET    | `/api/export/xlsx`       | Download a multi-sheet Excel workbook (summary, transactions, by category, by month) |
| GET    | `/api/export/html`       | Download a self-contained, offline-capable HTML dashboard (charts + filterable ledger) |
| POST   | `/api/import/csv`        | Import transactions from a CSV file  |
| POST   | `/api/import/file`       | Import transactions from a CSV, Excel (.xlsx), or PDF file |
| GET    | `/api/auth/status`       | Whether a PIN has been set up (public)|
| POST   | `/api/auth/setup`        | Set the PIN for the first time       |
| POST   | `/api/auth/login`        | Log in with either PIN, returns a token + role (`owner`/`guest`) |
| POST   | `/api/auth/logout`       | Invalidate the current session token |
| POST   | `/api/auth/change-pin`   | Change the owner PIN (requires current one) |
| POST   | `/api/auth/guest-pin`    | Set or rotate the guest PIN (requires current owner PIN) |
| DELETE | `/api/auth/guest-pin`    | Turn off guest access (requires current owner PIN) |
| GET    | `/health`                | Health check                         |

Transaction shape:

```json
{
  "type": "expense",       // or "income"
  "amount": 5000,
  "category": "Food",
  "description": "Groceries",
  "date": "2026-07-01"
}
```

Recurring rule shape:

```json
{
  "type": "expense",
  "amount": 15000,
  "category": "Rent",
  "description": "Monthly rent",
  "frequency": "monthly",  // or "weekly"
  "start_date": "2026-07-01"
}
```

Goal shape:

```json
{
  "name": "Emergency fund",
  "target_amount": 500000,
  "saved_amount": 50000,      // read-only from the client's perspective — see below
  "target_date": "2026-12-31" // optional
}
```

Goals track progress toward saving something — separate from budgets, which
cap spending. `saved_amount` only changes via
`POST /api/goals/{id}/contribute` with `{"amount": 5000}` (or a negative
number to correct an over-contribution; the request is rejected with 400 if
it would push `saved_amount` below 0). Editing a goal via `PUT` carries the
existing `saved_amount` forward unless the request body explicitly sets a
non-zero one — so renaming a goal or changing its target doesn't reset
progress.

**Note on Postgres:** goals are always stored in a local `goals.json`,
regardless of `DATABASE_URL` — same reasoning as the PIN/session store
(small, not part of the app's core ledger data). Worth revisiting if this
ever needs multi-instance support.

### Exporting your data

Three export formats, all served from the same underlying data as
`GET /api/reports`:

- **`GET /api/export/csv`** — flat CSV, one row per transaction. This is
  also the format `POST /api/import/csv` expects back, so round-tripping
  an exported file works out of the box.
- **`GET /api/export/xlsx`** — a four-sheet Excel workbook: `Summary`
  (totals + net), `Transactions` (the full ledger), `By Category`, and
  `By Month`. Built by a small stdlib-only OOXML writer (`xlsx.go`) — no
  third-party dependency, unlike the *importer's* Excel support below.
- **`GET /api/export/html`** — a single self-contained HTML file
  (`export_html.go`) with the transaction data embedded inline as JSON:
  charts, search/filter/sort on the ledger table, all running in vanilla
  JS with zero external requests, so it keeps working fully offline once
  downloaded (handy for archiving a snapshot or sharing a read-only view).

All three are wired up behind the "Export transactions" dropdown in the
Manage tab (`Manage.jsx`) and, like every other route, sit behind the PIN
lock once one is set up.

### CSV / Excel / PDF import

`POST /api/import/file` is what the app's "Import transactions" button
uses — it accepts a `multipart/form-data` upload under a `file` field and
picks a parser from the filename's extension (`.csv`, `.xlsx`/`.xls`, or
`.pdf`, falling back to the upload's `Content-Type` if the extension is
missing).

`POST /api/import/csv` still exists as a CSV-only endpoint (also accepts a
raw CSV body, no multipart required) for anyone scripting an upload
directly.

**CSV and Excel** are parsed exactly, not heuristically: both expect a
header row and match columns by name (case-insensitive), so a reordered
export from a spreadsheet still works. Required columns: `date`, `type`,
`category`, `amount`. Optional: `description` — this is the same shape
`GET /api/export/csv` produces, so round-tripping an exported file back in
works out of the box. For Excel, only the first sheet is read.

Rows that fail to parse (bad amount, unrecognized type, missing required
field) are skipped rather than aborting the whole import — the response
reports how many rows imported vs. were skipped, plus up to 20 example
error messages:

```json
{ "imported": 42, "skipped": 2, "errors": ["row 5: invalid amount \"n/a\"", "row 11: type must be 'income' or 'expense'"] }
```

**PDF is best-effort**, by nature: a PDF has no real columns once text is
extracted, so `handlers_import.go` looks for lines shaped like `<date>
<description> <amount>` (e.g. a line from a bank or card statement),
infers income vs. expense from the amount's sign/parentheses or a few
keywords ("deposit", "refund", ...), and files everything else under an
`Uncategorized` category for you to re-sort afterward. The response
carries an extra `note` field in this case, which the frontend surfaces as
a toast prompting a quick review:

```json
{ "imported": 18, "skipped": 1, "errors": ["line 24: could not save"], "note": "PDF import reads text heuristically..." }
```

Scanned/image-only PDFs (no extractable text layer) aren't supported —
run them through OCR first if you need those.

**One-time setup:** the Excel and PDF parsers aren't in `go.mod` yet the
way `pgx` is pre-added — from `backend/`, run:

```bash
go get github.com/xuri/excelize/v2 github.com/ledongthuc/pdf
go mod tidy
```

That pulls both libraries and populates `go.sum` (same reason it's not
pre-vendored as the Postgres driver above — keeps the repo you actually
pulled small).

Categories are stored in `categories.json` and seeded with sensible
defaults on first run — edit or add to them from the Manage tab in the app.
Deleting a category doesn't touch transactions that already used it; they
keep their category name as plain text.

Recurring rules are checked on server startup and every hour while it stays
running, so entries generate even if the app was closed when they were due
— it just catches up next time it opens.

## PIN lock

The app can be locked with a 4-8 digit PIN — set one up the first time you
open it, and every device that connects (your laptop, your phone) will need
it. Sessions last 30 days once unlocked, stored in `sessions.json`; the PIN
itself is never stored in plain text, only a salted SHA-256 hash in
`pin.json`.

**Be clear-eyed about the threat model this protects against.** It stops
someone on your Wi-Fi from casually opening the app's URL and seeing your
finances — that's it. It does **not** protect against:
- A determined attacker on the same network, since there's no HTTPS here —
  the PIN and session token travel in plain text over your local network.
- Someone with access to your laptop's filesystem — `pin.json` and
  `sessions.json` are readable by anyone with disk access, same as
  `data.json`.

If you outgrow this threat model (e.g. you deploy this somewhere beyond
your home network), the right next step is proper HTTPS via a reverse
proxy, not more PIN-lock engineering.

### Guest access

From the Manage tab, the owner can set up a second PIN for guests — handy
if you're sharing a tunnel link (see "Sharing over a tunnel" above) with
someone you don't want to hand full read/write access to. A guest session
can view the transaction ledger, reports, budgets/budget status, recurring
rules, and goals, but can't create, edit, or delete anything; can't
contribute to goals; can't import or export; and can't change either PIN.
Attempting a blocked action returns a 403 (distinct from the 401 an
expired session gets), and the frontend shows a small "View only" badge
for guest sessions.

Turning guest access off (also from Manage) immediately invalidates any
guest session currently in use, rather than waiting out its 30-day expiry.

## Project structure

```
backend/
  main.go        entrypoint, route wiring
  handlers.go    HTTP handlers (the "views" in Django terms)
  handlers_export.go export endpoints (CSV/XLSX/HTML)
  xlsx.go        stdlib-only .xlsx (OOXML) writer, used by the export endpoint
  export_html.go self-contained HTML dashboard builder, used by the export endpoint
  store.go       in-memory + JSON-file persistence, mutex-protected
  reports.go     aggregation logic for the reports endpoint
  models.go      data structs (the "models"/"serializers" in Django terms)
  middleware.go  CORS + request logging
  id.go          tiny stdlib-only random ID generator

frontend/
  src/
    api.js               fetch wrappers for the Go API
    format.js            currency/date formatting helpers
    useNotifications.js  toast + browser push notification system
    App.jsx              top-level state + layout
    App.css              ledger-themed design system
    components/
      BalanceHeader.jsx  running balance display
      TransactionForm.jsx add income/expense form
      TransactionList.jsx receipt-style transaction list
      Reports.jsx        category + monthly charts
      ToastStack.jsx     in-app toast notifications
      NotificationBell.jsx browser push permission toggle
      GoalsManager.jsx   savings goal tracking + contributions
```

## Notifications

The app has a small notification system covering both in-app toasts (always
on) and real browser push notifications (opt-in, click the bell icon in the
header):

- **Action feedback** — every create/update/delete (transactions,
  categories, budgets, recurring rules) shows a success or error toast.
- **Budget alerts** — a toast (and push, if enabled) fires the moment a
  budget crosses 80% used ("approaching limit") or 100% used ("exceeded"),
  not on every refresh — see `budgetTier`/`checkBudgetAlerts` in `App.jsx`.
- **Recurring transactions** — when a recurring rule's `next_due` advances
  (meaning the backend generated a new occurrence), a toast announces it.

Browser push notifications only fire when the tab is in the background
(`document.hidden`) and only after the user grants permission — while
you're looking at the app, the in-app toast alone is enough. Permission
state and dismiss timers live in `useNotifications.js`; there's no server
push infrastructure involved, this is the plain client-side `Notification`
API, so it only works while the app's tab/window still exists somewhere
(closing the browser entirely stops it, same limitation as the recurring
rule catch-up logic above).

## Deleting a transaction

Deleting a transaction doesn't hit the API immediately — it disappears from
the ledger right away, but the actual `DELETE` request is deferred for a
few seconds behind an "Undo" button on the toast. Clicking Undo cancels the
pending delete entirely (nothing was ever sent); dismissing the toast with
the × does not cancel it, only Undo does.

This state lives only in memory (`pendingDeleteIds` in `App.jsx`), so
closing the tab or reloading mid-window loses the pending timer — the
transaction just never gets deleted and reappears on next load, the same
outcome as if you'd clicked Undo.

## Budget history

Budgets used to only ever show the current calendar month — there was no
way to see whether you were over budget last month, or which categories
chronically run hot. Two additions cover that:

- **`GET /api/budgets/status?month=2026-05`** — the same spend-vs-limit
  computation the current-month view always used, now for any month.
- **`GET /api/budgets/history?months=6`** — each budget's spend trend
  across the last N calendar months (default 6, max 24), oldest first.

Each budget also keeps an effective-dated history of its `monthly_limit`
(`limit_history`), so both endpoints above compare a given month against
whatever limit was actually in effect *then* — not just today's. Raise a
budget from ₦500 to ₦700 and last month still shows against ₦500; this
month onward shows against ₦700. A budget that's never had its limit
changed carries no history and just uses its current limit throughout,
same as before this existed.

In the app, click a budget's limit figure (Manage tab) to edit it inline,
or the chevron on any budget row to expand a small six-month bar strip
showing the same green/gold/rust tiering as the main progress bar — each
bar's tooltip shows the limit that applied to that month.

## Next steps if you want to keep building

- ~~Swap `data.json` for real Postgres~~ — done, see "Using Postgres
  instead of JSON files" above. SQLite could still be a nice lighter-weight
  middle ground if you want SQL without running a separate server.
- ~~Budget history — see spend vs. limit for past months, not just the
  current one~~ — done, see "Budget history" above.
- ~~Track historical `monthly_limit` values so past months compare
  against the limit that was actually in effect at the time~~ — done,
  see "Budget history" above (`limit_history`).
- Deploy: the Go binary is a single static file (`go build -o app .`) — copy
  it to any small VPS and run it (plus a Postgres instance, if you're using
  that mode).
