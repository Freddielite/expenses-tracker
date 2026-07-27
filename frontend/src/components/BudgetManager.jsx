import { useEffect, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Plus, X, AlertTriangle, ChevronDown } from "lucide-react";
import { formatCurrency, formatMonth } from "../format";
import { getBudgetHistory } from "../api";

// One month's bar in the expanded history strip for a budget row. Height is
// relative to the limit, capped visually at 100% so a wildly over-budget
// month doesn't blow out the row — the color and label still communicate
// "how far over" beyond that point.
function HistoryBar({ month, limit, percentUsed }) {
  const isOver = percentUsed >= 100;
  const isClose = percentUsed >= 80 && !isOver;
  const tier = isOver ? "is-over" : isClose ? "is-close" : "is-ok";
  return (
    <div
      className="budget-history__bar-wrap"
      title={`${formatMonth(month)}: ${Math.round(percentUsed)}% used (limit ${formatCurrency(limit)})`}
    >
      <div className="budget-history__track">
        <div
          className={`budget-history__fill ${tier}`}
          style={{ height: `${Math.min(percentUsed, 100)}%` }}
        />
      </div>
      <span className="budget-history__label">{formatMonth(month).split(" ")[0]}</span>
    </div>
  );
}

function BudgetHistoryStrip({ budgetId }) {
  const [months, setMonths] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    let cancelled = false;
    getBudgetHistory(6)
      .then((history) => {
        if (cancelled) return;
        const entry = (history || []).find((h) => h.id === budgetId);
        setMonths(entry?.months || []);
      })
      .catch((err) => {
        if (!cancelled) setError(err.message || "Could not load history.");
      });
    return () => {
      cancelled = true;
    };
  }, [budgetId]);

  if (error) return <p className="manage-form__error">{error}</p>;
  if (!months) return <p className="budget-history__loading">Loading history…</p>;

  return (
    <div className="budget-history">
      {months.map((m) => (
        <HistoryBar key={m.month} month={m.month} limit={m.limit} percentUsed={m.percent_used} />
      ))}
    </div>
  );
}

