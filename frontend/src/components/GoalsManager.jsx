import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Plus, X, PiggyBank, CheckCircle2 } from "lucide-react";
import { formatCurrency, formatDate } from "../format";

export default function GoalsManager({ goals, onCreate, onDelete, onContribute, readOnly = false }) {
  const [name, setName] = useState("");
  const [targetAmount, setTargetAmount] = useState("");
  const [targetDate, setTargetDate] = useState("");
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(false);

  async function handleSubmit(e) {
    e.preventDefault();
    setError(null);
    const amount = parseFloat(targetAmount);
    if (!name.trim()) {
      setError("Give the goal a name.");
      return;
    }
    if (!amount || amount <= 0) {
      setError("Enter a target amount greater than 0.");
      return;
    }
    setSaving(true);
    try {
      await onCreate({
        name: name.trim(),
        target_amount: amount,
        target_date: targetDate || undefined,
      });
      setName("");
      setTargetAmount("");
      setTargetDate("");
    } catch (err) {
      setError(err.message || "Could not save goal.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="manage-section">
      <h3 className="manage-section__title">Savings goals</h3>
      <p className="manage-section__subtitle">
        Separate from budgets — track progress toward saving for something,
        rather than a spending limit. Add money to a goal whenever you set
        some aside.
      </p>

      {goals.length === 0 ? (
        <p className="manage-group__empty">No savings goals yet.</p>
      ) : (
        <ul className="goal-list">
          <AnimatePresence initial={false}>
            {goals.map((g) => (
              <GoalRow key={g.id} goal={g} onDelete={onDelete} onContribute={onContribute} readOnly={readOnly} />
            ))}
          </AnimatePresence>
        </ul>
      )}

      {!readOnly && (
        <form className="manage-form" onSubmit={handleSubmit}>
          <div className="manage-form__row">
            <input
              type="text"
              placeholder="Goal name (e.g. Emergency fund)"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="manage-form__input"
            />
            <input
              type="number"
              inputMode="decimal"
              min="0"
              step="0.01"
              placeholder="Target amount (₦)"
              value={targetAmount}
              onChange={(e) => setTargetAmount(e.target.value)}
              className="manage-form__input"
            />
          </div>
          <div className="manage-form__row">
            <input
              type="date"
              value={targetDate}
              onChange={(e) => setTargetDate(e.target.value)}
              className="manage-form__input"
              aria-label="Target date (optional)"
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
            {saving ? "Adding…" : "Add goal"}
          </motion.button>
        </form>
      )}
    </div>
  );
}

function GoalRow({ goal, onDelete, onContribute, readOnly = false }) {
  const [amount, setAmount] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [rowError, setRowError] = useState(null);

  const percent = goal.target_amount > 0 ? (goal.saved_amount / goal.target_amount) * 100 : 0;
  const reached = percent >= 100;

  async function handleContribute(e) {
    e.preventDefault();
    setRowError(null);
    const value = parseFloat(amount);
    if (!value) {
      setRowError("Enter an amount.");
      return;
    }
    setSubmitting(true);
    try {
      await onContribute(goal.id, value);
      setAmount("");
    } catch (err) {
      setRowError(err.message || "Could not update goal.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <motion.li
      className={`goal-row ${reached ? "is-reached" : ""}`}
      layout
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, x: -16 }}
    >
      <div className="goal-row__top">
        <span className="goal-row__name">
          {reached ? (
            <CheckCircle2 size={14} strokeWidth={2.5} />
          ) : (
            <PiggyBank size={14} strokeWidth={2} />
          )}
          {goal.name}
          {goal.target_date && (
            <span className="goal-row__date">by {formatDate(goal.target_date)}</span>
          )}
        </span>
        <span className="mono goal-row__figures">
          {formatCurrency(goal.saved_amount)} / {formatCurrency(goal.target_amount)}
        </span>
        {!readOnly && (
          <button
            type="button"
            className="goal-row__remove"
            onClick={() => onDelete(goal.id)}
            aria-label={`Remove goal ${goal.name}`}
          >
            <X size={13} strokeWidth={2.5} />
          </button>
        )}
      </div>

      <div className="goal-bar">
        <motion.div
          className="goal-bar__fill"
          initial={{ width: 0 }}
          animate={{ width: `${Math.min(percent, 100)}%` }}
          transition={{ duration: 0.5, ease: "easeOut" }}
        />
      </div>

      {!readOnly && (
        <form className="goal-row__contribute" onSubmit={handleContribute}>
          <input
            type="number"
            inputMode="decimal"
            step="0.01"
            placeholder="Add amount (₦)"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            className="goal-row__contribute-input"
          />
          <button type="submit" className="goal-row__contribute-submit" disabled={submitting}>
            {submitting ? "Adding…" : "Add"}
          </button>
        </form>
      )}
      {rowError && <p className="manage-form__error">{rowError}</p>}
    </motion.li>
  );
}
