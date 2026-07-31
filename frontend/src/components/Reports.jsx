import { useMemo, useState } from "react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
  Cell,
  LineChart,
  Line,
  Legend,
} from "recharts";
import { PieChart as PieChartIcon, CalendarRange } from "lucide-react";
import { formatCurrency, formatMonth, formatDate, todayISO } from "../format";
import { PERIODS, getPeriodRange, filterTransactionsByRange, buildReport } from "../reportUtils";

function budgetFlag(percentUsed) {
  if (percentUsed >= 100) return "over";
  if (percentUsed >= 80) return "close";
  return null;
}

function CategoryAxisTick({ x, y, payload, budgetByCategory }) {
  const budget = budgetByCategory?.get(payload.value);
  const flag = budget ? budgetFlag(budget.percent_used) : null;
  const color = flag === "over" ? "var(--rust)" : flag === "close" ? "var(--gold)" : "var(--ink)";
  return (
    <g transform={`translate(${x},${y})`}>
      <text x={-8} y={0} dy={4} textAnchor="end" fontSize={12} fill={color} fontWeight={flag ? 600 : 400}>
        {flag === "over" ? "⚠ " : ""}
        {payload.value}
      </text>
    </g>
  );
}

function CategoryTooltip({ active, payload, budgetByCategory }) {
  if (!active || !payload?.length) return null;
  const item = payload[0].payload;
  const budget = budgetByCategory?.get(item.category);
  const flag = budget ? budgetFlag(budget.percent_used) : null;
  return (
    <div className="chart-tooltip">
      <strong>{item.category}</strong>
      <div className="mono">{formatCurrency(item.total)}</div>
      <div className="chart-tooltip__sub">
        {item.count} {item.count === 1 ? "entry" : "entries"}
      </div>
      {budget && (
        <div className={`chart-tooltip__budget${flag ? ` is-${flag}` : ""}`}>
          Budget: {Math.round(budget.percent_used)}% used ({formatCurrency(budget.spent)} of{" "}
          {formatCurrency(budget.monthly_limit)})
        </div>
      )}
    </div>
  );
}

function MonthTooltip({ active, payload, label }) {
  if (!active || !payload?.length) return null;
  return (
    <div className="chart-tooltip">
      <strong>{formatMonth(label)}</strong>
      {payload.map((p) => (
        <div key={p.dataKey} className="mono" style={{ color: p.color }}>
          {p.name}: {formatCurrency(p.value)}
        </div>
      ))}
    </div>
  );
}

