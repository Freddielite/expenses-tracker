import { useState, useMemo } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { ArrowDownCircle, ArrowUpCircle, Check } from "lucide-react";
import { todayISO } from "../format";

function buildEmptyForm(defaultCategory) {
  return {
    type: "expense",
    amount: "",
    category: defaultCategory || "",
    description: "",
    date: todayISO(),
  };
}

export default function TransactionForm({ categories, onSubmit, submitting }) {
  const expenseCategories = useMemo(
    () => categories.filter((c) => c.type === "expense"),
    [categories]
  );
  const incomeCategories = useMemo(
    () => categories.filter((c) => c.type === "income"),
    [categories]
  );

  const [form, setForm] = useState(() =>
    buildEmptyForm(expenseCategories[0]?.name)
  );
  const [error, setError] = useState(null);
  const [justSaved, setJustSaved] = useState(false);

  const activeCategories = form.type === "income" ? incomeCategories : expenseCategories;
  const selectedMeta = activeCategories.find((c) => c.name === form.category);

  function handleTypeChange(type) {
    const list = type === "income" ? incomeCategories : expenseCategories;
    setForm((f) => ({ ...f, type, category: list[0]?.name || "" }));
  }

  async function handleSubmit(e) {
    e.preventDefault();
    setError(null);

    const amount = parseFloat(form.amount);
    if (!amount || amount <= 0) {
      setError("Enter an amount greater than 0.");
      return;
    }
    if (!form.category) {
      setError("Add a category first (see the Manage tab) before recording an entry.");
      return;
    }

    try {
      await onSubmit({
        type: form.type,
        amount,
        category: form.category,
        description: form.description.trim(),
        date: form.date,
      });
      setForm((f) => ({
        ...buildEmptyForm(f.category),
        type: f.type,
        date: f.date,
      }));
      setJustSaved(true);
      setTimeout(() => setJustSaved(false), 1400);
    } catch (err) {
      setError(err.message || "Could not save entry.");
    }
  }

  return (
    <form className="entry-form" onSubmit={handleSubmit}>
      <div className="entry-form__type-toggle">
        <motion.button
          type="button"
          whileTap={{ scale: 0.96 }}
          className={`type-toggle__btn type-toggle__btn--has-indicator ${form.type === "expense" ? "is-active is-rust" : ""}`}
          onClick={() => handleTypeChange("expense")}
        >
          <ArrowDownCircle size={16} strokeWidth={2} />
          Expense
          {form.type === "expense" && (
            <motion.span
              className="type-toggle__indicator is-rust"
              layoutId="type-toggle-indicator"
              transition={{ type: "spring", stiffness: 500, damping: 35 }}
            />
          )}
        </motion.button>
        <motion.button
          type="button"
          whileTap={{ scale: 0.96 }}
          className={`type-toggle__btn type-toggle__btn--has-indicator ${form.type === "income" ? "is-active is-jade" : ""}`}
          onClick={() => handleTypeChange("income")}
        >
          <ArrowUpCircle size={16} strokeWidth={2} />
          Income
          {form.type === "income" && (
            <motion.span
              className="type-toggle__indicator is-jade"
              layoutId="type-toggle-indicator"
              transition={{ type: "spring", stiffness: 500, damping: 35 }}
            />
          )}
        </motion.button>
      </div>

      <div className="entry-form__grid">
        <label className="field">
          <span>Amount (₦)</span>
          <input
            type="number"
            inputMode="decimal"
            min="0"
            step="0.01"
            placeholder="0.00"
            value={form.amount}
            onChange={(e) => setForm((f) => ({ ...f, amount: e.target.value }))}
            required
          />
        </label>

        <label className="field">
          <span>Category</span>
          {activeCategories.length === 0 ? (
            <div className="field__empty-hint">
              No {form.type} categories yet — add one in the Manage tab.
            </div>
          ) : (
            <div className="select-wrap">
              <span
                className="select-wrap__swatch"
                style={{ background: selectedMeta?.color || "#6b6f76" }}
              />
              <select
                value={form.category}
                onChange={(e) => setForm((f) => ({ ...f, category: e.target.value }))}
              >
                {activeCategories.map((c) => (
                  <option key={c.id} value={c.name}>
                    {c.name}
                  </option>
                ))}
              </select>
            </div>
          )}
        </label>

        <label className="field">
          <span>Date</span>
          <input
            type="date"
            value={form.date}
            onChange={(e) => setForm((f) => ({ ...f, date: e.target.value }))}
            required
          />
        </label>

        <label className="field field--wide">
          <span>Description</span>
          <input
            type="text"
            placeholder="What was this for?"
            value={form.description}
            onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
          />
        </label>
      </div>

      {error && <p className="entry-form__error">{error}</p>}

      <motion.button
        type="submit"
        whileTap={{ scale: 0.98 }}
        animate={justSaved ? { scale: [1, 1.03, 1] } : {}}
        transition={{ duration: 0.35 }}
        className={`entry-form__submit ${justSaved ? "is-success" : ""}`}
        disabled={submitting}
      >
        <AnimatePresence mode="wait" initial={false}>
          <motion.span
            key={justSaved ? "saved" : submitting ? "saving" : "idle"}
            initial={{ opacity: 0, y: 6 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -6 }}
            transition={{ duration: 0.15 }}
            style={{ display: "inline-flex", alignItems: "center", gap: 7 }}
          >
            {justSaved ? (
              <>
                <Check size={16} strokeWidth={2.5} /> Recorded
              </>
            ) : submitting ? (
              "Recording…"
            ) : (
              "Record entry"
            )}
          </motion.span>
        </AnimatePresence>
      </motion.button>
    </form>
  );
}
