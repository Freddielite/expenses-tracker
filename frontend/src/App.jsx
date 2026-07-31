import { useEffect, useRef, useState, useCallback, useMemo } from "react";
import "./App.css";
import { motion, AnimatePresence } from "framer-motion";
import { BookOpen, PieChart, Settings2, Loader2 } from "lucide-react";
import BalanceHeader from "./components/BalanceHeader";
import TransactionForm from "./components/TransactionForm";
import LedgerFilters from "./components/LedgerFilters";
import TransactionList from "./components/TransactionList";
import Reports from "./components/Reports";
import Manage from "./components/Manage";
import LockScreen from "./components/LockScreen";
import RegisterForm from "./components/RegisterForm";
import LoginForm from "./components/LoginForm";
import ToastStack from "./components/ToastStack";
import { useTheme } from "./useTheme";
import { useAuth } from "./useAuth";
import { useNotifications } from "./useNotifications";
import { formatCurrency } from "./format";
import {
  setForbiddenHandler,
  listTransactions,
  createTransaction,
  updateTransaction,
  patchTransaction,
  deleteTransaction,
  getReport,
  listCategories,
  createCategory,
  deleteCategory,
  getBudgetStatus,
  createBudget,
  updateBudget,
  deleteBudget,
  listRecurring,
  createRecurring,
  deleteRecurring,
  listGoals,
  createGoal,
  deleteGoal,
  contributeGoal,
  importTransactionsFile,
} from "./api";

// Budget usage thresholds that trigger a notification once crossed.
const BUDGET_WARN_PCT = 80;
const BUDGET_OVER_PCT = 100;

// How long a deleted transaction stays hidden-but-recoverable before the
// DELETE actually hits the API. Clicking "Undo" on the toast within this
// window cancels the pending delete entirely — nothing was ever sent.
const UNDO_WINDOW_MS = 5000;

function budgetTier(percentUsed) {
  if (percentUsed >= BUDGET_OVER_PCT) return "over";
  if (percentUsed >= BUDGET_WARN_PCT) return "warning";
  return "ok";
}

// Which sign-in screen makes sense as the default, given what's actually
// configured. Registered accounts are the primary way in now, so they
// take priority: any account existing means "login" is the useful
// default, regardless of whether a PIN also exists. Only a legacy
// install — a PIN already set up and no accounts yet — still defaults to
// the PIN pad. A totally fresh install (neither configured) leads with
// registration rather than PIN setup, though PIN setup is still one link
// away from there.
function computeDefaultView(pinConfigured, accountsExist) {
  if (accountsExist) return "login";
  if (pinConfigured) return "pin";
  return "register";
}

