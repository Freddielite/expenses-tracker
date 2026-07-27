import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Trash2, Pencil, Inbox } from "lucide-react";
import { formatCurrency, formatDate } from "../format";
import { getCategoryMeta } from "../categoryMeta";
import EditTransactionRow from "./EditTransactionRow";

export default function TransactionList({
  transactions,
  categories,
  onDelete,
  onUpdate,
  onPatchAmount,
  deletingId,
  isFiltering = false,
  totalCount,
  readOnly = false,
}) {
  const [editingId, setEditingId] = useState(null);
  const [savingId, setSavingId] = useState(null);
  const [amountEditId, setAmountEditId] = useState(null);
  const [amountDraft, setAmountDraft] = useState("");
  const [amountError, setAmountError] = useState(null);
  const [savingAmountId, setSavingAmountId] = useState(null);

  if (transactions.length === 0) {
    return (
      <motion.div
        className="empty-state"
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
      >
        <div className="empty-state__icon">
          <Inbox size={28} strokeWidth={1.5} />
        </div>
        {isFiltering ? (
          <>
            <p className="empty-state__title">No matches</p>
            <p className="empty-state__body">
              Nothing in your {totalCount} entries matches these filters —
              try widening the date range or clearing a filter.
            </p>
          </>
        ) : (
          <>
            <p className="empty-state__title">No entries yet</p>
            <p className="empty-state__body">
              Record your first income or expense above — it'll show up here
              as a line in the ledger.
            </p>
          </>
        )}
      </motion.div>
    );
  }

  async function handleSave(id, updated) {
    setSavingId(id);
    try {
      await onUpdate(id, updated);
      setEditingId(null);
    } finally {
      setSavingId(null);
    }
  }

  function startAmountEdit(t) {
    setAmountEditId(t.id);
    setAmountDraft(String(t.amount));
    setAmountError(null);
  }

  function cancelAmountEdit() {
    setAmountEditId(null);
    setAmountDraft("");
    setAmountError(null);
  }

  async function commitAmountEdit(t) {
    // A second blur can fire once the input becomes `disabled` below —
    // guard against re-entering while a save is already in flight.
    if (savingAmountId === t.id) return;

    const value = parseFloat(amountDraft);
    if (!Number.isFinite(value) || value <= 0) {
      setAmountError("Enter an amount greater than 0.");
      return;
    }
    if (value === t.amount) {
      cancelAmountEdit();
      return;
    }
    setSavingAmountId(t.id);
    try {
      await onPatchAmount(t.id, value);
      cancelAmountEdit();
    } catch (err) {
      setAmountError(err.message || "Could not update amount.");
    } finally {
      setSavingAmountId(null);
    }
  }

  return (
    <>
      {isFiltering && (
        <p className="ledger-filters__count">
          Showing {transactions.length} of {totalCount} entries
        </p>
      )}
      <ul className="ledger-list">
        <AnimatePresence initial={false}>
          {transactions.map((t, i) => {
            if (editingId === t.id) {
              return (
                <motion.li key={t.id} className="ledger-line ledger-line--editing" layout>
                  <EditTransactionRow
                    transaction={t}
                    categories={categories}
                    onSave={(updated) => handleSave(t.id, updated)}
                    onCancel={() => setEditingId(null)}
                    saving={savingId === t.id}
                  />
                </motion.li>
              );
            }

            const meta = getCategoryMeta(categories, t.category);
            const Icon = meta.icon;
            return (
              <motion.li
                key={t.id}
                className="ledger-line"
                style={{ "--accent": meta.color }}
                layout
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, x: -24, height: 0, marginBottom: 0, paddingTop: 0, paddingBottom: 0 }}
                transition={{ duration: 0.25, delay: Math.min(i, 10) * 0.03, ease: "easeOut" }}
              >
                <div
                  className="ledger-line__icon"
                  style={{ background: `${meta.color}1a`, color: meta.color }}
                >
                  <Icon size={16} strokeWidth={2} />
                </div>

                <div className="ledger-line__body">
                  <div className="ledger-line__main">
                    <span className="ledger-line__desc">
                      {t.description || t.category}
                    </span>
                    <span className="ledger-line__leader" aria-hidden="true" />
                    {amountEditId === t.id ? (
                      <input
                        type="number"
                        step="0.01"
                        min="0.01"
                        autoFocus
                        className="ledger-line__amount-input mono"
                        value={amountDraft}
                        disabled={savingAmountId === t.id}
                        onChange={(e) => {
                          setAmountDraft(e.target.value);
                          setAmountError(null);
                        }}
                        onBlur={() => commitAmountEdit(t)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") {
                            e.currentTarget.blur();
                          } else if (e.key === "Escape") {
                            cancelAmountEdit();
                          }
                        }}
                      />
                    ) : readOnly ? (
                      <span
                        className={`ledger-line__amount mono ${
                          t.type === "expense" ? "is-negative" : "is-positive"
                        }`}
                      >
                        {t.type === "expense" ? "−" : "+"}
                        {formatCurrency(t.amount)}
                      </span>
                    ) : (
                      <button
                        type="button"
                        className={`ledger-line__amount mono ledger-line__amount--editable ${
                          t.type === "expense" ? "is-negative" : "is-positive"
                        }`}
                        onClick={() => startAmountEdit(t)}
                        aria-label={`Edit amount for ${t.description || t.category}`}
                      >
                        {t.type === "expense" ? "−" : "+"}
                        {formatCurrency(t.amount)}
                      </button>
                    )}
                  </div>
                  {amountEditId === t.id && amountError && (
                    <p className="ledger-line__amount-error">{amountError}</p>
                  )}
                  <div className="ledger-line__meta">
                    <span
                      className="pill"
                      style={{ borderColor: `${meta.color}55`, color: meta.color }}
                    >
                      {t.category}
                    </span>
                    <span>{formatDate(t.date)}</span>
                    {!readOnly && (
                      <div className="ledger-line__actions">
                        <motion.button
                          type="button"
                          className="ledger-line__edit"
                          onClick={() => setEditingId(t.id)}
                          whileTap={{ scale: 0.9 }}
                          aria-label={`Edit entry: ${t.description || t.category}`}
                        >
                          <Pencil size={13} strokeWidth={2} />
                          Edit
                        </motion.button>
                        <motion.button
                          type="button"
                          className="ledger-line__delete"
                          onClick={() => onDelete(t.id)}
                          disabled={deletingId === t.id}
                          whileTap={{ scale: 0.9 }}
                          aria-label={`Delete entry: ${t.description || t.category}`}
                        >
                          <Trash2 size={13} strokeWidth={2} />
                          {deletingId === t.id ? "Removing…" : "Remove"}
                        </motion.button>
                      </div>
                    )}
                  </div>
                </div>
              </motion.li>
            );
          })}
        </AnimatePresence>
      </ul>
    </>
  );
}
