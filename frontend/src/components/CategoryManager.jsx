import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Plus, X } from "lucide-react";
import { getIconComponent, ICON_OPTIONS } from "../categoryMeta";

const COLOR_SWATCHES = [
  "#b54834", "#c9862c", "#c9a227", "#1f7a5c", "#35b98a",
  "#3d7a9e", "#6b4ba1", "#a13d5c", "#a15c3d", "#3d5aa1",
  "#2f9e7a", "#6b6f76",
];

export default function CategoryManager({ categories, onCreate, onDelete, readOnly = false }) {
  const [name, setName] = useState("");
  const [type, setType] = useState("expense");
  const [color, setColor] = useState(COLOR_SWATCHES[0]);
  const [icon, setIcon] = useState(ICON_OPTIONS[0]);
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(false);

  const expenseCategories = categories.filter((c) => c.type === "expense");
  const incomeCategories = categories.filter((c) => c.type === "income");

  async function handleSubmit(e) {
    e.preventDefault();
    setError(null);
    if (!name.trim()) {
      setError("Give the category a name.");
      return;
    }
    setSaving(true);
    try {
      await onCreate({ name: name.trim(), type, color, icon });
      setName("");
    } catch (err) {
      setError(err.message || "Could not save category.");
    } finally {
      setSaving(false);
    }
  }

  function renderGroup(title, list) {
    return (
      <div className="manage-group">
        <h4 className="manage-group__title">{title}</h4>
        {list.length === 0 ? (
          <p className="manage-group__empty">No categories yet.</p>
        ) : (
          <ul className="chip-list">
            <AnimatePresence initial={false}>
              {list.map((c) => {
                const Icon = getIconComponent(c.icon);
                return (
                  <motion.li
                    key={c.id}
                    className="chip"
                    style={{ borderColor: `${c.color}55`, color: c.color }}
                    layout
                    initial={{ opacity: 0, scale: 0.9 }}
                    animate={{ opacity: 1, scale: 1 }}
                    exit={{ opacity: 0, scale: 0.9 }}
                  >
                    <Icon size={13} strokeWidth={2} />
                    {c.name}
                    {!readOnly && (
                      <button
                        type="button"
                        className="chip__remove"
                        onClick={() => onDelete(c.id)}
                        aria-label={`Delete ${c.name}`}
                      >
                        <X size={12} strokeWidth={2.5} />
                      </button>
                    )}
                  </motion.li>
                );
              })}
            </AnimatePresence>
          </ul>
        )}
      </div>
    );
  }

  return (
    <div className="manage-section">
      <h3 className="manage-section__title">Categories</h3>
      <p className="manage-section__subtitle">
        Categories used in the entry form and reports. Deleting one doesn't
        touch past transactions — they keep their original category name.
      </p>

      {renderGroup("Expense", expenseCategories)}
      {renderGroup("Income", incomeCategories)}

      {!readOnly && (
        <form className="manage-form" onSubmit={handleSubmit}>
          <div className="manage-form__row">
            <input
              type="text"
              placeholder="New category name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="manage-form__input"
            />
            <select value={type} onChange={(e) => setType(e.target.value)}>
              <option value="expense">Expense</option>
              <option value="income">Income</option>
            </select>
          </div>

          <div className="swatch-row">
            {COLOR_SWATCHES.map((c) => (
              <button
                key={c}
                type="button"
                className={`swatch ${color === c ? "is-selected" : ""}`}
                style={{ background: c }}
                onClick={() => setColor(c)}
                aria-label={`Color ${c}`}
              />
            ))}
          </div>

          <div className="icon-row">
            {ICON_OPTIONS.map((key) => {
              const Icon = getIconComponent(key);
              return (
                <button
                  key={key}
                  type="button"
                  className={`icon-choice ${icon === key ? "is-selected" : ""}`}
                  onClick={() => setIcon(key)}
                  aria-label={`Icon ${key}`}
                >
                  <Icon size={15} strokeWidth={2} />
                </button>
              );
            })}
          </div>

          {error && <p className="manage-form__error">{error}</p>}

          <motion.button
            type="submit"
            whileTap={{ scale: 0.97 }}
            className="manage-form__submit"
            disabled={saving}
          >
            <Plus size={15} strokeWidth={2.5} />
            {saving ? "Adding…" : "Add category"}
          </motion.button>
        </form>
      )}
    </div>
  );
}
