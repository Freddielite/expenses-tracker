import { useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Download, Upload, FileSpreadsheet, LayoutDashboard, ChevronDown, ScanLine } from "lucide-react";
import CategoryManager from "./CategoryManager";
import BudgetManager from "./BudgetManager";
import RecurringManager from "./RecurringManager";
import GoalsManager from "./GoalsManager";
import ChangePinSection from "./ChangePinSection";
import GuestAccessSection from "./GuestAccessSection";
import ScanImportModal from "./ScanImportModal";
import { downloadExportCSV, downloadExportXLSX, downloadExportHTML } from "../api";

export default function Manage({
  categories,
  budgetStatus,
  recurringRules,
  goals,
  onCreateCategory,
  onDeleteCategory,
  onCreateBudget,
  onUpdateBudget,
  onDeleteBudget,
  onCreateRecurring,
  onDeleteRecurring,
  onCreateGoal,
  onDeleteGoal,
  onContributeGoal,
  onImportFile,
  onScanImportConfirm,
  readOnly = false,
}) {
  const [exportingFormat, setExportingFormat] = useState(null);
  const [exportError, setExportError] = useState(null);
  const [exportMenuOpen, setExportMenuOpen] = useState(false);
  const [importing, setImporting] = useState(false);
  const [importError, setImportError] = useState(null);
  const [scanModalOpen, setScanModalOpen] = useState(false);
  const fileInputRef = useRef(null);
  const exportMenuRef = useRef(null);
  const expenseCategories = categories.filter((c) => c.type === "expense");

  const EXPORTERS = {
    csv: { fn: downloadExportCSV, label: "CSV", hint: "Plain spreadsheet-ready file", icon: Download },
    xlsx: { fn: downloadExportXLSX, label: "Excel workbook", hint: "Summary, ledger, and breakdown sheets", icon: FileSpreadsheet },
    html: { fn: downloadExportHTML, label: "Interactive HTML report", hint: "Charts and a filterable ledger, works offline", icon: LayoutDashboard },
  };

  useEffect(() => {
    if (!exportMenuOpen) return;
    function handleOutsideEvent(e) {
      if (e.key === "Escape") {
        setExportMenuOpen(false);
        return;
      }
      if (e.type === "mousedown" && exportMenuRef.current && !exportMenuRef.current.contains(e.target)) {
        setExportMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", handleOutsideEvent);
    document.addEventListener("keydown", handleOutsideEvent);
    return () => {
      document.removeEventListener("mousedown", handleOutsideEvent);
      document.removeEventListener("keydown", handleOutsideEvent);
    };
  }, [exportMenuOpen]);

  async function handleExport(format) {
    setExportMenuOpen(false);
    setExportError(null);
    setExportingFormat(format);
    try {
      await EXPORTERS[format].fn();
    } catch (err) {
      setExportError(err.message || "Export failed.");
    } finally {
      setExportingFormat(null);
    }
  }

  function handleImportClick() {
    fileInputRef.current?.click();
  }

  async function handleFileChosen(e) {
    const file = e.target.files?.[0];
    // Clear the input immediately so choosing the exact same file again
    // later still fires a change event.
    e.target.value = "";
    if (!file) return;

    setImportError(null);
    setImporting(true);
    try {
      await onImportFile(file);
    } catch (err) {
      setImportError(err.message || "Import failed.");
    } finally {
      setImporting(false);
    }
  }

  return (
    <div className="manage">
      {!readOnly && (
        <>
          <div className="export-menu" ref={exportMenuRef}>
            <motion.button
              type="button"
              onClick={() => setExportMenuOpen((open) => !open)}
              className="export-button"
              whileTap={{ scale: 0.97 }}
              disabled={exportingFormat !== null}
              aria-haspopup="true"
              aria-expanded={exportMenuOpen}
            >
              <Download size={15} strokeWidth={2} />
              {exportingFormat ? "Preparing download…" : "Export transactions"}
              <ChevronDown size={14} strokeWidth={2} className={`export-menu__chevron${exportMenuOpen ? " export-menu__chevron--open" : ""}`} />
            </motion.button>
            <AnimatePresence>
              {exportMenuOpen && (
                <motion.div
                  className="export-menu__panel"
                  initial={{ opacity: 0, y: -6 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -6 }}
                  transition={{ duration: 0.15 }}
                  role="menu"
                >
                  {Object.entries(EXPORTERS).map(([format, { label, hint, icon: Icon }]) => (
                    <button
                      key={format}
                      type="button"
                      role="menuitem"
                      className="export-menu__item"
                      onClick={() => handleExport(format)}
                    >
                      <Icon size={16} strokeWidth={2} />
                      <span>
                        <span className="export-menu__item-label">{label}</span>
                        <span className="export-menu__item-hint">{hint}</span>
                      </span>
                    </button>
                  ))}
                </motion.div>
              )}
            </AnimatePresence>
          </div>
          {exportError && <p className="manage-form__error">{exportError}</p>}

          <motion.button
            type="button"
            onClick={handleImportClick}
            className="export-button"
            whileTap={{ scale: 0.97 }}
            disabled={importing}
          >
            <Upload size={15} strokeWidth={2} />
            {importing ? "Importing…" : "Import transactions"}
          </motion.button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".csv,.xlsx,.xls,.pdf,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.ms-excel,application/pdf"
            onChange={handleFileChosen}
            style={{ display: "none" }}
          />
          <p className="manage-form__hint">CSV, Excel (.xlsx), or PDF statements.</p>
          {importError && <p className="manage-form__error">{importError}</p>}

          <motion.button
            type="button"
            onClick={() => setScanModalOpen(true)}
            className="export-button"
            whileTap={{ scale: 0.97 }}
          >
            <ScanLine size={15} strokeWidth={2} />
            Scan screenshot
          </motion.button>
          <p className="manage-form__hint">
            OPay or Moniepoint transaction list — reads on your device, you review before saving.
          </p>

          <AnimatePresence>
            {scanModalOpen && (
              <ScanImportModal
                categories={categories}
                onConfirm={onScanImportConfirm}
                onClose={() => setScanModalOpen(false)}
              />
            )}
          </AnimatePresence>
        </>
      )}

      <CategoryManager
        categories={categories}
        onCreate={onCreateCategory}
        onDelete={onDeleteCategory}
        readOnly={readOnly}
      />

      <BudgetManager
        budgetStatus={budgetStatus}
        expenseCategories={expenseCategories}
        onCreate={onCreateBudget}
        onUpdate={onUpdateBudget}
        onDelete={onDeleteBudget}
        readOnly={readOnly}
      />

      <GoalsManager
        goals={goals}
        onCreate={onCreateGoal}
        onDelete={onDeleteGoal}
        onContribute={onContributeGoal}
        readOnly={readOnly}
      />

      <RecurringManager
        rules={recurringRules}
        categories={categories}
        onCreate={onCreateRecurring}
        onDelete={onDeleteRecurring}
        readOnly={readOnly}
      />

      {!readOnly && (
        <>
          <ChangePinSection />
          <GuestAccessSection />
        </>
      )}
    </div>
  );
}
