// Falls back to the relative path when VITE_API_URL isn't set. That relative
// path still works for:
// - `npm run dev` (Vite proxies /api to the Go backend, see vite.config.js)
// - the built app served directly by the Go backend on :8080
// - a tunnel (cloudflared/ngrok) pointed at that single Go port, since
//   there's only ever one origin involved, no separate port to reach.
const BASE_URL = `/api`;
const TOKEN_KEY = "ledger-auth-token";
const ROLE_KEY = "ledger-auth-role";

let unauthorizedHandler = null;
let forbiddenHandler = null;

// App.jsx registers a callback here so that any request rejected with 401
// (session expired, PIN changed elsewhere, etc.) drops the user back to the
// lock screen instead of silently failing.
export function setUnauthorizedHandler(fn) {
  unauthorizedHandler = fn;
}

// A 403 means the session is perfectly valid but belongs to a guest trying
// something guests can't do — distinct from 401 so the frontend can show a
// "you don't have permission" message instead of bouncing to the lock
// screen as if the session had expired.
export function setForbiddenHandler(fn) {
  forbiddenHandler = fn;
}

export function getToken() {
  return window.localStorage.getItem(TOKEN_KEY);
}

export function setToken(token) {
  window.localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  window.localStorage.removeItem(TOKEN_KEY);
}

export function getRole() {
  return window.localStorage.getItem(ROLE_KEY);
}

export function setRole(role) {
  window.localStorage.setItem(ROLE_KEY, role);
}

export function clearRole() {
  window.localStorage.removeItem(ROLE_KEY);
}

async function handleResponse(res) {
  if (res.status === 401 && unauthorizedHandler) {
    unauthorizedHandler();
  }
  if (res.status === 204) return null;
  const data = await res.json().catch(() => null);
  if (!res.ok) {
    const message = data?.error || `Request failed with status ${res.status}`;
    if (res.status === 403 && forbiddenHandler) {
      forbiddenHandler(message);
    }
    throw new Error(message);
  }
  return data;
}

// Central fetch wrapper: attaches the session token (if any) to every
// request, so individual API functions below don't need to repeat that.
async function apiFetch(path, options = {}) {
  const token = getToken();
  const headers = { ...(options.headers || {}) };
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }
  const res = await fetch(`${BASE_URL}${path}`, { ...options, headers });
  return handleResponse(res);
}

function apiPost(path, body) {
  return apiFetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

function apiPut(path, body) {
  return apiFetch(path, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

function apiDelete(path) {
  return apiFetch(path, { method: "DELETE" });
}

// ---- Auth ----
// Auth endpoints deliberately don't go through apiFetch's auto-attached
// token — status/setup/login happen before a token exists, and the backend
// treats them as public regardless.

export async function getAuthStatus() {
  const res = await fetch(`${BASE_URL}/auth/status`);
  return handleResponse(res);
}

export async function setupPin(pin) {
  const res = await fetch(`${BASE_URL}/auth/setup`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ pin }),
  });
  return handleResponse(res);
}

export async function loginPin(pin) {
  const res = await fetch(`${BASE_URL}/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ pin }),
  });
  return handleResponse(res);
}

// Creates a real account (email + password) and, on success, returns a
// live session token — same "registration doubles as login" pattern as
// setupPin above. Distinct from the owner/guest PIN system; see
// HANDOVER.md for how the two currently coexist.
export async function registerAccount(email, password) {
  const res = await fetch(`${BASE_URL}/auth/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  return handleResponse(res);
}

// Signs an existing registered account back in. Distinct from loginPin
// above, which is the numeric owner/guest PIN and unrelated to accounts.
export async function accountLogin(email, password) {
  const res = await fetch(`${BASE_URL}/auth/account-login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  return handleResponse(res);
}

export async function logout() {
  return apiPost("/auth/logout", {});
}

// Deliberately tiny authenticated call — no data of its own, just proves
// the session is still valid. Polled frequently (see useAuth) so a
// revoked session (most importantly: the owner turning off guest access)
// bounces the affected tab back to the PIN screen within a few seconds,
// instead of waiting for whatever the next real data request happens to
// be. Goes through apiFetch (not a bare fetch) specifically so a 401 here
// runs through the same unauthorizedHandler as everything else.
export async function checkSession() {
  return apiFetch("/auth/session");
}

export async function changePin(currentPin, newPin) {
  return apiPost("/auth/change-pin", { current_pin: currentPin, new_pin: newPin });
}

// Sets up the household PIN for the first time from inside the already-
// unlocked app — for someone who registered an account and never went
// through the pre-login PIN setup screen. Distinct from setupPin (used on
// the pre-login screen): this one is authenticated and doesn't touch the
// caller's own session token.
export async function enablePin(pin) {
  return apiPost("/auth/enable-pin", { pin });
}

// Owner-only: create/rotate or remove guest access. Both require the
// current owner PIN — see HandleAuthGuestPIN on the backend for why (a
// live owner session isn't proof of knowing the PIN on its own).
export async function setGuestPin(currentPin, guestPin) {
  return apiPost("/auth/guest-pin", { current_pin: currentPin, guest_pin: guestPin });
}

export async function removeGuestPin(currentPin) {
  return apiFetch("/auth/guest-pin", {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ current_pin: currentPin }),
  });
}

// ---- Transactions ----

export function listTransactions() {
  return apiFetch("/transactions");
}

export function createTransaction(transaction) {
  return apiPost("/transactions", transaction);
}

export function updateTransaction(id, transaction) {
  return apiPut(`/transactions/${id}`, transaction);
}

// Partial update — only send the fields that changed. Prefer this over
// updateTransaction when you don't already have the full record in hand
// (e.g. a quick inline edit of just the amount or category).
export function patchTransaction(id, changes) {
  return apiFetch(`/transactions/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(changes),
  });
}

