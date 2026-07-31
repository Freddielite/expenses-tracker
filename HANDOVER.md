# Handover notes

Not part of the app — just a running log of context for whoever's picking
this up next (including future-you). Kept out of git on purpose since it's
personal working notes, not project documentation (that's what README.md
is for).

## Effective-dated budget limits (this session)

Built the feature the README's "Next steps" flagged: budget history used
to always compare every month against *today's* `monthly_limit`, even
though the limit to catch that could've changed. Now each budget keeps a
`limit_history` log so past months compare against whatever limit was
actually in effect at the time.

**Backend:**
- `models.go` — new `BudgetLimitChange{Month, Limit}` and `Budget.LimitHistory
  []BudgetLimitChange`. A blank `Month` means "since the beginning" — the
  baseline entry seeded the first time a pre-existing budget (one with no
  history yet) gets its limit changed. `BudgetMonthStatus` gained a `Limit`
  field so the history response shows which limit applied to each month,
  not just the current one.
- `handlers_budgets.go` — `recordLimitChange(history, oldLimit, newLimit)`
  appends/collapses an entry into the history on every limit change;
  `limitAtMonth(b, month)` resolves the limit that applied to a given
  month, falling back to the budget's current `MonthlyLimit` when there's
  no applicable entry (covers both months before any recorded change and
  legacy budgets with nil history — so anything that predates this feature
  and has never been edited since behaves exactly as before). POST seeds a
  fresh budget's history with one entry at creation. PUT fetches the
  existing record first (rather than trusting whatever `limit_history` the
  client body might contain) and derives the new history from that.
  `HandleBudgetStatus` and `HandleBudgetHistory` both now resolve the limit
  per-month via `limitAtMonth` instead of reading `b.MonthlyLimit` directly.
- Two edits within the same calendar month collapse into one history entry
  rather than recording a spurious in-month change — verified by hand (see
  Testing below).
- `schema.go` — new `limit_history JSONB NOT NULL DEFAULT '[]'::jsonb`
  column on `budgets`, added via `ALTER TABLE ... IF NOT EXISTS` so it's
  safe on an existing table.
- `pg_item_stores.go` — `PostgresBudgetStore` now marshals/unmarshals
  `LimitHistory` to/from that column on Create/List/Get/Update.

**Frontend:**
- There was previously no UI path to `PUT /api/budgets/{id}` at all (only
  create/delete) — added one, since otherwise this feature would've shipped
  unreachable from the app. `api.js` — new `updateBudget(id, budget)`.
  `App.jsx` — new `handleUpdateBudgetLimit`, threaded down through
  `Manage.jsx` as `onUpdateBudget`.
- `BudgetManager.jsx` — a budget row's limit figure is now click-to-edit
  inline (same interaction pattern as the ledger's inline amount edit:
  click → number input → Enter/blur commits, Escape cancels, guarded
  against a double-submit from a second `blur` once the input disables
  mid-save). Hidden for guests (`readOnly`), same as the remove button.
  The history strip's per-bar tooltip now also shows the limit that
  applied to that month.
- `App.css` — new `.budget-row__limit-editable` / `-input` / `-error`
  rules, built from the same tokens the ledger's amount-edit styling
  already uses (`--gold`, `--gold-soft`, `--line`, `--paper-raised`) —
  no new colors introduced.

**Testing done:**
Installed a real Go toolchain via `apt` and used the same throwaway
stub-module technique as prior sessions for `pgx`/`excelize`/
`ledongthuc/pdf` (this sandbox has no module-proxy access) — `go build
./...` and `go vet ./...` both clean, `go.mod` reverted to exactly what it
was afterward (diffed before restoring). Built an actual binary and drove
it by hand with `curl`:
- A budget created and edited within the same real-time month correctly
  collapses to one history entry rather than two.
