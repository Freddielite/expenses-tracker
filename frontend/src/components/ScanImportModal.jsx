import { useMemo, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { X, ScanLine, Trash2, Loader2, AlertTriangle } from "lucide-react";
import { useScreenshotOCR } from "../scanImport/useScreenshotOCR";
import { parseBankScreenshot } from "../scanImport/bankParsers";
import "./ScanImportModal.css";

const BANKS = [
  { id: "opay", label: "OPay" },
  { id: "moniepoint", label: "Moniepoint" },
];

// Full-screen review flow: pick bank -> pick screenshot -> OCR runs
// entirely in the browser -> parsed rows show up as editable drafts ->
// user confirms which ones actually get saved. Nothing is written to the
// ledger until the user hits "Add" on the review step, since OCR-off-a-
// screenshot is meaningfully less reliable than the CSV/Excel import path.
export default function ScanImportModal({ categories, onConfirm, onClose }) {
  const [bank, setBank] = useState("opay");
  const [step, setStep] = useState("pick"); // pick | scanning | review | saving
  const [drafts, setDrafts] = useState([]);
  const [saveError, setSaveError] = useState(null);
  const fileInputRef = useRef(null);
  const { recognize, status, progress, error: ocrError } = useScreenshotOCR();

  const expenseCategories = useMemo(() => categories.filter((c) => c.type === "expense"), [categories]);
  const incomeCategories = useMemo(() => categories.filter((c) => c.type === "income"), [categories]);

  const includedCount = drafts.filter((d) => d.include).length;

  async function handleFileChosen(e) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;

    setStep("scanning");
    try {
      const text = await recognize(file);
      const parsed = parseBankScreenshot(bank, text, categories);
      setDrafts(parsed);
      setStep("review");
    } catch {
      // ocrError is already set by the hook; stay on the scanning step so
      // the error banner + retry button show.
    }
  }

  function updateDraft(id, changes) {
    setDrafts((prev) => prev.map((d) => (d.id === id ? { ...d, ...changes } : d)));
  }

  function removeDraft(id) {
    setDrafts((prev) => prev.filter((d) => d.id !== id));
  }

  async function handleConfirm() {
    setSaveError(null);
    const toSave = drafts.filter((d) => d.include);
    if (toSave.length === 0) return;

    setStep("saving");
    try {
      await onConfirm(toSave);
      onClose();
    } catch (err) {
      setSaveError(err.message || "Couldn't save those entries.");
      setStep("review");
    }
  }

  return (
    <div className="scan-import__backdrop" role="dialog" aria-modal="true" aria-label="Scan screenshot">
      <motion.div
        className="scan-import__panel"
        initial={{ opacity: 0, y: 16, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        exit={{ opacity: 0, y: 16, scale: 0.98 }}
        transition={{ type: "spring", stiffness: 420, damping: 34 }}
      >
        <div className="scan-import__header">
          <div>
            <h3 className="scan-import__title">Scan a bank app screenshot</h3>
            <p className="scan-import__subtitle">
              Runs on your device — the image is never uploaded anywhere.
            </p>
          </div>
          <button type="button" className="scan-import__close" onClick={onClose} aria-label="Close">
            <X size={18} strokeWidth={2} />
          </button>
        </div>

        {step === "pick" && (
          <div className="scan-import__body">
            <p className="scan-import__label">Which app is the screenshot from?</p>
            <div className="scan-import__bank-toggle">
              {BANKS.map((b) => (
                <button
                  key={b.id}
                  type="button"
                  className={`scan-import__bank-btn${bank === b.id ? " is-active" : ""}`}
                  onClick={() => setBank(b.id)}
                >
                  {b.label}
                </button>
              ))}
            </div>

            <button
              type="button"
              className="scan-import__pick-file"
              onClick={() => fileInputRef.current?.click()}
            >
              <ScanLine size={18} strokeWidth={2} />
              Choose screenshot
            </button>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              onChange={handleFileChosen}
              style={{ display: "none" }}
            />
            <p className="scan-import__hint">
              Works best with a clear, uncropped screenshot of a transaction list.
              You'll get a chance to review and edit everything it finds before
              anything is saved.
            </p>
          </div>
        )}

        {step === "scanning" && (
          <div className="scan-import__body scan-import__body--centered">
            {status === "error" ? (
              <>
                <AlertTriangle size={28} strokeWidth={1.75} className="scan-import__error-icon" />
                <p className="scan-import__label">{ocrError}</p>
                <button type="button" className="scan-import__pick-file" onClick={() => setStep("pick")}>
                  Try another screenshot
                </button>
              </>
            ) : (
              <>
                <Loader2 size={28} strokeWidth={2} className="scan-import__spinner" />
                <p className="scan-import__label">Reading the screenshot…</p>
                <div className="scan-import__progress-track">
                  <div
                    className="scan-import__progress-fill"
                    style={{ width: `${Math.round(progress * 100)}%` }}
                  />
                </div>
              </>
            )}
          </div>
        )}

        {(step === "review" || step === "saving") && (
          <div className="scan-import__body">
            {drafts.length === 0 ? (
              <div className="scan-import__body--centered">
                <p className="scan-import__label">
                  Couldn't find any transactions in that screenshot.
                </p>
                <button type="button" className="scan-import__pick-file" onClick={() => setStep("pick")}>
                  Try another screenshot
                </button>
              </div>
            ) : (
              <>
                <p className="scan-import__label">
                  Found {drafts.length} possible transaction{drafts.length === 1 ? "" : "s"} — check
                  each one before adding.
                </p>
                <div className="scan-import__rows">
                  <AnimatePresence initial={false}>
                    {drafts.map((d) => {
                      const activeCategories = d.type === "income" ? incomeCategories : expenseCategories;
                      return (
                        <motion.div
                          key={d.id}
                          className={`scan-import__row${d.include ? "" : " is-excluded"}`}
                          initial={{ opacity: 0, height: 0 }}
                          animate={{ opacity: 1, height: "auto" }}
                          exit={{ opacity: 0, height: 0 }}
                          layout
                        >
                          <div className="scan-import__row-top">
                            <label className="scan-import__checkbox">
                              <input
                                type="checkbox"
                                checked={d.include}
                                onChange={(e) => updateDraft(d.id, { include: e.target.checked })}
                              />
                            </label>
                            <input
                              type="text"
                              className="scan-import__desc-input"
                              value={d.description}
                              onChange={(e) => updateDraft(d.id, { description: e.target.value })}
                              placeholder="Description"
                            />
                            <button
                              type="button"
                              className="scan-import__remove"
                              onClick={() => removeDraft(d.id)}
                              aria-label="Discard row"
                            >
                              <Trash2 size={14} strokeWidth={2} />
                            </button>
                          </div>
                          <div className="scan-import__row-fields">
                            <div className="scan-import__type-toggle">
                              <button
                                type="button"
                                className={`scan-import__type-btn${d.type === "expense" ? " is-active is-rust" : ""}`}
                                onClick={() =>
                                  updateDraft(d.id, {
                                    type: "expense",
                                    category: expenseCategories[0]?.name || "",
                                  })
                                }
                              >
                                Expense
                              </button>
                              <button
                                type="button"
                                className={`scan-import__type-btn${d.type === "income" ? " is-active is-jade" : ""}`}
                                onClick={() =>
                                  updateDraft(d.id, {
                                    type: "income",
                                    category: incomeCategories[0]?.name || "",
                                  })
                                }
                              >
                                Income
                              </button>
                            </div>
                            <input
                              type="number"
                              step="0.01"
                              className="scan-import__amount-input"
                              value={d.amount}
                              onChange={(e) => updateDraft(d.id, { amount: e.target.value })}
                            />
                            <select
                              className="scan-import__category-select"
                              value={d.category}
                              onChange={(e) => updateDraft(d.id, { category: e.target.value })}
                            >
                              <option value="">Category…</option>
                              {activeCategories.map((c) => (
                                <option key={c.name} value={c.name}>
                                  {c.name}
                                </option>
                              ))}
                            </select>
                            <input
                              type="date"
                              className="scan-import__date-input"
                              value={d.date}
                              onChange={(e) => updateDraft(d.id, { date: e.target.value })}
                            />
                          </div>
                        </motion.div>
                      );
                    })}
                  </AnimatePresence>
                </div>

                {saveError && <p className="scan-import__error-text">{saveError}</p>}

                <div className="scan-import__footer">
                  <button type="button" className="scan-import__secondary" onClick={() => setStep("pick")}>
                    Scan another
                  </button>
                  <button
                    type="button"
                    className="scan-import__confirm"
                    onClick={handleConfirm}
                    disabled={includedCount === 0 || step === "saving"}
                  >
                    {step === "saving"
                      ? "Adding…"
                      : `Add ${includedCount} transaction${includedCount === 1 ? "" : "s"}`}
                  </button>
                </div>
              </>
            )}
          </div>
        )}
      </motion.div>
    </div>
  );
}
