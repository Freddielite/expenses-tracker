import { useState } from "react";
import { motion } from "framer-motion";
import { Check, X } from "lucide-react";

export default function EditTransactionRow({ transaction, categories, onSave, onCancel, saving }) {
  const [type, setType] = useState(transaction.type);
  const [amount, setAmount] = useState(String(transaction.amount));
  const [category, setCategory] = useState(transaction.category);
  const [description, setDescription] = useState(transaction.description || "");
  const [date, setDate] = useState(transaction.date);
  const [error, setError] = useState(null);

  const options = categories.filter((c) => c.type === type);

  async function handleSave(e) {
    e.preventDefault();
    setError(null);
    const amt = parseFloat(amount);
    if (!amt || amt <= 0) {
      setError("Amount must be greater than 0.");
      return;
    }
    if (!category) {
      setError("Choose a category.");
      return;
    }
    try {
      await onSave({
        type,
        amount: amt,
        category,
        description: description.trim(),
        date,
      });
    } catch (err) {
      setError(err.message || "Could not save changes.");
    }
  }

  return (
    <motion.form
      className="edit-row"
      onSubmit={handleSave}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      layout
    >
      <div className="edit-row__type-toggle">
        <button
          type="button"
          className={`type-toggle__btn type-toggle__btn--sm ${type === "expense" ? "is-active is-rust" : ""}`}
          onClick={() => {
            setType("expense");
            const first = categories.find((c) => c.type === "expense");
            if (first) setCategory(first.name);
          }}
        >
          Expense
        </button>
        <button
          type="button"
          className={`type-toggle__btn type-toggle__btn--sm ${type === "income" ? "is-active is-jade" : ""}`}
          onClick={() => {
            setType("income");
            const first = categories.find((c) => c.type === "income");
            if (first) setCategory(first.name);
          }}
        >
          Income
        </button>
      </div>

      <div className="edit-row__grid">
        <input
          type="number"
          inputMode="decimal"
          min="0"
          step="0.01"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          placeholder="Amount"
        />
        <select value={category} onChange={(e) => setCategory(e.target.value)}>
          {options.map((c) => (
            <option key={c.id} value={c.name}>
              {c.name}
            </option>
          ))}
        </select>
        <input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
        <input
          type="text"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="Description"
        />
      </div>

      {error && <p className="edit-row__error">{error}</p>}

      <div className="edit-row__actions">
        <motion.button
          type="submit"
          whileTap={{ scale: 0.96 }}
          className="edit-row__save"
          disabled={saving}
        >
          <Check size={14} strokeWidth={2.5} />
          {saving ? "Saving…" : "Save"}
        </motion.button>
        <motion.button
          type="button"
          whileTap={{ scale: 0.96 }}
          className="edit-row__cancel"
          onClick={onCancel}
          disabled={saving}
        >
          <X size={14} strokeWidth={2.5} />
          Cancel
        </motion.button>
      </div>
    </motion.form>
  );
}