- Re-submitting the *same* limit is a no-op — history untouched.
- Simulated a budget that had existed since an earlier month at one limit
  (hand-edited `budgets.json` to backdate its history, since the sandbox
  can't fast-forward real time) — raising its limit correctly left the
  earlier months' `history` entries and their `percent_used` pointing at
  the *old* limit, with only the current month picking up the new one;
  `?month=` on `/budgets/status` for one of those earlier months also
  correctly returned the old limit.
- A budget with `limit_history` deleted entirely (simulating data that
  predates this feature) round-trips fine through both `/budgets` and
  `/budgets/status` with no crash, and its *first* edit under the new code
  correctly seeds the blank-month baseline entry before adding the new one.
- Malformed `?month=`/`?months=` still 400 cleanly — unchanged behavior,
  confirmed not to have regressed.
- Frontend: `npm install` + `oxlint src` (0 warnings/errors) + `vite build`
  (clean) — `node_modules`/`dist` removed again afterward, nothing built
  ships in the zip. Didn't have a real browser to click the new inline
  limit edit — worth a quick manual pass on the blur/Enter/Escape
  interactions specifically, same caveat as every other inline-edit
  feature in this app so far.

## Guest PIN (this session)

Built the feature the last handover entry sketched out. A second PIN,
separate from the owner's, for people you share a tunnel link with.

**Backend:**
- `auth.go`: `pin.json` now holds an optional `guest_salt`/`guest_hash`
  pair alongside the original `salt`/`hash` (owner) fields — old files
  with just the owner fields still load fine, `guest_salt`/`guest_hash`
  come back empty and `guestPin` just stays nil. `sessions.json` entries
  gained an optional `role` field (`"owner"` or `"guest"`); missing/empty
  is normalized to `owner` so pre-existing sessions keep working exactly
  as before. `Login` now checks the owner PIN first, then the guest PIN,
  and returns which role matched. `ValidateSession` returns the role
  alongside the valid/expired bool. New owner-only methods: `SetGuestPIN`
  (create/rotate, requires the current *owner* PIN — there's no
  "already set" restriction like the owner's `SetupPIN`, so it doubles as
  rotation) and `RemoveGuestPIN` (also requires the owner PIN, and kicks
  out any currently-active guest sessions immediately rather than letting
  them ride out their 30-day TTL).
- `middleware.go`: `withAuth` now checks role after confirming the
  session is valid. A guest hitting a guest-blocked route gets **403**
  ("guests don't have permission to do this"), kept deliberately distinct
  from the **401** used for missing/expired sessions — this was the open
  question from the last handover, and 403 won because it lets the
  frontend show "you don't have permission" instead of silently bouncing
  to the lock screen as if the session had died. `guestAllowed(method,
  path)` is deny-by-default: export/import are blocked outright
  (regardless of method — walking away with a full data copy isn't
  read-only in spirit even though the HTTP verb is GET), `/api/auth/*` is
  blocked except `POST /api/auth/logout`, and everything else is
  read-only (GET-only) for guests. A new route only becomes
  guest-visible by being explicitly added here, never by omission from a
  blocklist.
- `handlers_auth.go`: login/setup responses now include `"role"`.
  `/api/auth/status` gained `guest_pin_set` (safe to expose publicly —
  it's just on/off, not the PIN). New `HandleAuthGuestPIN` handles both
  `POST` (set/rotate) and `DELETE` (remove) on `/api/auth/guest-pin`,
  registered in `main.go`. Both require the *current owner PIN* in the
  body, not just a valid owner session — a live unlocked session isn't
  proof of knowing the PIN (e.g. an unattended browser tab), and granting
  guest access is exactly the kind of thing that shouldn't be doable with
  just a stolen/borrowed session token.

**Frontend:**
- `api.js`: token + role are now both stored in localStorage
  (`ledger-auth-role` alongside the existing `ledger-auth-token`).
  `setForbiddenHandler` mirrors the existing `setUnauthorizedHandler` but
  for 403s specifically. New `setGuestPin`/`removeGuestPin` calls.
- `useAuth.js`: exposes `role` and `isGuest`. A token saved before this
  feature existed has no stored role — defaults to `"owner"` on load
  rather than leaving it blank, since every token issued back then was
  necessarily an owner token.
- `App.jsx`: registers the 403 handler to surface a toast ("Not available
  in view-only mode") — this is the backstop for anything not already
  hidden client-side, not the primary defense. `TransactionForm` is
  hidden entirely for guests; `isGuest` threaded down to
  `TransactionList` (as `readOnly`) and `Manage` (same), which forwards
  it to `CategoryManager`/`BudgetManager`/`GoalsManager`/
  `RecurringManager` (hides create forms and delete/contribute buttons,
  keeps the read views) and hides the export/import/scan-screenshot
  section plus `ChangePinSection` entirely for guests.
- New `GuestAccessSection.jsx` (owner-only, sits in `Manage` where
  `ChangePinSection` is): set/rotate the guest PIN, or turn it off with a
  second confirmation step (re-enter the owner PIN — this is the action
  that immediately signs out any active guest).
- `BalanceHeader.jsx`: small "View only" badge next to the "Ledger"
  eyebrow when `isGuest`.

**Testing done:**
Same throwaway-stub-module technique as prior sessions for `pgx`,
`excelize`, `ledongthuc/pdf` — `go build ./...` and `go vet ./...` both
clean, `go.mod`/`go.sum` reverted to exactly what they were afterward
(confirmed with `diff` before restoring). Built and ran an actual binary
(JSON mode) and drove the whole guest-PIN lifecycle by hand with `curl`:
owner setup → wrong-PIN guest-PIN-set rejected → correct-PIN guest PIN
set → guest login → guest can GET transactions/budgets/status (200) →
guest blocked on POST transactions, POST goal contribute, GET export/csv,
POST change-pin, POST guest-pin (all 403 with the guest-specific message)
→ guest can POST logout (200) → reused guest token now 401. Separately
verified `RemoveGuestPIN` immediately invalidates a live guest session
(200 → 401 on the same token) and that the old guest PIN stops
authenticating anything afterward. Also specifically tested backward
compatibility: hand-built an old-format `pin.json` (no guest fields) and
`sessions.json` (no `role` field) from before this feature, pointed a
freshly built binary at them, and confirmed the server starts cleanly,
`guest_pin_set` correctly reads `false`, the pre-existing session token
still authenticates with full owner access, and the original owner PIN
still logs in as `role: "owner"`. Finally, rebuilt the frontend
(`npm install` + `oxlint src` → 0 warnings/errors, same pre-existing
tesseract-bundle-only warnings as before when linting the whole tree +
`vite build` clean) and re-ran the same curl sequence against the actual
built app served off the single Go port, to make sure this feature and
the single-port/tunnel work from last session still cooperate correctly
together.

Didn't have a real browser, so unverified: the actual UI (the "View
only" badge rendering correctly, forms/buttons really disappearing for a
guest session rather than just being logically gated, the
`GuestAccessSection` remove-confirmation flow, and whether hiding the
`TransactionForm` outright vs. showing it disabled reads better in
practice) — worth a manual pass, ideally with two browser sessions (or
one regular + one incognito) logged in as owner and guest side by side.

**One thing noticed but out of scope for this session:** this zip
doesn't contain any `.gitignore` files at all (root, `backend/`, or
`frontend/`) — a prior handover entry describes auditing three of them
in detail, so they existed at some point; either the zip export step
drops dotfiles or they were lost some other way. Recreated all three
from that prior entry's description (see next entry below) rather than
leaving it for later, since it was quick and low-risk.

## .gitignore files recreated (this session, follow-up)

The three `.gitignore` files (root, `backend/`, `frontend/`) were
missing from this zip entirely (see note above) — recreated them from
the prior session's own description of what each should cover, then
verified the same way that prior session did rather than trusting the
description: `git init` + `git add -A`, then touched a stand-in for
every artifact each file claims to ignore — `data.json`/`categories
.json`/`budgets.json`/`recurring.json`/`goals.json`/`pin.json`/
`sessions.json`, `*.log`, all three binary names (`app`/
`expensetracker`/`expense-tracker`), `*.xlsx`/`*.pdf`/
`expense-tracker-report.html`, `node_modules`/`dist`, OS/editor cruft
(`.DS_Store`, `Thumbs.db`, `.vscode/`, `.idea/`, `*.swp`) at both root
and nested depth, `.env`/`.env.*` at multiple depths, and `HANDOVER.md`
itself. `git status --ignored` confirmed every one of them ignored
correctly, `git ls-files` still showed all 76 real project files
tracked and untouched, and `backend/go.sum` (hand-created as a stand-in,
since none exists in this JSON-mode-only environment) came back
untracked-but-addable rather than ignored, same as the prior audit
found. Scratch git repo and every simulated file were removed afterward
— nothing test-related shipped in this zip either.

## Budget history feature (this session)

Went through the whole app to find the next worthwhile feature. Landed on
budget history: `BudgetStatus` (`handlers_budgets.go`) only ever computed
spend-vs-limit for the *current* calendar month — once a month ended,
there was no way to see how you did against a budget in the past, even
though the ledger data to answer that was sitting right there.

**Backend:**
- `HandleBudgetStatus` now accepts an optional `?month=2026-05` query
  param (still defaults to the current month if omitted) instead of
  hardcoding `time.Now()`. 400s on a malformed month.
- New `HandleBudgetHistory` (`GET /api/budgets/history?months=N`, default
  6, clamped to 1-24) returns each budget's spend trend across the last N
  months, oldest first. Refactored the spend-by-category-for-a-month loop
  out into `spendByCategoryForMonth()` so status and history can't drift
  out of sync on what counts as "spend," and the history handler groups
  by month once up front (`spendPerMonth`) rather than re-scanning all
  transactions per budget per month.
- New models: `BudgetMonthStatus` (one month's spend/percent) and
  `BudgetHistory` (a budget + its `Months` slice).
- Deliberate simplification, called out in the README: `monthly_limit`
  isn't tracked historically, so every month in a trend is judged against
  *today's* limit. Flagged as the natural next step if this needs to be
  more precise (an effective-dated list of past limits per budget).
- Registered `/api/budgets/history` in `main.go` — exact-match route sits
  fine alongside the existing `/api/budgets/status` and `/api/budgets/`
  prefix route, same pattern already used for `status`.

**Frontend:**
- `api.js` — added `getBudgetHistory(months = 6)`.
- `BudgetManager.jsx` — each budget row now has a chevron toggle; expanding
  it lazily fetches history (only for the expanded row, only once per
  expand) and renders a small six-bar strip (`BudgetHistoryStrip`/
  `HistoryBar`), one bar per month, height proportional to percent-used
  and capped visually at 100%, colored with the same green/gold/rust
  tiering as the main progress bar. Labels use the existing `formatMonth`
  helper from `format.js`.
- `App.css` — new `.budget-history*` rules and `.budget-row__history-toggle`,
  built entirely from existing tokens (`--jade`/`--gold`/`--rust`/
  `--paper-sunken`/`--ink-soft`) — no new colors introduced.

**Testing done:**
- Installed a real Go toolchain via `apt` (archive.ubuntu.com reachable,
  same as prior sessions) and ran `go build ./...` + `go vet ./...` clean
  across the whole backend, including the new/changed budget files. Used
  the same throwaway stub-module technique as previous sessions for the
  packages this sandbox can't reach (`pgx`, `excelize`, `ledongthuc/pdf`)
  to get a real compile check, then reverted `go.mod` to exactly what it
  was (just the one `pgx` require, no stub replaces) once done.
- Went a step further than a bare compile check: built an actual runnable
  binary against the stubs, ran it for real (JSON-file mode), and exercised
  the new endpoints by hand with `curl` — created a budget and four
  transactions spread across April-July 2026, then verified: current-month
  status reflects only July's spend and correctly shows `percent_used` over
  100 once a category goes over; `?month=2026-05` returns only that month's
  number; `/budgets/history` (default and `?months=3`) returns the right
  months in oldest-first order with correct per-month spend; and both
  `?months=abc` and `?month=nonsense` return clean 400s instead of panicking
  or silently defaulting. Killed the test server and restored `go.mod`
  afterward — nothing from this test run should ship (data.json etc. were
  all created in a scratch directory, not the repo).
- Frontend: `npm install` + `oxlint` (0 errors — same pre-existing warning
  count as before, all inside the vendored Tesseract wasm bundle, nothing
  in the files touched this session) + `vite build` (clean). Didn't have a
  real browser to click the chevron and watch the strip animate in/out —
  worth a quick manual pass on that specifically, plus confirming the
  loading state (`Loading history…`) doesn't flash annoyingly on a fast
  connection.

## Export feature audit + repo hygiene pass

You asked me to check over the export feature (`handlers_export.go`,
`xlsx.go`, `export_html.go`), fix the `.gitignore` files, and update
docs. Here's what actually happened, since "fix the gitignores" turned
out to need more investigation than fixing:

**The three `.gitignore` files were already correct.** I didn't take that
on faith — I ran an actual `git init` + `git add -A` against the repo,
then simulated every runtime artifact each `.gitignore` claims to cover:
`data.json`/`categories.json`/`budgets.json`/`recurring.json`/
`goals.json`/`pin.json`/`sessions.json`, `*.log`, all three possible
compiled-binary names (`app`/`expensetracker`/`expense-tracker` —
confirmed `go build ./...` really does drop a binary named after the
module, `expensetracker`, straight into `backend/` with no `-o` flag),
`node_modules`/`dist`, OS/editor cruft, and `.env*` at every depth. Every
single one was ignored correctly, and `go.sum` (which should be
committed, not ignored) was correctly left untouched. `HANDOVER.md`
itself is also still correctly excluded by the root `.gitignore`, same
as always.
- One genuine gap, given this session's new export feature: nothing
  ignored a stray downloaded `.xlsx`/`.html`/`.pdf` if someone manually
  curl- or browser-tests `/api/export/xlsx`, `/api/export/html`, or PDF
  import from inside `backend/`. Added `*.xlsx`, `*.pdf`, and
  `expense-tracker-report.html` to `backend/.gitignore` for that —
  verified backend ships no legitimate file of any of those types, so
  this is a safe blanket rule, and confirmed with a second `git add -A`
  pass that all 27 real backend files still track normally.
- If a "fix the gitignores" ask like this comes up again: check first
  before changing anything. It's tempting to assume there's a bug because
  that's the framing, but a wrong preemptive edit is worse than reporting
  back "these are actually fine, here's what I checked."

**README** — the export feature (`/api/export/xlsx`, `/api/export/html`)
had shipped with zero documentation; only `/api/export/csv` was in the
API reference table. Added both new routes to the table, a new
"Exporting your data" section explaining what each of the three formats
contains and that the XLSX writer is deliberately stdlib-only (no
dependency) unlike the *importer's* Excel support, and added the three
new backend files to the project-structure listing.

**Testing done:** no real Go toolchain or module proxy access up to this
point in earlier sessions per prior notes below, but this time I did get
`golang-go` installed via `apt` (reachable) and got further than before:
`go build ./...` and `go vet ./...` both came back clean across every
file, including all three new export files, using the same throwaway
stub-module technique noted in earlier entries for the packages this
sandbox can't reach (`pgx`, plus `excelize`/`ledongthuc/pdf` this time).
Went one step further on `xlsx.go` specifically, since it's a hand-rolled
OOXML writer and that's exactly the kind of code that looks right but
silently produces a corrupt file: wrote a standalone harness calling
`BuildXLSX` directly with text/number/bold cells and a value containing
`<`, `&`, and `"` (to stress the XML escaping), then opened the result
with Python's `openpyxl` — parsed cleanly, all sheets/values/special
characters round-tripped exactly as written. Also caught and fixed one
real (if trivial) bug this found: `xlsx.go`'s cell-constructor block had
fallen out of `gofmt` alignment after `boldNumberCell` was added with a
longer name than its neighbors — cosmetic only, `gofmt -w` fixed it, no
behavior change. Did not build a similar harness for `export_html.go`;
reviewed it by hand instead (the `<` neutralizing in `safeJSON`, and the
JS side using `textContent`/`innerHTML` correctly for user-entered
description/category strings, both look right) — worth a quick manual
browser check of the downloaded HTML file if you want a second pair of
eyes on it before calling it fully verified.

## Recent work: PIN lock feature

Added a 4-8 digit PIN lock to protect the app on shared/local networks.

**Backend** (`backend/auth.go`, `handlers_auth.go`, `middleware.go`):
- PIN stored as salted SHA-256 hash in `pin.json`, never in plaintext.
- Sessions persisted to `sessions.json`, 30-day TTL.
- 5 failed attempts triggers a 60s lockout.
- `withAuth` middleware gates all non-public `/api/*` routes once a PIN
  is set (scoped to `/api/` specifically since a later change added static
  frontend file serving — see "Recent work: single-port serving" below —
  and static files must load before a PIN can even be entered).

**Frontend** (`useAuth.js`, `components/LockScreen.jsx`,
`components/ChangePinSection.jsx`):
- Numeric keypad lock screen, setup/confirm flow, change-PIN section
  under Manage.
- 401 from any API call drops the user back to the lock screen.

### Bugs found & fixed along the way (worth remembering)

1. **Lock screen wouldn't reset after a wrong PIN.** Root cause: watching
   the `error` prop for changes doesn't work when two consecutive failures
   produce the identical string ("incorrect PIN") — React bails on
   same-value state updates, so the effect never re-fired. Fixed by having
   `handleLogin`/`handleSetup` return a boolean success/failure directly
   instead of relying on error-text identity.
2. **Error styling stayed on when typing a fresh attempt.** The `is-error`
   CSS class was applied unconditionally to all 4 dots whenever `error`
   was truthy, regardless of how many digits had been retyped. Fixed with
   a local `showError` flag, cleared as soon as a new digit is pressed.
3. **No auto-reset.** Originally the error state only cleared once the user
   started typing again. Changed to auto-clear after ~700ms
   (`ERROR_DISPLAY_MS` in `LockScreen.jsx`) so it resets itself.
4. **Zip export bug (my mistake, not the app's):** for a while every zip I
   handed back was built with `zip -x "*.git*"`, which also matches
   `.gitignore` (since it contains the substring ".git") and silently
   dropped all three `.gitignore` files from the archive. Fixed by
   excluding `*/.git/*` instead. If a zip from around that time ever
   resurfaces, re-check it has `.gitignore` files before trusting it.

### Known minor gaps

- ~~`HandleAuthChangePIN` returns a generic 500...~~ Fixed — now returns a
  clean 400 for `ErrNoPINSet`, matching how `HandleAuthLogin` handles it.
- ~~Expired sessions aren't pruned...~~ Fixed — `AuthStore.PruneExpiredSessions`
  now runs on the same hourly ticker `main.go` already used for recurring
  transactions.
- PIN hashing is salted SHA-256, not bcrypt/argon2. Reasonable given the
  README's stated threat model (casual local-network access), but a short
  numeric PIN is weak against offline brute force if `pin.json` leaks.
  Left as-is for now — deliberate tradeoff, not an oversight.

### Next up

- `PATCH /api/transactions/{id}` for partial updates instead of full `PUT`
  replacement (from README's "Next steps" list) — good next self-contained
  feature.

## PATCH /api/transactions/{id}

Added partial-update support alongside the existing full-replacement `PUT`.

- `TransactionPatch` (models.go) — pointer fields so an omitted field in
  the JSON body stays nil/untouched, distinguishable from an explicitly
  zeroed one (e.g. clearing `description` to `""`).
- Handler reuses the existing `Get` → validate → `Update` flow already
  used by `PUT`, just merging the patch onto the fetched record first.
- Added `patchTransaction()` to `api.js` for frontend use later — not
  wired into the UI yet, since `EditTransactionRow.jsx` already collects
  and sends the full object, so there was no call site that needed it.
- Remembered to add `PATCH` to the CORS `Access-Control-Allow-Methods`
  list in `middleware.go` — easy to miss, and browser preflight requests
  would've silently failed without it.

### Inline amount edit (uses PATCH)

Gave `PATCH` an actual call site: click a transaction's amount in the
ledger list to edit just that number in place, without opening the full
edit row.

- `TransactionList.jsx` — clicking the amount swaps it for a number
  input (autofocus). Enter/blur commits via `onPatchAmount`; Escape
  cancels without saving.
- Guarded against a double-submit: disabling the input mid-save can
  trigger a second `blur` event in some browsers, which would otherwise
  re-enter `commitAmountEdit` and fire the PATCH twice.
- `App.jsx` — `handlePatchAmount(id, amount)` calls `patchTransaction`
  then refreshes, same pattern as the other mutation handlers.
- `App.css` — new `.ledger-line__amount--editable` / `-input` / `-error`
  styles, using existing `--gold`/`--rust`/`--paper-raised` tokens rather
  than introducing new ones.
- The full edit row (`EditTransactionRow.jsx`, still `PUT`-based) is
  untouched — this is purely an additional fast path for the single
  most commonly tweaked field.

## Postgres support (opt-in, JSON stays the default)

Added Postgres as an alternative storage backend, selected via a
`DATABASE_URL` env var, without touching any handler logic.

**Design:**
- New `stores.go` defines `TransactionStore`/`CategoryStore`/
  `BudgetStore`/`RecurringStore` interfaces matching the exact method
  signatures the existing JSON-backed `Store`/`FileStore[T]` already had.
  `handlers.go`'s `API` struct and `recurring.go`'s
  `GenerateDueTransactions` now depend on these interfaces instead of the
  concrete JSON types — that was the only real "surgery" needed elsewhere.
- `pg_store.go` / `pg_item_stores.go` — Postgres implementations of those
  same interfaces using `database/sql` + `jackc/pgx/v5/stdlib`. Same
  `ErrNotFound` semantics as the JSON stores (checked via `RowsAffected()`
  on updates/deletes, `sql.ErrNoRows` on lookups).
- `schema.go` — plain `CREATE TABLE IF NOT EXISTS` DDL, run once at
  startup when Postgres is selected. No migration framework — consistent
  with the project's stdlib-first philosophy, and fine for a
  single-developer app. Flagged in the README as the thing to replace
  with something like golang-migrate if this ever needs real schema
  migrations later.
- `storage.go` — `initStores()` checks `os.Getenv("DATABASE_URL")` and
  returns either the JSON stores or Postgres stores + a cleanup func
  (closes the DB pool). `main.go` just calls this instead of constructing
  stores directly.
- **Deliberately did NOT move `pin.json`/`sessions.json` to Postgres.**
  Auth is a separate concern from the app's business data — small,
  security-sensitive, and tied to the existing "local-only" threat model.
  Worth reconsidering only if this ever becomes multi-instance.

**Testing done (this sandbox has no internet access to the Go module
proxy, so verification took a couple of workarounds):**
- Installed a real Go toolchain via `apt` (archive.ubuntu.com is
  reachable) and ran `go vet`/`go build` — confirmed zero syntax or type
  errors across every file, including the new ones.
- The actual `jackc/pgx/v5` dependency couldn't be fully fetched here (its
  transitive test deps eventually need `gopkg.in`, which isn't reachable
  in this sandbox) — this will resolve normally on a real machine with
  `go get github.com/jackc/pgx/v5/stdlib && go mod tidy`. To still get a
  real compile check, built a throwaway local stub package satisfying the
  same import path via a `go.mod` `replace` directive, and got a clean
  full build + `go vet` against it.
- Installed a real local Postgres 16 via `apt`, applied the exact `schema`
  constant from `schema.go`, then ran every literal SQL statement used in
  `pg_store.go`/`pg_item_stores.go` against it by hand — inserts, list/get
  queries, updates, deletes, and the `CHECK` constraints all behaved
  correctly.
- Net effect: high confidence the code is correct, even though it couldn't
  be run end-to-end as compiled Go against a live DB in this environment.

**One-time setup still needed on your machine:**
```bash
cd backend
go get github.com/jackc/pgx/v5/stdlib
go mod tidy
export DATABASE_URL="postgres://user:pass@localhost:5432/expensetracker?sslmode=disable"
go run .
```

## Notification system + budget color states

Added toasts + browser push notifications, and made the budget UI reflect
green/yellow/red status beyond just the thin progress bar.

**Notifications** (`useNotifications.js`, `components/ToastStack.jsx`,
`components/NotificationBell.jsx`):
- `useNotifications()` owns toast state and the `Notification.permission`
  state; `notify(type, title, message, opts)` pushes a toast and, only when
  the tab is `document.hidden` and permission is `"granted"`, also fires a
  real `new Notification(...)`. Deliberately not double-firing when the tab
  is focused — the toast is enough there.
- `App.jsx` wraps every mutation handler in a small `withFeedback()` helper
  that notifies success/error and **re-throws** on error so each form's own
  existing `try/catch` (inline validation messages, `finally` cleanup in
  `TransactionForm`/`CategoryManager`/etc.) still runs exactly as before —
  the toast is additive, not a replacement.
- Budget threshold alerts and recurring-rule-fired alerts are diffed against
  a previous snapshot (`budgetTiersRef`/`recurringDueRef` in `App.jsx`), so
  refreshing at 92% used doesn't re-announce the same "approaching limit"
  toast every poll — only actual tier crossings do. First load seeds these
  maps silently (no backlog of alerts for budgets already over limit
  yesterday).
- Added a quiet 3-minute polling loop (`refresh({ silent: true })`) while
  unlocked, since recurring rules and budget usage can both change
  server-side without the user touching anything — this is what lets those
  two alert types actually surface during a live session instead of only
  after a manual action.

**Budget colors** (`BudgetManager.jsx`, `App.css`):
- The progress bar already shifted green → gold → rust at 80%/100% before
  this — that part wasn't new. What changed: the whole row now carries the
  same tier as a class (`is-ok`/`is-close`/`is-over`), so the category
  label, the spent/limit figures, and a subtle row background tint all
  shift together instead of just the bar. `budget-row__category` was
  previously hardcoded to `--rust` regardless of status (a leftover ledger
  accent, not an actual warning color) — fixed to track status instead.

### Known minor gaps
- Push notifications are plain client-side `Notification` API — no service
  worker, no server push. Only fires while some tab/window of the app is
  still open somewhere; closing the browser fully stops it, same
  limitation the recurring-rule catch-up logic already has.
- No "notification history" — dismissed/expired toasts are gone for good,
  nothing is persisted. Fine for now given the app's single-user local
  scope; would need a Postgres-backed table for a proper log.

## Repo hygiene

- Root `.gitignore` added covering OS/editor cruft and `.env*`.
- `backend/.gitignore` and `frontend/.gitignore` already handle their own
  build artifacts and — importantly — `pin.json` / `sessions.json` /
  `data.json`. Double-check `git status` before any push to make sure none
  of those show up staged.

## CSV import + undo delete + savings goals

Three self-contained features added in one pass.

### CSV import (`handlers_import.go`, `api.js`, `Manage.jsx`)

- `POST /api/import/csv` mirrors the export format but matches columns by
  *name* (case-insensitive) instead of position, so a reordered spreadsheet
  export still imports cleanly. Accepts either a raw CSV body or a
  multipart upload — the frontend always sends multipart via `FormData`,
  but raw-body works too if you're scripting against the API directly.
- Bad rows are skipped, not fatal — the response reports
  `{imported, skipped, errors}` with up to 20 example error strings so one
  malformed row doesn't sink an otherwise-good file. Verified by hand: a
  5-row test CSV with one bad amount and one bad type correctly imported 3
  and skipped 2, with matching error messages, via both raw-body and
  multipart requests.
- Frontend: a plain `<input type="file">` hidden behind a styled button in
  `Manage.jsx` (same visual treatment as the existing export button),
  triggered via a ref instead of a `<label>` so the disabled/importing
  state can gate it. Import result surfaces as a success toast (with
  skipped count) plus a separate warning toast listing example errors if
  any rows failed.

### Undo delete (`useNotifications.js`, `ToastStack.jsx`, `App.jsx`)

- Deleting a transaction no longer calls `DELETE` immediately. It's hidden
  from the ledger right away (`pendingDeleteIds`, a Set of ids filtered out
  in `filteredTransactions`), and the actual API call is deferred 5s
  (`UNDO_WINDOW_MS`) behind a `setTimeout`. The toast for the deletion gets
  a new `action: { label, onClick }` (rendered by `ToastStack.jsx` as a
  `.toast__action` button) — clicking it clears the pending timeout and
  un-hides the transaction, so nothing was ever sent to the server.
- Dismissing the toast with the × does **not** cancel the pending delete —
  only the explicit Undo button does. This was a deliberate choice: people
  reflexively dismiss toasts, and a dismiss-cancels-delete design would
  make "clean up this notification" accidentally undo actions.
- Known limitation, documented in the README: this is in-memory only.
  Closing the tab or reloading mid-window loses the timer, so the
  transaction just never gets deleted (same outcome as clicking Undo, just
  unintentional). A more robust version would need a server-side soft
  delete with a scheduled sweep — didn't seem worth the complexity for a
  single-user local app; flagging here in case that tradeoff is revisited.
- `deletingId` state (previously used to show "Removing…" and disable the
  button mid-delete) was removed — it's no longer meaningful once delete is
  optimistic, since the row disappears before a second click is possible.
  `TransactionList` still accepts the prop (now always passed as `null`)
  rather than touching that component's signature for a one-line change.

### Savings goals (`models.go`, `handlers_goals.go`, `GoalsManager.jsx`)

- Deliberately separate from `Budget`: a budget caps spending in a
  category (derived from transactions), a goal tracks progress toward
  saving something (`saved_amount`, updated only via explicit
  contributions). Kept them decoupled rather than trying to derive
  "savings" from transaction data, which would either require a dedicated
  category with special-cased handling or double-count money already
  reflected in the ledger.
- `POST /api/goals/{id}/contribute` takes `{"amount": N}` — N can be
  negative to correct an over-contribution, rejected with 400 if it would
  push `saved_amount` below 0. Editing a goal via `PUT` carries the
  existing `saved_amount` forward unless the body explicitly sets a
  non-zero one, so renaming a goal or bumping its target doesn't wipe
  progress.
- `/api/goals/{id}/contribute` is routed through the same
  `HandleGoalByID` as PUT/DELETE rather than a separate mux pattern —
  Go 1.22's `ServeMux` doesn't do wildcard sub-paths without a second,
  more specific registration, so it's split manually with
  `strings.CutSuffix(path, "/contribute")` instead. Slightly less obvious
  than a dedicated route, but keeps `main.go`'s route list to one line per
  resource like everything else.
- **Deliberately not wired into the Postgres path** — `goals` always lives
  in a local `goals.json` regardless of `DATABASE_URL`, same reasoning as
  the PIN/session store (small, not core ledger data, not worth the
  `stores.go` interface + `pg_item_stores.go` plumbing yet). If goals ever
  need to survive a Postgres migration, that's the place to revisit.
- Frontend (`GoalsManager.jsx`) intentionally mirrors `BudgetManager.jsx`'s
  structure (progress bar, row list, inline create form) but with the
  color logic inverted — for a budget, more filled means worse (gold at
  80%, rust at 100%); for a goal, more filled is always good, so the bar
  stays `--jade` throughout and the row only gets an extra highlight once
  it's fully reached (100%+, with a checkmark icon swapped in for the
  piggy-bank one).

### Testing done

Same sandbox constraint as the Postgres work: no network access to the Go
module proxy. Installed `golang-go` via `apt` (reachable) and, to satisfy
the `jackc/pgx/v5/stdlib` import without touching any Postgres code, built
a tiny throwaway stub module (`sql.Register("pgx", ...)` returning a nil
driver) wired in via a temporary `go.mod` `replace` directive — removed
again before finishing, `go.mod` is back to exactly what it was. `go vet`
and `go build` both came back clean against the stub, and none of this
touched `pg_store.go`/`pg_item_stores.go`, so that code is exactly as it
was in the previous handover.

Ran the actual built binary (JSON mode) and exercised every new endpoint
by hand with `curl`: goal create → contribute → over-withdraw rejection →
list → edit (saved_amount preserved) → delete; CSV import via both raw
body and multipart, including the malformed-row skip behavior. Frontend:
`npm install` + `oxlint` (0 warnings) + `vite build` both clean, and
confirmed the built bundle actually references `/goals` and `/import/csv`
rather than silently failing to wire up. Didn't have a real browser in
this sandbox to click through the UI, so the undo-delete timing/toast
interaction and the goal progress-bar animation are unverified visually —
worth a quick manual pass in a real browser before considering this fully
done.

## Recent work: single-port serving (for tunneling)

Goal: let the whole app run off one port so it can be shared over a
tunnel (`cloudflared`/`ngrok`) without the frontend's API calls breaking.

**The problem:** `frontend/src/api.js` used to build `BASE_URL` from
`window.location.hostname` with a hardcoded `:8080` suffix — fine on a
LAN, but a tunnel only forwards one port to one public URL, so the app
would load but every API call would 404/hang trying to reach a `:8080`
that doesn't exist from outside.

**The fix — same-origin instead of two ports:**
- `frontend/src/api.js`: `BASE_URL` is now just `/api` (relative,
  same-origin) instead of `http://${hostname}:8080/api`.
- `frontend/vite.config.js`: added a dev-server proxy (`/api` and
  `/health` → `http://localhost:8080`) so `npm run dev` still works
  exactly as before with the relative path; also set `host: true` to
  keep LAN access from a phone working.
- `backend/main.go`: added `spaFileServer()`, a small static file
  handler with SPA fallback (unknown paths serve `index.html`, since
  routing is client-side). Registered on `/` in the mux, reading from
  `../frontend/dist` by default (`FRONTEND_DIST` env var overrides).
  Only registers the handler if that directory actually exists, so
  running without a build first just logs a note and stays API-only —
  doesn't crash.
- `backend/middleware.go`: **found and fixed a real bug while doing
  this** — `withAuth` previously gated *every* non-public path once a
  PIN was set, which would've 401'd the static frontend files
  themselves (index.html/JS/CSS) the instant someone had a PIN
  configured, since browsers don't attach the session token to a normal
  page load. Scoped the check to `strings.HasPrefix(r.URL.Path,
  "/api/")` so static files always load; the actual data endpoints are
  exactly as protected as before.
- `frontend/src/App.jsx`: updated one stale error-message string that
  referenced the old hardcoded `:8080` assumption. Text only, no logic
  change.

### Testing done

No Go toolchain available in that sandbox session (`go: not found`), so
this was reviewed by hand rather than compiled — flagged that explicitly
to the user. Worth running `go build ./...` and `go vet ./...` for real
before trusting it further, particularly the `withAuth` path-prefix
change since auth bugs are exactly the kind of thing that's easy to get
subtly wrong and expensive to get wrong quietly.

Confirmed via user's own terminal screenshots (not by me directly) that:
`npm run build` → `go run .` → `cloudflared tunnel --url
http://localhost:8080` produces a working `trycloudflare.com` link that
loads the full app end-to-end, including from a second, unrelated device
on a different network. Quick tunnels generate a new random URL on every
restart, and require both the `go run .` and `cloudflared tunnel`
processes to stay running the whole time — either stopping kills the
link for everyone immediately. Worth keeping in mind if this pattern
gets used again: quick tunnels have no uptime guarantee per Cloudflare's
own CLI output, fine for casual sharing, not for anything that needs to
stay up reliably.

## Next up: guest PIN (not started yet)

**Why this over multiple accounts:** user is actively sharing tunnel
links (see previous section) with other people, and right now anyone
with the PIN has full read/write access to everything — that's the
immediate problem. Multiple accounts (splitting transactions across
e.g. checking/savings/credit card) was considered too, and is a
reasonable feature on its own, but is lower priority since nothing is
actively blocked by its absence today. Confirmed the two don't
conflict: guest PIN only touches `auth.go`/`middleware.go`/
`handlers_auth.go`, multiple accounts would only touch
`models.go`/`store.go`/a new `handlers_accounts.go` — no shared surface
area, safe to build in either order.

**What it should do**, per discussion with the user:

Guest sessions can:
- View the transaction ledger, balance, reports/charts
- View budgets, budget status, and goals
- View recurring rules

Guest sessions cannot:
- Create, edit, or delete transactions
- Create/delete categories, budgets, recurring rules, or goals
- Contribute to goals
- Import (CSV/Excel/PDF)
- Export — defaulted to owner-only since exporting lets someone walk
  away with the full data even though the action itself is read-only in
  spirit; revisit if that turns out to be too strict
- Change either PIN

**Planned implementation shape** (not yet built, sketch only):
- A second PIN, stored alongside the existing one (`pin.json` currently
  holds one salted hash — needs a second slot, e.g. `guest_pin_hash`).
- Sessions need a role (`owner` vs `guest`) attached at login time,
  depending on which PIN was entered — `sessions.json`/`AuthStore`
  currently has no concept of role, just a valid/expired token.
- `withAuth` (middleware.go) currently only checks "is there a valid
  session" for `/api/*` — needs to also check the session's role against
  an allowlist of guest-permitted routes/methods for anything beyond a
  plain valid-session check.
- Frontend: `LockScreen.jsx` needs no change to the PIN entry itself
  (owner and guest PINs look identical to type in — the backend
  determines which one matched). `useAuth.js` needs to expose the
  resulting role so components can react to it. Any write-triggering UI
  (add/edit/delete buttons across `TransactionForm.jsx`,
  `EditTransactionRow.jsx`, `BudgetManager.jsx`, `RecurringManager.jsx`,
  `GoalsManager.jsx`, `CategoryManager.jsx`, `Manage.jsx`'s import/export
  section) needs to hide or disable itself when role is `guest`.

**Open question for whoever picks this up:** whether a 401 on a
guest-blocked route should look identical to an expired-session 401
(simplest) or return something more specific so the frontend can show
"you don't have permission" instead of bouncing to the lock screen as if
logged out — worth deciding before wiring up the frontend gating, since
it changes what `useAuth.js`'s 401 handler needs to do.


