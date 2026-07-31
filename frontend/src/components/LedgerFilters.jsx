import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Search, SlidersHorizontal, X } from "lucide-react";

export default function LedgerFilters({ categories, filters, onChange }) {
  const [panelOpen, setPanelOpen] = useState(false);

  const hasActiveFilters =
    filters.type !== "all" ||
    filters.category !== "all" ||
    filters.dateFrom ||
    filters.dateTo;

  function update(patch) {
    onChange({ ...filters, ...patch });
  }

  function clearAll() {
    onChange({ query: filters.query, type: "all", category: "all", dateFrom: "", dateTo: "" });
  }

  const categoryNames = [...new Set(categories.map((c) => c.name))].sort();

  return (
    <div className="ledger-filters">
      <div className="ledger-filters__row">
        <div className="search-input">
          <Search size={15} strokeWidth={2} />
          <input
            type="text"
            placeholder="Search description or category…"
            value={filters.query}
            onChange={(e) => update({ query: e.target.value })}
          />
          {filters.query && (
            <button
              type="button"
              className="search-input__clear"
              onClick={() => update({ query: "" })}
              aria-label="Clear search"
            >
              <X size={13} strokeWidth={2.5} />
            </button>
          )}
        </div>

        <motion.button
          type="button"
          whileTap={{ scale: 0.95 }}
          className={`filter-toggle ${hasActiveFilters ? "has-active" : ""}`}
          onClick={() => setPanelOpen((v) => !v)}
        >
          <SlidersHorizontal size={15} strokeWidth={2} />
          Filters
          {hasActiveFilters && <span className="filter-toggle__dot" />}
        </motion.button>
      </div>

      <AnimatePresence initial={false}>
        {panelOpen && (
          <motion.div
            className="filter-panel"
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.2, ease: "easeOut" }}
          >
            <div className="filter-panel__row">
              <button
                type="button"
                className={`type-toggle__btn type-toggle__btn--sm ${filters.type === "all" ? "is-active" : ""}`}
                onClick={() => update({ type: "all" })}
              >
                All
              </button>
              <button
                type="button"
                className={`type-toggle__btn type-toggle__btn--sm ${filters.type === "expense" ? "is-active is-rust" : ""}`}
                onClick={() => update({ type: "expense" })}
              >
                Expense
              </button>
              <button
                type="button"
                className={`type-toggle__btn type-toggle__btn--sm ${filters.type === "income" ? "is-active is-jade" : ""}`}
                onClick={() => update({ type: "income" })}
              >
                Income
              </button>
            </div>

            <div className="filter-panel__row">
              <select
                value={filters.category}
                onChange={(e) => update({ category: e.target.value })}
              >
                <option value="all">All categories</option>
                {categoryNames.map((name) => (
                  <option key={name} value={name}>
                    {name}
                  </option>
                ))}
              </select>
            </div>

            <div className="filter-panel__row">
              <label className="filter-panel__date">
                <span>From</span>
                <input
                  type="date"
                  value={filters.dateFrom}
                  onChange={(e) => update({ dateFrom: e.target.value })}
                />
              </label>
              <label className="filter-panel__date">
                <span>To</span>
                <input
                  type="date"
                  value={filters.dateTo}
                  onChange={(e) => update({ dateTo: e.target.value })}
                />
              </label>
            </div>

            {hasActiveFilters && (
              <button type="button" className="filter-panel__clear" onClick={clearAll}>
                Clear filters
              </button>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
