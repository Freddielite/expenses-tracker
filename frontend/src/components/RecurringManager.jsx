import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Plus, X, Repeat } from "lucide-react";
import { formatCurrency, formatDate, todayISO } from "../format";

export default function RecurringManager({ rules, categories, onCreate, onDelete, readOnly = false }) {
  const [type, setType] = useState("expense");
  const [amount, setAmount] = useState("");
  const [category, setCategory] = useState("");
  const [description, setDescription] = useState("");
  const [frequency, setFrequency] = useState("monthly");
  const [startDate, setStartDate] = useState(todayISO());
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(false);

  const activeCategories = categories.filter((c) => c.type === type);

  async function handleSubmit(e) {
    e.preventDefault();
    setError(null);
    const amt = parseFloat(amount);
    const cat = category || activeCategories[0]?.name;
    if (!amt || amt <= 0) {
      setError("Enter an amount greater than 0.");
      return;
    }
    if (!cat) {
      setError("Add a category for this type first (see Categories above).");
      return;
    }
    setSaving(true);
    try {
      await onCreate({
        type,
        amount: amt,
        category: cat,
        description: description.trim(),
        frequency,
        start_date: startDate,
      });
      setAmount("");
      setDescription("");
    } catch (err) {
      setError(err.message || "Could not save recurring rule.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="manage-section">
      <h3 className="manage-section__title">Recurring entries</h3>
      <p className="manage-section__subtitle">
        Rules generate real ledger entries automatically — checked whenever
        the app starts, and every hour while it stays open, so it catches up
        even if your computer was off.
      </p>

      {rules.length === 0 ? (
        <p className="manage-group__empty">No recurring entries yet.</p>
      ) : (
        <ul className="recurring-list">
          <AnimatePresence initial={false}>
            {rules.map((r) => (
              <motion.li
                key={r.id}
                className="recurring-row"
                layout
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, x: -16 }}
              >
                <div className="recurring-row__icon">
                  <Repeat size={15} strokeWidth={2} />
                </div>
                <div className="recurring-row__body">
                  <div className="recurring-row__main">
                    <span>{r.description || r.category}</span>
                    <span
                      className={`mono ${r.type === "expense" ? "is-negative" : "is-positive"}`}
                    >
                      {r.type === "expense" ? "−" : "+"}
                      {formatCurrency(r.amount)}
                    </span>
                  </div>
                  <div className="recurring-row__meta">
                    <span className="pill">{r.category}</span>
                    <span className="pill">
                      {r.frequency === "weekly" ? "Weekly" : "Monthly"}
                    </span>
                    <span>Next: {formatDate(r.next_due)}</span>
                  </div>
                </div>
                {!readOnly && (
                  <button
                    type="button"
                    className="recurring-row__remove"
                    onClick={() => onDelete(r.id)}
                    aria-label={`Delete recurring entry: ${r.description || r.category}`}
                  >
                    <X size={14} strokeWidth={2.5} />
                  </button>
                )}
              </motion.li>
            ))}
          </AnimatePresence>
        </ul>
      )}

      {!readOnly && (
        <form className="manage-form" onSubmit={handleSubmit}>
          <div className="entry-form__type-toggle">
            <button
              type="button"
              className={`type-toggle__btn ${type === "expense" ? "is-active is-rust" : ""}`}
              onClick={() => {
                setType("expense");
                setCategory("");
              }}
            >
              Expense
            </button>
            <button
              type="button"
              className={`type-toggle__btn ${type === "income" ? "is-active is-jade" : ""}`}
              onClick={() => {
                setType("income");
                setCategory("");
              }}
            >
              Income
            </button>
          </div>

          <div className="manage-form__row">
            <input
              type="number"
              inputMode="decimal"
              min="0"
              step="0.01"
              placeholder="Amount (₦)"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              className="manage-form__input"
            />
            <select
              value={category || activeCategories[0]?.name || ""}
              onChange={(e) => setCategory(e.target.value)}
            >
              {activeCategories.map((c) => (
                <option key={c.id} value={c.name}>
                  {c.name}
                </option>
              ))}
            </select>
          </div>

          <div className="manage-form__row">
            <select value={frequency} onChange={(e) => setFrequency(e.target.value)}>
              <option value="monthly">Monthly</option>
              <option value="weekly">Weekly</option>
            </select>
            <input
              type="date"
              value={startDate}
              onChange={(e) => setStartDate(e.target.value)}
            />
          </div>

          <input
            type="text"
            placeholder="Description (e.g. Rent, Netflix)"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="manage-form__input manage-form__input--full"
          />

          {error && <p className="manage-form__error">{error}</p>}

          <motion.button
            type="submit"
            whileTap={{ scale: 0.97 }}
            className="manage-form__submit"
            disabled={saving}
          >
            <Plus size={15} strokeWidth={2.5} />
            {saving ? "Adding…" : "Add recurring entry"}
          </motion.button>
        </form>
      )}
    </div>
  );
}