export default function Reports({ transactions, budgetStatus }) {
  const [period, setPeriod] = useState("this-month");
  const [customFrom, setCustomFrom] = useState(() => todayISO());
  const [customTo, setCustomTo] = useState(() => todayISO());

  const range = useMemo(
    () => getPeriodRange(period, { from: customFrom, to: customTo }),
    [period, customFrom, customTo]
  );

  const periodTransactions = useMemo(
    () => filterTransactionsByRange(transactions, range),
    [transactions, range]
  );

  const report = useMemo(() => buildReport(periodTransactions), [periodTransactions]);

  const budgetByCategory = useMemo(() => {
    const map = new Map();
    for (const b of budgetStatus || []) map.set(b.category, b);
    return map;
  }, [budgetStatus]);

  // Budgets are tracked against the current calendar month (see BudgetManager),
  // so overlaying budget context only makes sense when the report window is
  // exactly "this month" — otherwise spend-so-far and the monthly limit
  // wouldn't be describing the same window.
  const showBudgets = period === "this-month" && budgetByCategory.size > 0;

  if (!transactions || transactions.length === 0) {
    return (
      <div className="empty-state">
        <div className="empty-state__icon">
          <PieChartIcon size={28} strokeWidth={1.5} />
        </div>
        <p className="empty-state__title">Nothing to summarize yet</p>
        <p className="empty-state__body">
          Reports fill in once you've logged a few entries.
        </p>
      </div>
    );
  }

  const categoryData = report.by_category.map((c) => ({
    ...c,
    displayTotal: Math.abs(c.total),
  }));

  const rangeLabel =
    period === "all-time"
      ? "all time"
      : `${formatDate(range.from)} – ${formatDate(range.to)}`;

  return (
    <div className="reports">
      <div className="period-select">
        <div className="period-select__row">
          {PERIODS.map((p) => (
            <button
              key={p.id}
              type="button"
              className={`period-select__btn${period === p.id ? " is-active" : ""}`}
              onClick={() => setPeriod(p.id)}
            >
              {p.label}
            </button>
          ))}
        </div>
        {period === "custom" && (
          <div className="period-select__custom">
            <input
              type="date"
              value={customFrom}
              max={customTo}
              onChange={(e) => setCustomFrom(e.target.value)}
            />
            <span>to</span>
            <input
              type="date"
              value={customTo}
              min={customFrom}
              max={todayISO()}
              onChange={(e) => setCustomTo(e.target.value)}
            />
          </div>
        )}
        <p className="period-select__summary">
          <CalendarRange size={13} strokeWidth={2} />
          {report.transaction_count} {report.transaction_count === 1 ? "entry" : "entries"} ·{" "}
          {rangeLabel} · net{" "}
          <span className={report.net >= 0 ? "is-jade" : "is-rust"}>{formatCurrency(report.net)}</span>
        </p>
      </div>

      {report.transaction_count === 0 ? (
        <div className="empty-state">
          <div className="empty-state__icon">
            <PieChartIcon size={28} strokeWidth={1.5} />
          </div>
          <p className="empty-state__title">No entries in this window</p>
          <p className="empty-state__body">Try a wider period to see a report.</p>
        </div>
      ) : (
        <>
          <section className="report-card">
            <h3 className="report-card__title">By category</h3>
            <p className="report-card__subtitle">
              Net effect of each category — bars to the right of zero are net income, left of zero
              is net spend.
              {showBudgets && " ⚠ marks a category at or near its monthly budget."}
            </p>
            <ResponsiveContainer width="100%" height={Math.max(220, categoryData.length * 40)}>
              <BarChart
                data={categoryData}
                layout="vertical"
                margin={{ top: 4, right: 24, left: 8, bottom: 4 }}
              >
                <CartesianGrid strokeDasharray="3 3" stroke="var(--line)" horizontal={false} />
                <XAxis
                  type="number"
                  tickFormatter={(v) => formatCurrency(v)}
                  tick={{ fontSize: 11, fill: "var(--ink-soft)" }}
                  stroke="var(--line)"
                />
                <YAxis
                  type="category"
                  dataKey="category"
                  width={110}
                  tick={<CategoryAxisTick budgetByCategory={showBudgets ? budgetByCategory : null} />}
                  stroke="var(--line)"
                />
                <Tooltip
                  content={<CategoryTooltip budgetByCategory={showBudgets ? budgetByCategory : null} />}
                />
                <Bar dataKey="total" radius={[0, 4, 4, 0]}>
                  {categoryData.map((entry, i) => (
                    <Cell key={i} fill={entry.total < 0 ? "var(--rust)" : "var(--jade)"} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </section>

          <section className="report-card">
            <h3 className="report-card__title">Income vs. expense by month</h3>
            <p className="report-card__subtitle">
              Track how your inflow and outflow trend over the selected period.
            </p>
            <ResponsiveContainer width="100%" height={260}>
              <LineChart data={report.by_month} margin={{ top: 8, right: 24, left: 8, bottom: 4 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--line)" />
                <XAxis
                  dataKey="month"
                  tickFormatter={formatMonth}
                  tick={{ fontSize: 11, fill: "var(--ink-soft)" }}
                  stroke="var(--line)"
                />
                <YAxis
                  tickFormatter={(v) => formatCurrency(v)}
                  tick={{ fontSize: 11, fill: "var(--ink-soft)" }}
                  stroke="var(--line)"
                  width={80}
                />
                <Tooltip content={<MonthTooltip />} />
                <Legend wrapperStyle={{ fontSize: 12, fontFamily: "var(--font-ui)" }} />
                <Line
                  type="monotone"
                  dataKey="total_income"
                  name="Income"
                  stroke="var(--jade)"
                  strokeWidth={2}
                  dot={{ r: 3 }}
                />
                <Line
                  type="monotone"
                  dataKey="total_expense"
                  name="Expense"
                  stroke="var(--rust)"
                  strokeWidth={2}
                  dot={{ r: 3 }}
                />
              </LineChart>
            </ResponsiveContainer>
          </section>
        </>
      )}
    </div>
  );
}