export default function BudgetManager({ budgetStatus, expenseCategories, onCreate, onUpdate, onDelete, readOnly = false }) {
  const [category, setCategory] = useState(expenseCategories[0]?.name || "");
  const [limit, setLimit] = useState("");
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(false);
  const [expandedId, setExpandedId] = useState(null);
  const [limitEditId, setLimitEditId] = useState(null);
  const [limitDraft, setLimitDraft] = useState("");
  const [limitError, setLimitError] = useState(null);
  const [savingLimitId, setSavingLimitId] = useState(null);

  const budgetedCategories = new Set(budgetStatus.map((b) => b.category));
  const availableCategories = expenseCategories.filter(
    (c) => !budgetedCategories.has(c.name)
  );

  async function handleSubmit(e) {
    e.preventDefault();
    setError(null);
    const amount = parseFloat(limit);
    if (!category) {
      setError("Choose a category.");
      return;
    }
    if (!amount || amount <= 0) {
      setError("Enter a monthly limit greater than 0.");
      return;
    }
    setSaving(true);
    try {
      await onCreate({ category, monthly_limit: amount });
      setLimit("");
    } catch (err) {
      setError(err.message || "Could not save budget.");
    } finally {
      setSaving(false);
    }
  }

  function startLimitEdit(b) {
    setLimitEditId(b.id);
    setLimitDraft(String(b.monthly_limit));
    setLimitError(null);
  }

  function cancelLimitEdit() {
    setLimitEditId(null);
    setLimitDraft("");
    setLimitError(null);
  }

  async function commitLimitEdit(b) {
    // A second blur can fire once the input becomes `disabled` below —
    // guard against re-entering while a save is already in flight (same
    // reasoning as the transaction inline-amount edit).
    if (savingLimitId === b.id) return;

    const value = parseFloat(limitDraft);
    if (!Number.isFinite(value) || value <= 0) {
      setLimitError("Enter a limit greater than 0.");
      return;
    }
    if (value === b.monthly_limit) {
      cancelLimitEdit();
      return;
    }
    setSavingLimitId(b.id);
    try {
      await onUpdate(b.id, { category: b.category, monthly_limit: value });
      cancelLimitEdit();
    } catch (err) {
      setLimitError(err.message || "Could not update limit.");
    } finally {
      setSavingLimitId(null);
    }
  }

  return (
    <div className="manage-section">
      <h3 className="manage-section__title">Budgets</h3>
      <p className="manage-section__subtitle">
        Set a monthly spending limit per category — tracked against the
        current calendar month. Click a limit to change it, or the arrow
        on a budget to see its last six months.
      </p>

      {budgetStatus.length === 0 ? (
        <p className="manage-group__empty">No budgets set yet.</p>
      ) : (
        <ul className="budget-list">
          <AnimatePresence initial={false}>
            {budgetStatus.map((b) => {
              const isOver = b.percent_used >= 100;
              const isClose = b.percent_used >= 80 && !isOver;
              const statusClass = isOver ? "is-over" : isClose ? "is-close" : "is-ok";
              return (
                <motion.li
                  key={b.id}
                  className={`budget-row ${statusClass}`}
                  layout
                  initial={{ opacity: 0, y: 8 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, x: -16 }}
                >
                  <div className="budget-row__top">
                    <span className="budget-row__category">
                      {(isOver || isClose) && <AlertTriangle size={13} strokeWidth={2.5} />}
                      {b.category}
                    </span>
                    <span className="mono budget-row__figures">
                      {formatCurrency(b.spent)} /{" "}
                      {limitEditId === b.id ? (
                        <input
                          type="number"
                          step="0.01"
                          min="0.01"
                          autoFocus
                          className="budget-row__limit-input mono"
                          value={limitDraft}
                          disabled={savingLimitId === b.id}
                          onChange={(e) => {
                            setLimitDraft(e.target.value);
                            setLimitError(null);
                          }}
                          onBlur={() => commitLimitEdit(b)}
                          onKeyDown={(e) => {
                            if (e.key === "Enter") {
                              e.currentTarget.blur();
                            } else if (e.key === "Escape") {
                              cancelLimitEdit();
                            }
                          }}
                        />
                      ) : readOnly ? (
                        formatCurrency(b.monthly_limit)
                      ) : (
                        <button
                          type="button"
                          className="budget-row__limit-editable"
                          onClick={() => startLimitEdit(b)}
                          aria-label={`Edit monthly limit for ${b.category}`}
                        >
                          {formatCurrency(b.monthly_limit)}
                        </button>
                      )}
                    </span>
                    <button
                      type="button"
                      className={`budget-row__history-toggle ${expandedId === b.id ? "is-open" : ""}`}
                      onClick={() => setExpandedId(expandedId === b.id ? null : b.id)}
                      aria-label={`${expandedId === b.id ? "Hide" : "Show"} history for ${b.category}`}
                      aria-expanded={expandedId === b.id}
                    >
                      <ChevronDown size={13} strokeWidth={2.5} />
                    </button>
                    {!readOnly && (
                      <button
                        type="button"
                        className="budget-row__remove"
                        onClick={() => onDelete(b.id)}
                        aria-label={`Remove budget for ${b.category}`}
                      >
                        <X size={13} strokeWidth={2.5} />
                      </button>
                    )}
                  </div>
                  {limitEditId === b.id && limitError && (
                    <p className="budget-row__limit-error">{limitError}</p>
                  )}
                  <div className="budget-bar">
                    <motion.div
                      className={`budget-bar__fill ${statusClass}`}
                      initial={{ width: 0 }}
                      animate={{ width: `${Math.min(b.percent_used, 100)}%` }}
                      transition={{ duration: 0.5, ease: "easeOut" }}
                    />
                  </div>
                  <AnimatePresence initial={false}>
                    {expandedId === b.id && (
                      <motion.div
                        initial={{ opacity: 0, height: 0 }}
                        animate={{ opacity: 1, height: "auto" }}
                        exit={{ opacity: 0, height: 0 }}
                        transition={{ duration: 0.2 }}
                        style={{ overflow: "hidden" }}
                      >
                        <BudgetHistoryStrip budgetId={b.id} />
                      </motion.div>
                    )}
                  </AnimatePresence>
                </motion.li>
              );
            })}
          </AnimatePresence>
        </ul>
      )}

      {!readOnly && availableCategories.length > 0 && (
        <form className="manage-form" onSubmit={handleSubmit}>
          <div className="manage-form__row">
            <select value={category} onChange={(e) => setCategory(e.target.value)}>
              {availableCategories.map((c) => (
                <option key={c.id} value={c.name}>
                  {c.name}
                </option>
              ))}
            </select>
            <input
              type="number"
              inputMode="decimal"
              min="0"
              step="0.01"
              placeholder="Monthly limit (₦)"
              value={limit}
              onChange={(e) => setLimit(e.target.value)}
              className="manage-form__input"
            />
          </div>

          {error && <p className="manage-form__error">{error}</p>}

          <motion.button
            type="submit"
            whileTap={{ scale: 0.97 }}
            className="manage-form__submit"
            disabled={saving}
          >
            <Plus size={15} strokeWidth={2.5} />
            {saving ? "Adding…" : "Add budget"}
          </motion.button>
        </form>
      )}
    </div>
  );
}
