package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// exportTxn is the shape of a transaction as embedded in the HTML export's
// JSON payload — trimmed to what the report actually uses, so we don't leak
// internal fields like CreatedAt for no reason.
type exportTxn struct {
	Date        string  `json:"date"`
	Type        string  `json:"type"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
}

// safeJSON marshals v and neutralizes "<" so the result can never
// accidentally close a surrounding <script> tag (or open an HTML comment)
// when embedded directly into a page — transaction descriptions and
// category names are user-entered text, so this matters.
func safeJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	b = bytes.ReplaceAll(b, []byte("<"), []byte(`\u003c`))
	return string(b), nil
}

// BuildDashboardHTML renders a single, self-contained HTML file: all data is
// embedded inline as JSON and all charts/filtering run in vanilla JS with no
// external requests, so the file works fully offline once downloaded.
func BuildDashboardHTML(transactions []*Transaction, report ReportResponse) ([]byte, error) {
	txns := make([]exportTxn, 0, len(transactions))
	for _, t := range transactions {
		txns = append(txns, exportTxn{
			Date:        t.Date,
			Type:        string(t.Type),
			Category:    t.Category,
			Description: t.Description,
			Amount:      t.Amount,
		})
	}

	txnJSON, err := safeJSON(txns)
	if err != nil {
		return nil, err
	}
	reportJSON, err := safeJSON(report)
	if err != nil {
		return nil, err
	}

	generatedAt := time.Now().Format("2 Jan 2006, 15:04")

	// NOTE: the template below is full of literal "%" characters (CSS
	// percentages, JS string concatenation for bar widths, etc.), so it is
	// deliberately NOT run through fmt.Sprintf — that would misinterpret
	// every stray "%" as a format verb. Plain token replacement sidesteps
	// that entirely.
	replacer := strings.NewReplacer(
		"__GENERATED_AT__", generatedAt,
		"__TXN_COUNT__", strconv.Itoa(report.TransactionCount),
		"__TRANSACTIONS_JSON__", txnJSON,
		"__REPORT_JSON__", reportJSON,
	)
	page := replacer.Replace(htmlTemplate)
	return []byte(page), nil
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Ledger Export</title>
<style>
  :root {
    --ink: #14151f;
    --ink-soft: #6b6e85;
    --paper: #f3f4fa;
    --paper-raised: #ffffff;
    --paper-sunken: #eceef7;
    --line: #e3e5f0;
    --jade: #0ea968;
    --jade-soft: #e3f7ec;
    --jade-bright: #6ee6b5;
    --rust: #e6492f;
    --rust-soft: #fce9e5;
    --rust-bright: #ff9a90;
    --violet: #6552ec;
    --violet-soft: #ecebfd;
    --teal: #12b7c6;
    --on-dark-fg: #f6f6fb;
    --on-dark-fg-soft: rgba(246, 246, 251, 0.68);
    --on-dark-line: rgba(246, 246, 251, 0.14);
    --r-sm: 10px;
    --r-md: 14px;
    --r-lg: 20px;
    --r-xl: 26px;
    --shadow-sm: 0 2px 8px rgba(30, 27, 75, 0.06), 0 1px 2px rgba(30, 27, 75, 0.05);
    --shadow-md: 0 12px 28px -8px rgba(30, 27, 75, 0.16), 0 2px 6px rgba(30, 27, 75, 0.05);
    --font-display: "Space Grotesk", ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
    --font-mono: ui-monospace, "SFMono-Regular", "JetBrains Mono", Menlo, Consolas, monospace;
    --font-ui: "Inter", ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
  }
  * { box-sizing: border-box; }
  body {
    margin: 0;
    background: var(--paper);
    color: var(--ink);
    font-family: var(--font-ui);
    -webkit-font-smoothing: antialiased;
  }
  .wrap { max-width: 1080px; margin: 0 auto; padding: 28px 24px 80px; }

  /* ---------- Hero (mirrors the app's balance header) ---------- */
  .hero {
    background: linear-gradient(160deg, #15111f 0%, #191534 55%, #101018 100%);
    color: var(--on-dark-fg);
    border-radius: var(--r-xl);
    padding: 28px 28px 22px;
    margin-bottom: 22px;
    position: relative;
    overflow: hidden;
    box-shadow: var(--shadow-md);
  }
  .hero::after {
    content: "";
    position: absolute;
    inset: 0;
    background:
      radial-gradient(circle at 0% 0%, rgba(101, 82, 236, 0.35), transparent 55%),
      radial-gradient(circle at 100% 0%, rgba(18, 183, 198, 0.28), transparent 50%);
    pointer-events: none;
  }
  .hero__top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 14px;
    position: relative;
  }
  .hero__eyebrow {
    font-family: var(--font-mono);
    font-size: 11px;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--teal);
    font-weight: 600;
  }
  .hero__badge {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    font-size: 11px;
    font-weight: 600;
    padding: 5px 11px;
    border-radius: 999px;
  }
  .hero__badge.is-jade { background: rgba(52, 211, 153, 0.16); color: var(--jade-bright); }
  .hero__badge.is-rust { background: rgba(255, 107, 94, 0.16); color: var(--rust-bright); }
  .hero__main { display: flex; flex-direction: column; gap: 5px; margin-bottom: 16px; position: relative; }
  .hero__label { font-size: 13px; color: var(--on-dark-fg-soft); font-weight: 500; }
  .hero__amount {
    font-family: var(--font-display);
    font-size: 42px;
    font-weight: 700;
    letter-spacing: -0.02em;
    line-height: 1.1;
    font-variant-numeric: tabular-nums;
  }
  .hero__amount.is-negative { color: var(--rust-bright); }
  .hero__amount.is-positive {
    background: linear-gradient(120deg, var(--on-dark-fg) 30%, #b9ffe9 100%);
    -webkit-background-clip: text;
    background-clip: text;
    color: transparent;
  }
  .hero__row {
    display: flex;
    gap: 24px;
    flex-wrap: wrap;
    border-top: 1px solid var(--on-dark-line);
    padding-top: 14px;
    position: relative;
  }
  .hero__stat { display: flex; align-items: center; gap: 7px; font-size: 13px; color: var(--on-dark-fg); opacity: 0.92; }
  .hero__stat .mono { font-family: var(--font-mono); }
  .hero__meta {
    margin-top: 14px;
    padding-top: 14px;
    border-top: 1px solid var(--on-dark-line);
    font-size: 11.5px;
    font-family: var(--font-mono);
    color: var(--on-dark-fg-soft);
    position: relative;
  }
  .hero__meta strong { color: var(--on-dark-fg); }
  .dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; flex-shrink: 0; }
  .dot--jade { background: var(--jade); box-shadow: 0 0 0 3px rgba(52, 211, 153, 0.18); }
  .dot--rust { background: var(--rust); box-shadow: 0 0 0 3px rgba(255, 107, 94, 0.18); }

  /* ---------- Panels ---------- */
  .panels { display: grid; grid-template-columns: 1.1fr 1fr; gap: 16px; margin-bottom: 20px; }
  @media (max-width: 760px) { .panels { grid-template-columns: 1fr; } }
  .panel {
    background: var(--paper-raised);
    border: 1px solid var(--line);
    border-radius: var(--r-lg);
    padding: 18px 20px;
    box-shadow: var(--shadow-sm);
  }
  .panel h2 {
    font-family: var(--font-display);
    font-size: 13px;
    font-weight: 600;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--ink-soft);
    margin: 0 0 14px;
  }

  .cat-row { display: flex; align-items: center; gap: 10px; margin-bottom: 9px; font-size: 12.5px; }
  .cat-row .cat-name { width: 108px; flex-shrink: 0; color: var(--ink-soft); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .cat-row .bar-track { flex: 1; background: var(--paper-sunken); border-radius: 6px; height: 9px; overflow: hidden; }
  .cat-row .bar-fill { height: 100%; border-radius: 6px; background: var(--violet); }
  .cat-row.income .bar-fill { background: var(--jade); }
  .cat-row .cat-amount { font-family: var(--font-mono); width: 92px; text-align: right; flex-shrink: 0; }

  #trendChart { width: 100%; height: auto; overflow: visible; }
  .trend-tooltip { font-family: var(--font-mono); font-size: 10.5px; fill: var(--ink-soft); }

  .controls { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 12px; }
  .controls input, .controls select {
    font-family: var(--font-ui);
    font-size: 13px;
    padding: 9px 12px;
    border-radius: var(--r-sm);
    border: 1px solid var(--line);
    background: var(--paper-raised);
    color: var(--ink);
  }
  .controls input { flex: 1; min-width: 160px; }
  .controls input:focus, .controls select:focus { outline: 2px solid var(--violet); outline-offset: -1px; }

  table { width: 100%; min-width: 560px; border-collapse: collapse; background: var(--paper-raised); border-radius: var(--r-lg); overflow: hidden; box-shadow: var(--shadow-sm); }
  .table-scroll { overflow-x: auto; border-radius: var(--r-lg); -webkit-overflow-scrolling: touch; }
  thead th {
    text-align: left;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--ink-soft);
    padding: 10px 14px;
    border-bottom: 1px solid var(--line);
    cursor: pointer;
    user-select: none;
    white-space: nowrap;
  }
  thead th:hover { color: var(--ink); }
  thead th.sorted::after { content: " " attr(data-dir); font-size: 9px; }
  tbody td { padding: 10px 14px; border-bottom: 1px solid var(--line); font-size: 13px; vertical-align: top; }
  tbody tr:last-child td { border-bottom: none; }
  tbody tr:hover { background: var(--paper-sunken); }
  td.amount, th.amount { text-align: right; font-family: var(--font-mono); white-space: nowrap; }
  td.amount.income { color: var(--jade); }
  td.amount.expense { color: var(--rust); }
  td.type-tag span {
    font-size: 10.5px;
    padding: 2px 8px;
    border-radius: 20px;
    font-weight: 600;
  }
  td.type-tag .income-tag { background: var(--jade-soft); color: var(--jade); }
  td.type-tag .expense-tag { background: var(--rust-soft); color: var(--rust); }

  .empty-row td { text-align: center; color: var(--ink-soft); padding: 28px; font-size: 13px; }
  .row-count { font-size: 12px; color: var(--ink-soft); margin: 10px 2px 0; font-family: var(--font-mono); }

  footer { margin-top: 32px; font-size: 11.5px; color: var(--ink-soft); text-align: center; }

  @media print {
    .controls { display: none; }
    body { background: white; }
    .panel, table { box-shadow: none; }
  }

  @media (max-width: 480px) {
    .wrap { padding: 18px 14px 60px; }
    .hero { padding: 22px 18px 18px; border-radius: var(--r-lg); }
    .hero__amount { font-size: 32px; }
    .hero__row { gap: 16px; }
    .panel { padding: 16px; border-radius: var(--r-md); }
    .controls { flex-direction: column; }
    .controls input, .controls select { width: 100%; }
    .cat-row .cat-name { width: 84px; }
  }
</style>
</head>
<body>
<div class="wrap">

  <header class="hero">
    <div class="hero__top">
      <span class="hero__eyebrow">Ledger export</span>
      <span class="hero__badge" id="heroBadge"></span>
    </div>
    <div class="hero__main">
      <span class="hero__label">Net balance</span>
      <span class="hero__amount" id="heroAmount">—</span>
    </div>
    <div class="hero__row">
      <div class="hero__stat"><span class="dot dot--jade"></span><span>In</span><span class="mono" id="heroIncome">—</span></div>
      <div class="hero__stat"><span class="dot dot--rust"></span><span>Out</span><span class="mono" id="heroExpense">—</span></div>
    </div>
    <div class="hero__meta">Generated <strong>__GENERATED_AT__</strong> &middot; <strong>__TXN_COUNT__</strong> transactions &middot; amounts in NGN</div>
  </header>

  <div class="panels">
    <div class="panel">
      <h2>By category</h2>
      <div id="categoryChart"></div>
    </div>
    <div class="panel">
      <h2>Monthly trend</h2>
      <svg id="trendChart" viewBox="0 0 420 200" preserveAspectRatio="xMidYMid meet"></svg>
    </div>
  </div>

  <div class="panel">
    <h2 style="margin-bottom:12px;">Transactions</h2>
    <div class="controls">
      <input type="text" id="searchBox" placeholder="Search description or category…">
      <select id="typeFilter">
        <option value="">All types</option>
        <option value="income">Income</option>
        <option value="expense">Expense</option>
      </select>
      <select id="categoryFilter"><option value="">All categories</option></select>
    </div>
    <div class="table-scroll">
    <table>
      <thead>
        <tr>
          <th data-key="date">Date</th>
          <th data-key="type">Type</th>
          <th data-key="category">Category</th>
          <th data-key="description">Description</th>
          <th data-key="amount" class="amount">Amount</th>
        </tr>
      </thead>
      <tbody id="txnBody"></tbody>
    </table>
    </div>
    <div class="row-count" id="rowCount"></div>
  </div>

  <footer>Exported from your expense tracker. This file is self-contained — it works offline and shares no data over the network.</footer>
</div>

<script>
  const TRANSACTIONS = __TRANSACTIONS_JSON__;
  const REPORT = __REPORT_JSON__;

  const nairaFull = new Intl.NumberFormat("en-NG", { style: "currency", currency: "NGN", minimumFractionDigits: 2 });
  const fmtMoney = (n) => nairaFull.format(n);
  const fmtDate = (iso) => {
    const d = new Date(iso + "T00:00:00");
    if (isNaN(d.getTime())) return iso;
    return d.toLocaleDateString("en-GB", { day: "2-digit", month: "short", year: "numeric" });
  };
  const fmtMonth = (m) => {
    const [y, mo] = m.split("-");
    return new Date(Number(y), Number(mo) - 1, 1).toLocaleDateString("en-GB", { month: "short", year: "numeric" });
  };

  // ---- Hero card ----
  const isPositive = REPORT.net >= 0;
  const heroAmount = document.getElementById("heroAmount");
  heroAmount.textContent = fmtMoney(REPORT.net);
  heroAmount.classList.add(isPositive ? "is-positive" : "is-negative");
  const heroBadge = document.getElementById("heroBadge");
  heroBadge.textContent = isPositive ? "▲ In the black" : "▼ In the red";
  heroBadge.classList.add(isPositive ? "is-jade" : "is-rust");
  document.getElementById("heroIncome").textContent = fmtMoney(REPORT.total_income);
  document.getElementById("heroExpense").textContent = fmtMoney(REPORT.total_expense);

  // ---- Category bars ----
  (function renderCategoryChart() {
    const container = document.getElementById("categoryChart");
    const cats = (REPORT.by_category || []).slice().sort((a, b) => Math.abs(b.total) - Math.abs(a.total));
    if (cats.length === 0) {
      container.innerHTML = '<p style="color:var(--ink-soft);font-size:13px;">No transactions yet.</p>';
      return;
    }
    const maxAbs = Math.max(...cats.map((c) => Math.abs(c.total)), 1);
    container.innerHTML = cats.map((c) => {
      const pct = Math.max((Math.abs(c.total) / maxAbs) * 100, 2);
      const isIncome = c.total >= 0;
      return '<div class="cat-row' + (isIncome ? ' income' : '') + '">' +
        '<div class="cat-name" title="' + escapeHtml(c.category) + '">' + escapeHtml(c.category) + '</div>' +
        '<div class="bar-track"><div class="bar-fill" style="width:' + pct.toFixed(1) + '%"></div></div>' +
        '<div class="cat-amount">' + fmtMoney(c.total) + '</div>' +
        '</div>';
    }).join("");
  })();

  // ---- Monthly trend (simple SVG line chart, income vs expense) ----
  (function renderTrendChart() {
    const svg = document.getElementById("trendChart");
    const months = REPORT.by_month || [];
    if (months.length === 0) {
      svg.outerHTML = '<p style="color:var(--ink-soft);font-size:13px;">No transactions yet.</p>';
      return;
    }
    const W = 420, H = 200, padL = 34, padR = 10, padT = 12, padB = 24;
    const plotW = W - padL - padR, plotH = H - padT - padB;
    const maxVal = Math.max(...months.map((m) => Math.max(m.total_income, m.total_expense)), 1);
    const x = (i) => padL + (months.length === 1 ? plotW / 2 : (i / (months.length - 1)) * plotW);
    const y = (v) => padT + plotH - (v / maxVal) * plotH;

    const line = (key, color) => {
      const pts = months.map((m, i) => x(i) + "," + y(m[key]).toFixed(1)).join(" ");
      return '<polyline points="' + pts + '" fill="none" stroke="' + color + '" stroke-width="2"/>' +
        months.map((m, i) => '<circle cx="' + x(i) + '" cy="' + y(m[key]).toFixed(1) + '" r="2.5" fill="' + color + '"/>').join("");
    };

    const gridLines = [0, 0.5, 1].map((f) => {
      const gy = padT + plotH - f * plotH;
      return '<line x1="' + padL + '" y1="' + gy + '" x2="' + (W - padR) + '" y2="' + gy + '" stroke="var(--line)" stroke-width="1"/>' +
        '<text x="4" y="' + (gy + 3) + '" class="trend-tooltip">' + Math.round(maxVal * f).toLocaleString() + '</text>';
    }).join("");

    const labels = months.map((m, i) => {
      if (months.length > 8 && i % Math.ceil(months.length / 8) !== 0) return "";
      return '<text x="' + x(i) + '" y="' + (H - 6) + '" class="trend-tooltip" text-anchor="middle">' + fmtMonth(m.month) + '</text>';
    }).join("");

    svg.innerHTML = gridLines + line("total_income", "var(--jade)") + line("total_expense", "var(--rust)") + labels;
  })();

  // ---- Transaction table ----
  const tbody = document.getElementById("txnBody");
  const searchBox = document.getElementById("searchBox");
  const typeFilter = document.getElementById("typeFilter");
  const categoryFilter = document.getElementById("categoryFilter");
  const rowCount = document.getElementById("rowCount");
  let sortKey = "date";
  let sortDir = -1; // newest first by default

  const uniqueCategories = [...new Set(TRANSACTIONS.map((t) => t.category))].sort();
  categoryFilter.innerHTML += uniqueCategories.map((c) => '<option value="' + escapeHtml(c) + '">' + escapeHtml(c) + '</option>').join("");

  function escapeHtml(s) {
    const div = document.createElement("div");
    div.textContent = s;
    return div.innerHTML;
  }

  function currentRows() {
    const q = searchBox.value.trim().toLowerCase();
    const type = typeFilter.value;
    const cat = categoryFilter.value;
    let rows = TRANSACTIONS.filter((t) => {
      if (type && t.type !== type) return false;
      if (cat && t.category !== cat) return false;
      if (q && !(t.description.toLowerCase().includes(q) || t.category.toLowerCase().includes(q))) return false;
      return true;
    });
    rows.sort((a, b) => {
      let av = a[sortKey], bv = b[sortKey];
      if (sortKey === "amount") { av = Number(av); bv = Number(bv); }
      if (av < bv) return -1 * sortDir;
      if (av > bv) return 1 * sortDir;
      return 0;
    });
    return rows;
  }

  function render() {
    const rows = currentRows();
    if (rows.length === 0) {
      tbody.innerHTML = '<tr class="empty-row"><td colspan="5">No transactions match your filters.</td></tr>';
    } else {
      tbody.innerHTML = rows.map((t) => {
        const tagClass = t.type === "income" ? "income-tag" : "expense-tag";
        const amtClass = t.type === "income" ? "income" : "expense";
        const sign = t.type === "income" ? "+" : "−";
        return '<tr>' +
          '<td>' + fmtDate(t.date) + '</td>' +
          '<td class="type-tag"><span class="' + tagClass + '">' + t.type + '</span></td>' +
          '<td>' + escapeHtml(t.category) + '</td>' +
          '<td>' + escapeHtml(t.description || "—") + '</td>' +
          '<td class="amount ' + amtClass + '">' + sign + fmtMoney(t.amount) + '</td>' +
          '</tr>';
      }).join("");
    }
    rowCount.textContent = rows.length + " of " + TRANSACTIONS.length + " transactions shown";
    document.querySelectorAll("thead th[data-key]").forEach((th) => {
      th.classList.toggle("sorted", th.dataset.key === sortKey);
      th.dataset.dir = sortDir === 1 ? "▲" : "▼";
    });
  }

  document.querySelectorAll("thead th[data-key]").forEach((th) => {
    th.addEventListener("click", () => {
      const key = th.dataset.key;
      if (sortKey === key) { sortDir *= -1; } else { sortKey = key; sortDir = 1; }
      render();
    });
  });
  searchBox.addEventListener("input", render);
  typeFilter.addEventListener("change", render);
  categoryFilter.addEventListener("change", render);

  render();
</script>
</body>
</html>
`