export default function App() {
  const { theme, toggleTheme } = useTheme();
  const auth = useAuth();
  // "pin" | "register" | "login" — which sign-in screen to show while
  // locked. Independent of auth.status; just controls which form renders.
  //
  // Default is picked by computeDefaultView below, re-derived whenever
  // the underlying facts change or the app goes from unlocked back to
  // locked (explicit lock button, or a session expiring mid-use) so a
  // stale register/login/pin view doesn't linger. Accounts are the
  // primary way in now, so registration/login take priority over the
  // PIN pad: a household with any registered account defaults to login;
  // a truly fresh install (nothing configured at all) defaults to
  // registration, not PIN setup; only a legacy install that already has
  // a PIN and no accounts yet still defaults to the PIN pad.
  const [authView, setAuthView] = useState("register");
  const wasUnlocked = useRef(false);
  useEffect(() => {
    const goingLocked = wasUnlocked.current && auth.status !== "unlocked";
    const justResolved = auth.status !== "checking" && auth.status !== "unlocked";
    if (goingLocked || justResolved) {
      setAuthView(computeDefaultView(auth.pinConfigured, auth.accountsExist));
    }
    wasUnlocked.current = auth.status === "unlocked";
  }, [auth.status, auth.pinConfigured, auth.accountsExist]);
  const { toasts, notify, dismiss, permission, requestPermission } =
    useNotifications();
  const [transactions, setTransactions] = useState([]);
  const [report, setReport] = useState(null);
  const [categories, setCategories] = useState([]);
  const [budgetStatus, setBudgetStatus] = useState([]);
  const [recurringRules, setRecurringRules] = useState([]);
  const [goals, setGoals] = useState([]);
  const [tab, setTab] = useState("ledger");
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(null);
  const [submitting, setSubmitting] = useState(false);
  // Transactions the user just deleted but that are still within the undo
  // window — hidden from the ledger immediately, but the actual DELETE call
  // hasn't fired yet (see handleDelete / pendingDeleteTimers below).
  const [pendingDeleteIds, setPendingDeleteIds] = useState(() => new Set());
  const pendingDeleteTimers = useRef(new Map());
  const [filters, setFilters] = useState({
    query: "",
    type: "all",
    category: "all",
    dateFrom: "",
    dateTo: "",
  });

  // Tracks the last notified budget tier per budget id, so a refresh that
  // still sits at 92% doesn't re-fire the same "approaching limit" alert
  // every time — only actual crossings (ok -> warning -> over, or back down)
  // notify. Recurring rules are tracked by their `next_due` so a generated
  // occurrence (next_due advancing) only announces itself once.
  const budgetTiersRef = useRef(new Map());
  const recurringDueRef = useRef(new Map());
  const hasSeenDataRef = useRef(false);

  // If the app closes/unmounts mid-undo-window, cancel any pending deletes
  // rather than leaving orphaned timers — the transaction just stays as-is
  // (same outcome as if the user had clicked Undo).
  useEffect(() => {
    const timers = pendingDeleteTimers.current;
    return () => {
      for (const timeoutId of timers.values()) clearTimeout(timeoutId);
      timers.clear();
    };
  }, []);

  const checkBudgetAlerts = useCallback(
    (nextBudgetStatus) => {
      for (const b of nextBudgetStatus) {
        const tier = budgetTier(b.percent_used);
        const prevTier = budgetTiersRef.current.get(b.id) || "ok";
        if (tier !== prevTier) {
          budgetTiersRef.current.set(b.id, tier);
          if (tier === "over") {
            notify(
              "error",
              `${b.category} budget exceeded`,
              `${formatCurrency(b.spent)} spent of a ${formatCurrency(b.monthly_limit)} limit this month.`,
              { tag: `budget-${b.id}` }
            );
          } else if (tier === "warning") {
            notify(
              "warning",
              `${b.category} budget at ${Math.round(b.percent_used)}%`,
              `${formatCurrency(b.spent)} of ${formatCurrency(b.monthly_limit)} used this month.`,
              { tag: `budget-${b.id}` }
            );
          }
        }
      }
      const liveIds = new Set(nextBudgetStatus.map((b) => b.id));
      for (const id of budgetTiersRef.current.keys()) {
        if (!liveIds.has(id)) budgetTiersRef.current.delete(id);
      }
    },
    [notify]
  );

  const checkRecurringFires = useCallback(
    (nextRecurring) => {
      for (const r of nextRecurring) {
        const prevDue = recurringDueRef.current.get(r.id);
        if (prevDue && prevDue !== r.next_due) {
          notify(
            "info",
            "Recurring transaction added",
            `${r.category} — ${formatCurrency(r.amount)} added to your ledger.`,
            { tag: `recurring-${r.id}` }
          );
        }
        recurringDueRef.current.set(r.id, r.next_due);
      }
      const liveIds = new Set(nextRecurring.map((r) => r.id));
      for (const id of recurringDueRef.current.keys()) {
        if (!liveIds.has(id)) recurringDueRef.current.delete(id);
      }
    },
    [notify]
  );

  // `silent` skips the loading spinner — used for the background poll so it
  // doesn't visually reset the screen every couple of minutes.
  const refresh = useCallback(
    async ({ silent = false } = {}) => {
      if (!silent) setLoadError(null);
      try {
        const [list, rep, cats, budgetStat, recurring, goalList] = await Promise.all([
          listTransactions(),
          getReport(),
          listCategories(),
          getBudgetStatus(),
          listRecurring(),
          listGoals(),
        ]);
        setTransactions(list || []);
        setReport(rep);
        setCategories(cats || []);
        setBudgetStatus(budgetStat || []);
        setRecurringRules(recurring || []);
        setGoals(goalList || []);

        if (hasSeenDataRef.current) {
          checkBudgetAlerts(budgetStat || []);
          checkRecurringFires(recurring || []);
        } else {
          // First load: seed the tracking maps without notifying, so
          // opening the app doesn't immediately blast alerts for
          // budgets that were already over limit yesterday.
          for (const b of budgetStat || [])
            budgetTiersRef.current.set(b.id, budgetTier(b.percent_used));
          for (const r of recurring || [])
            recurringDueRef.current.set(r.id, r.next_due);
          hasSeenDataRef.current = true;
        }
      } catch (err) {
        const message =
          err.message ||
          "Could not reach the API. Is the Go backend running?";
        if (!silent) setLoadError(message);
        else notify("error", "Couldn't refresh", message);
      } finally {
        if (!silent) setLoading(false);
      }
    },
    [checkBudgetAlerts, checkRecurringFires, notify]
  );

  useEffect(() => {
    if (auth.status === "unlocked") {
      refresh();
    }
  }, [auth.status, refresh]);

  // A 403 means a guest session tried something guest-blocked. Most of
  // these are already prevented by hiding the relevant buttons for guests
  // (see the isGuest prop threaded through below), but this is the
  // backstop for anything not yet gated client-side, and for guests who
  // reach a write endpoint some other way (e.g. a stale UI state).
  useEffect(() => {
    setForbiddenHandler((message) => {
      notify("warning", "Not available in view-only mode", message);
    });
  }, [notify]);

  // Recurring rules generate server-side (on an hourly ticker, even with no
  // tab open) and budget usage shifts as the month goes on — poll quietly
  // every few minutes so alerts about either can surface without requiring
  // the user to touch anything.
  useEffect(() => {
    if (auth.status !== "unlocked") return;
    const interval = setInterval(() => refresh({ silent: true }), 3 * 60 * 1000);
    return () => clearInterval(interval);
  }, [auth.status, refresh]);

  const monthlyTrend = useMemo(() => {
    if (!report?.by_month?.length) return [];
    let running = 0;
    return report.by_month.map((m) => {
      running += m.total_income - m.total_expense;
      return { month: m.month, net: running };
    });
  }, [report]);

  const filteredTransactions = useMemo(() => {
    const query = filters.query.trim().toLowerCase();
    return transactions.filter((t) => {
      if (pendingDeleteIds.has(t.id)) return false;
      if (filters.type !== "all" && t.type !== filters.type) return false;
      if (filters.category !== "all" && t.category !== filters.category) return false;
      if (filters.dateFrom && t.date < filters.dateFrom) return false;
      if (filters.dateTo && t.date > filters.dateTo) return false;
      if (query) {
        const haystack = `${t.description} ${t.category}`.toLowerCase();
        if (!haystack.includes(query)) return false;
      }
      return true;
    });
  }, [transactions, filters, pendingDeleteIds]);

  const visibleTotalCount = useMemo(
    () => transactions.filter((t) => !pendingDeleteIds.has(t.id)).length,
    [transactions, pendingDeleteIds]
  );

  const isFiltering =
    filters.query.trim() !== "" ||
    filters.type !== "all" ||
    filters.category !== "all" ||
    filters.dateFrom !== "" ||
    filters.dateTo !== "";

  // Wraps a mutation with success/error toasts. Errors are re-thrown after
  // notifying so the calling form's own try/catch (inline validation
  // messages, `finally` cleanup, etc.) still runs exactly as before — the
  // toast is additive, not a replacement for that local handling.
  async function withFeedback(action, { successTitle, successMessage, errorTitle } = {}) {
    try {
      const result = await action();
      if (successTitle) notify("success", successTitle, successMessage);
      return result;
    } catch (err) {
      notify("error", errorTitle, err.message || "Something went wrong.");
      throw err;
    }
  }

  async function handleCreate(entry) {
    setSubmitting(true);
    try {
      await withFeedback(
        async () => {
          await createTransaction(entry);
          await refresh();
        },
        {
          successTitle: "Transaction added",
          successMessage: `${entry.category} — ${formatCurrency(entry.amount)}`,
          errorTitle: "Couldn't add transaction",
        }
      );
    } finally {
      setSubmitting(false);
    }
  }

  async function handleUpdate(id, updated) {
    await withFeedback(
      async () => {
        await updateTransaction(id, updated);
        await refresh();
      },
      { successTitle: "Transaction updated", errorTitle: "Couldn't update transaction" }
    );
  }

  // Quick inline edit — only sends the field that changed instead of the
  // full transaction object.
  async function handlePatchAmount(id, amount) {
    await withFeedback(
      async () => {
        await patchTransaction(id, { amount });
        await refresh();
      },
      { errorTitle: "Couldn't update amount" }
    );
  }

  // Deleting a transaction doesn't call the API right away — it's hidden
  // from the ledger immediately (via pendingDeleteIds) and the real DELETE
  // is deferred by UNDO_WINDOW_MS, giving the toast's "Undo" button
  // something to cancel. Dismissing the toast itself (the X) does NOT
  // cancel the pending delete, only clicking Undo does.
  //
  // Known limitation: this undo state lives only in memory, so closing the
  // tab or reloading mid-window loses the pending timer — the transaction
  // simply never gets deleted and reappears on the next load, same as if
  // Undo had been clicked.
  function handleDelete(id) {
    setPendingDeleteIds((prev) => new Set(prev).add(id));

    const timeoutId = setTimeout(async () => {
      pendingDeleteTimers.current.delete(id);
      try {
        await deleteTransaction(id);
        await refresh();
      } catch (err) {
        notify("error", "Couldn't delete transaction", err.message || "Something went wrong.");
      } finally {
        setPendingDeleteIds((prev) => {
          const next = new Set(prev);
          next.delete(id);
          return next;
        });
      }
    }, UNDO_WINDOW_MS);

    pendingDeleteTimers.current.set(id, timeoutId);

    notify("info", "Transaction deleted", null, {
      durationMs: UNDO_WINDOW_MS,
      action: {
        label: "Undo",
        onClick: () => {
          const pending = pendingDeleteTimers.current.get(id);
          if (pending) {
            clearTimeout(pending);
            pendingDeleteTimers.current.delete(id);
          }
          setPendingDeleteIds((prev) => {
            const next = new Set(prev);
            next.delete(id);
            return next;
          });
        },
      },
    });
  }

  async function handleCreateCategory(category) {
    await withFeedback(
      async () => {
        await createCategory(category);
        await refresh();
      },
      { successTitle: "Category added", successMessage: category.name, errorTitle: "Couldn't add category" }
    );
  }

  async function handleDeleteCategory(id) {
    await withFeedback(
      async () => {
        await deleteCategory(id);
        await refresh();
      },
      { successTitle: "Category removed", errorTitle: "Couldn't remove category" }
    );
  }

  async function handleCreateBudget(budget) {
    await withFeedback(
      async () => {
        await createBudget(budget);
        await refresh();
      },
      { successTitle: "Budget set", successMessage: budget.category, errorTitle: "Couldn't set budget" }
    );
  }

  async function handleUpdateBudgetLimit(id, budget) {
    await withFeedback(
      async () => {
        await updateBudget(id, budget);
        await refresh();
      },
      { successTitle: "Budget limit updated", successMessage: budget.category, errorTitle: "Couldn't update budget limit" }
    );
  }

  async function handleDeleteBudget(id) {
    await withFeedback(
      async () => {
        await deleteBudget(id);
        await refresh();
      },
      { successTitle: "Budget removed", errorTitle: "Couldn't remove budget" }
    );
  }

  async function handleCreateRecurring(rule) {
    await withFeedback(
      async () => {
        await createRecurring(rule);
        await refresh();
      },
      { successTitle: "Recurring rule added", successMessage: rule.category, errorTitle: "Couldn't add recurring rule" }
    );
  }

  async function handleDeleteRecurring(id) {
    await withFeedback(
      async () => {
        await deleteRecurring(id);
        await refresh();
      },
      { successTitle: "Recurring rule removed", errorTitle: "Couldn't remove recurring rule" }
    );
  }

  async function handleCreateGoal(goal) {
    await withFeedback(
      async () => {
        await createGoal(goal);
        await refresh();
      },
      { successTitle: "Goal added", successMessage: goal.name, errorTitle: "Couldn't add goal" }
    );
  }

  async function handleDeleteGoal(id) {
    await withFeedback(
      async () => {
        await deleteGoal(id);
        await refresh();
      },
      { successTitle: "Goal removed", errorTitle: "Couldn't remove goal" }
    );
  }

  async function handleContributeGoal(id, amount) {
    await withFeedback(
      async () => {
        await contributeGoal(id, amount);
        await refresh();
      },
      {
        successTitle: amount > 0 ? "Contribution added" : "Goal adjusted",
        successMessage: formatCurrency(Math.abs(amount)),
        errorTitle: "Couldn't update goal",
      }
    );
  }

  // Not wrapped in withFeedback since the success message depends on the
  // imported/skipped counts rather than a fixed string — everything else
  // follows the same notify-then-rethrow shape so Manage's own inline
  // error state (importError) still works exactly like the other forms.
  async function handleImportFile(file) {
    try {
      const result = await importTransactionsFile(file);
      await refresh();
      if (result.imported > 0) {
        notify(
          "success",
          `Imported ${result.imported} transaction${result.imported === 1 ? "" : "s"}`,
          result.skipped > 0 ? `${result.skipped} row(s) skipped — see below.` : undefined
        );
      }
      if (result.errors?.length > 0) {
        notify(
          "warning",
          `${result.skipped} row${result.skipped === 1 ? "" : "s"} could not be imported`,
          result.errors.slice(0, 3).join(" · "),
          { stayOpen: true }
        );
      }
      // Set for formats parsed heuristically rather than exactly (PDF) —
      // worth a nudge to double-check even when nothing was "skipped".
      if (result.note) {
        notify("info", "Worth a double check", result.note, { stayOpen: true });
      }
      return result;
    } catch (err) {
      notify("error", "Import failed", err.message || "Something went wrong.");
      throw err;
    }
  }

  // Draft rows come from client-side OCR (ScanImportModal), already
  // reviewed/edited by the user, so this just persists them one at a time
  // through the normal create endpoint — no dedicated backend route needed.
  async function handleScanImportConfirm(rows) {
    let saved = 0;
    const failures = [];

    for (const row of rows) {
      const amount = parseFloat(row.amount);
      if (!amount || amount <= 0 || !row.category) {
        failures.push(row.description || "Untitled row");
        continue;
      }
      try {
        await createTransaction({
          type: row.type,
          amount,
          category: row.category,
          description: row.description.trim(),
          date: row.date,
        });
        saved++;
      } catch {
        failures.push(row.description || "Untitled row");
      }
    }

    await refresh();

    if (saved > 0) {
      notify(
        "success",
        `Added ${saved} transaction${saved === 1 ? "" : "s"} from screenshot`,
        failures.length > 0 ? `${failures.length} row(s) couldn't be saved.` : undefined
      );
    }
    if (failures.length > 0) {
      notify(
        "warning",
        `${failures.length} row${failures.length === 1 ? "" : "s"} not saved`,
        failures.slice(0, 3).join(" · "),
        { stayOpen: true }
      );
    }
    if (saved === 0) {
      throw new Error("None of those rows could be saved — check amount and category.");
    }
  }

  const tabs = [
    { id: "ledger", label: "Ledger", icon: BookOpen },
    { id: "reports", label: "Reports", icon: PieChart },
    { id: "manage", label: "Manage", icon: Settings2 },
  ];

  if (auth.status === "checking") {
    return (
      <div className="app-shell app-shell--centered">
        <p className="loading-text">
          <Loader2 size={15} className="spin" /> Checking…
        </p>
      </div>
    );
  }

  if (auth.status === "setup" || auth.status === "locked") {
    return (
      <div className="app-shell app-shell--centered">
        {authView === "register" && (
          <RegisterForm
            onSubmit={auth.handleRegister}
            onBack={() => setAuthView(auth.pinConfigured ? "pin" : "login")}
            error={auth.error}
            loading={auth.submitting}
          />
        )}
        {authView === "login" && (
          <div className="auth-screen">
            <LoginForm
              onSubmit={auth.handleAccountLogin}
              error={auth.error}
              loading={auth.submitting}
            />
            <button
              type="button"
              className="lock-screen__link"
              onClick={() => setAuthView("register")}
              disabled={auth.submitting}
            >
              Create an account instead
            </button>
            {auth.pinConfigured && (
              <button
                type="button"
                className="lock-screen__link"
                onClick={() => setAuthView("pin")}
                disabled={auth.submitting}
              >
                Use PIN instead
              </button>
            )}
          </div>
        )}
        {authView === "pin" && (
          <div className="auth-screen">
            <LockScreen
              mode={auth.status === "setup" ? "setup" : "login"}
              onSubmit={auth.status === "setup" ? auth.handleSetup : auth.handleLogin}
              error={auth.error}
              loading={auth.submitting}
            />
            {auth.accountsExist && (
              <button
                type="button"
                className="lock-screen__link"
                onClick={() => setAuthView("login")}
                disabled={auth.submitting}
              >
                Log in to your account
              </button>
            )}
            <button
              type="button"
              className="lock-screen__link"
              onClick={() => setAuthView("register")}
              disabled={auth.submitting}
            >
              Create an account instead
            </button>
          </div>
        )}
      </div>
    );
  }

  return (
    <div className="app-shell">
      <ToastStack toasts={toasts} onDismiss={dismiss} />

      <BalanceHeader
        totalIncome={report?.total_income ?? 0}
        totalExpense={report?.total_expense ?? 0}
        net={report?.net ?? 0}
        monthlyTrend={monthlyTrend}
        theme={theme}
        onToggleTheme={toggleTheme}
        onLock={auth.lock}
        notificationPermission={permission}
        onRequestNotifications={requestPermission}
        isGuest={auth.isGuest}
      />

      {loadError && (
        <motion.div
          className="banner banner--error"
          initial={{ opacity: 0, y: -6 }}
          animate={{ opacity: 1, y: 0 }}
        >
          {loadError}
        </motion.div>
      )}

      {!auth.isGuest && (
        <TransactionForm
          categories={categories}
          onSubmit={handleCreate}
          submitting={submitting}
        />
      )}

      <nav className="tabs">
        {tabs.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            className={`tabs__btn ${tab === id ? "is-active" : ""}`}
            onClick={() => setTab(id)}
          >
            <Icon size={15} strokeWidth={2} />
            {label}
            {tab === id && (
              <motion.span
                className="tabs__indicator"
                layoutId="tabs-indicator"
                transition={{ type: "spring", stiffness: 500, damping: 35 }}
              />
            )}
          </button>
        ))}
      </nav>

      <main className="app-main">
        {loading ? (
          <p className="loading-text">
            <Loader2 size={15} className="spin" /> Loading your ledger…
          </p>
        ) : (
          <AnimatePresence mode="wait">
            {tab === "ledger" && (
              <motion.div
                key="ledger"
                initial={{ opacity: 0, x: -12 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: 12 }}
                transition={{ duration: 0.2, ease: "easeOut" }}
              >
                <LedgerFilters
                  categories={categories}
                  filters={filters}
                  onChange={setFilters}
                />
                <TransactionList
                  transactions={filteredTransactions}
                  categories={categories}
                  onDelete={handleDelete}
                  onUpdate={handleUpdate}
                  onPatchAmount={handlePatchAmount}
                  deletingId={null}
                  isFiltering={isFiltering}
                  totalCount={visibleTotalCount}
                  readOnly={auth.isGuest}
                />
              </motion.div>
            )}
            {tab === "reports" && (
              <motion.div
                key="reports"
                initial={{ opacity: 0, x: 12 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -12 }}
                transition={{ duration: 0.2, ease: "easeOut" }}
              >
                <Reports transactions={transactions} budgetStatus={budgetStatus} />
              </motion.div>
            )}
            {tab === "manage" && (
              <motion.div
                key="manage"
                initial={{ opacity: 0, x: 12 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -12 }}
                transition={{ duration: 0.2, ease: "easeOut" }}
              >
                <Manage
                  categories={categories}
                  budgetStatus={budgetStatus}
                  recurringRules={recurringRules}
                  goals={goals}
                  onCreateCategory={handleCreateCategory}
                  onDeleteCategory={handleDeleteCategory}
                  onCreateBudget={handleCreateBudget}
                  onUpdateBudget={handleUpdateBudgetLimit}
                  onDeleteBudget={handleDeleteBudget}
                  onCreateRecurring={handleCreateRecurring}
                  onDeleteRecurring={handleDeleteRecurring}
                  onCreateGoal={handleCreateGoal}
                  onDeleteGoal={handleDeleteGoal}
                  onContributeGoal={handleContributeGoal}
                  onImportFile={handleImportFile}
                  onScanImportConfirm={handleScanImportConfirm}
                  readOnly={auth.isGuest}
                />
              </motion.div>
            )}
          </AnimatePresence>
        )}
      </main>
    </div>
  );
}