export function deleteTransaction(id) {
  return apiDelete(`/transactions/${id}`);
}

export function getReport() {
  return apiFetch("/reports");
}

// ---- Categories ----

export function listCategories() {
  return apiFetch("/categories");
}

export function createCategory(category) {
  return apiPost("/categories", category);
}

export function deleteCategory(id) {
  return apiDelete(`/categories/${id}`);
}

// ---- Budgets ----

export function listBudgets() {
  return apiFetch("/budgets");
}

export function getBudgetStatus() {
  return apiFetch("/budgets/status");
}

// Spend-vs-limit trend for each budget across the last N calendar months
// (defaults to 6 server-side). Each month is compared against whatever
// limit was actually in effect then, not just today's — see the `limit`
// field on each month entry.
export function getBudgetHistory(months = 6) {
  return apiFetch(`/budgets/history?months=${months}`);
}

export function createBudget(budget) {
  return apiPost("/budgets", budget);
}

export function updateBudget(id, budget) {
  return apiPut(`/budgets/${id}`, budget);
}

export function deleteBudget(id) {
  return apiDelete(`/budgets/${id}`);
}

// ---- Recurring rules ----

export function listRecurring() {
  return apiFetch("/recurring");
}

export function createRecurring(rule) {
  return apiPost("/recurring", rule);
}

export function deleteRecurring(id) {
  return apiDelete(`/recurring/${id}`);
}

// ---- Goals ----
// Separate from budgets: a budget caps spending, a goal tracks progress
// toward saving something. saved_amount only changes via contributeGoal,
// never derived from transactions.

export function listGoals() {
  return apiFetch("/goals");
}

export function createGoal(goal) {
  return apiPost("/goals", goal);
}

export function updateGoal(id, goal) {
  return apiPut(`/goals/${id}`, goal);
}

export function deleteGoal(id) {
  return apiDelete(`/goals/${id}`);
}

// amount can be negative to correct an over-contribution.
export function contributeGoal(id, amount) {
  return apiPost(`/goals/${id}/contribute`, { amount });
}

// ---- Import ----
// Uploads a CSV as multipart/form-data under the "file" field — the
// backend also accepts a raw CSV body, but FormData is the natural fit
// for a browser <input type="file">.

export async function importTransactionsCSV(file) {
  const token = getToken();
  const headers = token ? { Authorization: `Bearer ${token}` } : {};
  const formData = new FormData();
  formData.append("file", file);
  const res = await fetch(`${BASE_URL}/import/csv`, {
    method: "POST",
    headers,
    body: formData,
  });
  return handleResponse(res);
}

// Like importTransactionsCSV, but hits the combined endpoint that also
// accepts Excel (.xlsx/.xls) and PDF uploads — the backend picks a parser
// based on the file's extension, so this one function covers all three.
export async function importTransactionsFile(file) {
  const token = getToken();
  const headers = token ? { Authorization: `Bearer ${token}` } : {};
  const formData = new FormData();
  formData.append("file", file);
  const res = await fetch(`${BASE_URL}/import/file`, {
    method: "POST",
    headers,
    body: formData,
  });
  return handleResponse(res);
}

// ---- Export ----
// The export endpoint is behind auth once a PIN is set, and browsers don't
// attach custom headers to plain <a href> downloads — so this fetches the
// CSV with the session token attached, then triggers the download manually
// via a temporary object URL.

async function downloadExport(path, filename) {
  const token = getToken();
  const headers = token ? { Authorization: `Bearer ${token}` } : {};
  const res = await fetch(`${BASE_URL}${path}`, { headers });
  if (res.status === 401 && unauthorizedHandler) {
    unauthorizedHandler();
  }
  if (!res.ok) {
    throw new Error("Could not export transactions.");
  }
  const blob = await res.blob();
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  window.URL.revokeObjectURL(url);
}

export async function downloadExportCSV() {
  return downloadExport("/export/csv", "transactions.csv");
}

export async function downloadExportXLSX() {
  return downloadExport("/export/xlsx", "expense-tracker-export.xlsx");
}

export async function downloadExportHTML() {
  return downloadExport("/export/html", "expense-tracker-report.html");
}
