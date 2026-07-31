import { todayISO } from "./format";

// PERIODS defines the selectable report windows. "custom" is handled
// separately (it needs two date inputs instead of a fixed calculation).
export const PERIODS = [
  { id: "this-month", label: "This month" },
  { id: "last-3-months", label: "Last 3 months" },
  { id: "this-year", label: "This year" },
  { id: "all-time", label: "All time" },
  { id: "custom", label: "Custom" },
];

function isoMonthsAgo(n) {
  const d = new Date();
  d.setDate(1); // avoid month-length rollover surprises (e.g. Mar 31 - 1mo)
  d.setMonth(d.getMonth() - n);
  return d.toISOString().slice(0, 10);
}

function firstOfMonthISO() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-01`;
}

function firstOfYearISO() {
  return `${new Date().getFullYear()}-01-01`;
}

// getPeriodRange resolves a period id (+ optional custom bounds) to a
// concrete { from, to } pair of ISO date strings, or { from: null, to: null }
// for "all-time" (no filtering).
export function getPeriodRange(periodId, custom) {
  const today = todayISO();
  switch (periodId) {
    case "this-month":
      return { from: firstOfMonthISO(), to: today };
    case "last-3-months":
      return { from: isoMonthsAgo(2), to: today };
    case "this-year":
      return { from: firstOfYearISO(), to: today };
    case "custom":
      return { from: custom?.from || null, to: custom?.to || today };
    case "all-time":
    default:
      return { from: null, to: null };
  }
}

// filterTransactionsByRange keeps transactions whose ISO date string falls
// within [from, to] inclusive. ISO "YYYY-MM-DD" strings sort lexicographically
// in the same order as chronologically, so plain string comparison is safe.
export function filterTransactionsByRange(transactions, range) {
  if (!range.from && !range.to) return transactions;
  return transactions.filter((t) => {
    if (range.from && t.date < range.from) return false;
    if (range.to && t.date > range.to) return false;
    return true;
  });
}

// buildReport mirrors the backend's BuildReport aggregation (see
// backend/reports.go) so the Reports page can recompute totals for any
// selected period without a round trip to the server.
export function buildReport(transactions) {
  const categoryTotals = new Map();
  const monthTotals = new Map();
  let totalIncome = 0;
  let totalExpense = 0;

  for (const t of transactions) {
    if (!categoryTotals.has(t.category)) {
      categoryTotals.set(t.category, { category: t.category, total: 0, count: 0 });
    }
    const cs = categoryTotals.get(t.category);
    cs.total += t.type === "expense" ? -t.amount : t.amount;
    cs.count += 1;

    const month = t.date.slice(0, 7);
    if (!monthTotals.has(month)) {
      monthTotals.set(month, { month, total_income: 0, total_expense: 0, net: 0 });
    }
    const ms = monthTotals.get(month);
    if (t.type === "income") {
      totalIncome += t.amount;
      ms.total_income += t.amount;
    } else {
      totalExpense += t.amount;
      ms.total_expense += t.amount;
    }
    ms.net = ms.total_income - ms.total_expense;
  }

  const by_category = [...categoryTotals.values()].sort((a, b) => a.total - b.total);
  const by_month = [...monthTotals.values()].sort((a, b) => (a.month < b.month ? -1 : 1));

  return {
    total_income: totalIncome,
    total_expense: totalExpense,
    net: totalIncome - totalExpense,
    by_category,
    by_month,
    transaction_count: transactions.length,
  };
}
