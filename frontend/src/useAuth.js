import { useCallback, useEffect, useState } from "react";
import {
  getAuthStatus,
  setupPin as apiSetupPin,
  loginPin as apiLoginPin,
  registerAccount as apiRegisterAccount,
  accountLogin as apiAccountLogin,
  logout as apiLogout,
  checkSession,
  getToken,
  setToken,
  clearToken,
  getRole,
  setRole as storeRole,
  clearRole,
  setUnauthorizedHandler,
} from "./api";

// How often an unlocked session pings the lightweight session-check
// endpoint. This is what makes a revoked guest session (owner turns off
// guest access) bounce that guest's tab back to the PIN screen within a
// few seconds, rather than only on their next real action. Cheap enough
// on the backend (see HandleAuthSession) to run this often.
const SESSION_POLL_MS = 4000;

export function useAuth() {
  // "checking" | "setup" | "locked" | "unlocked"
  const [status, setStatus] = useState("checking");
  const [role, setRole] = useState(null); // "owner" | "guest" | "user" | null
  const [error, setError] = useState(null);
  const [submitting, setSubmitting] = useState(false);
  // Whether the legacy owner PIN has ever been configured. Lets the
  // frontend default to the account login screen instead of a PIN pad
  // that can never succeed, for a household that only ever registered
  // accounts and never touched the PIN.
  // Whether the legacy owner PIN has ever been configured, and whether
  // any account has ever been registered. Together these decide which
  // screen makes sense as the *default* on the locked/setup screen (see
  // App.jsx) — a household with accounts should land on login, not a PIN
  // pad; a truly fresh install should lead with registration now that
  // accounts are the primary way in, not PIN setup.
  const [pinConfigured, setPinConfigured] = useState(true);
  const [accountsExist, setAccountsExist] = useState(true);

  const checkStatus = useCallback(async () => {
    try {
      const { pin_set, accounts_exist } = await getAuthStatus();
      setPinConfigured(pin_set);
      setAccountsExist(accounts_exist);
      if (!pin_set && !accounts_exist) {
        // Truly nothing configured yet — this is a fresh install, so
        // guide toward first-time setup. RegisterForm is still reachable
        // from this screen via a link, it's just not the default.
        setStatus("setup");
        return;
      }
      // Either a PIN or at least one registered account already exists —
      // don't force PIN setup on people who only ever registered an
      // account and never touched the PIN. Show the normal locked screen
      // (PIN pad, with links to log in or register) instead.
      // A PIN is configured — if we already hold a token, assume it's good
      // and let the first real API call prove otherwise (via the 401
      // handler below). Avoids an extra round trip just to validate it.
      if (getToken()) {
        // A token saved before guest PINs existed has no stored role —
        // every token issued back then was an owner token, so default to
        // that rather than leaving role blank.
        setRole(getRole() || "owner");
        setStatus("unlocked");
      } else {
        setStatus("locked");
      }
    } catch {
      // Backend unreachable — nothing productive to show but a lock
      // screen; the rest of the app will surface the connection error once
      // it tries to load data.
      if (getToken()) {
        setRole(getRole() || "owner");
        setStatus("unlocked");
      } else {
        setStatus("locked");
      }
    }
  }, []);

  useEffect(() => {
    setUnauthorizedHandler(() => {
      clearToken();
      clearRole();
      setRole(null);
      setStatus("locked");
    });
    checkStatus();
  }, [checkStatus]);

  // Poll the session-check endpoint while unlocked. A 401 here runs
  // through apiFetch -> handleResponse -> the unauthorizedHandler
  // registered above, same path as any other rejected request, so this
  // effect itself doesn't need to do anything on failure — just keep
  // asking. Network hiccups are ignored; they're not evidence the session
  // is actually gone, just that this one check didn't complete.
  useEffect(() => {
    if (status !== "unlocked") return undefined;
    const interval = setInterval(() => {
      checkSession().catch(() => {});
    }, SESSION_POLL_MS);
    return () => clearInterval(interval);
  }, [status]);

  // Returns true/false rather than relying on callers to watch the `error`
  // string for changes — two consecutive failures produce the identical
  // message ("incorrect PIN"), and React skips re-rendering (and thus any
  // effect keyed on it) when a state update doesn't actually change the
  // value. A plain boolean result doesn't have that problem.
  async function handleSetup(pin) {
    setSubmitting(true);
    setError(null);
    try {
      const { token, role: grantedRole } = await apiSetupPin(pin);
      setToken(token);
      storeRole(grantedRole);
      setRole(grantedRole);
      setStatus("unlocked");
      return true;
    } catch (err) {
      setError(err.message || "Could not set up PIN.");
      return false;
    } finally {
      setSubmitting(false);
    }
  }

  async function handleLogin(pin) {
    setSubmitting(true);
    setError(null);
    try {
      const { token, role: grantedRole } = await apiLoginPin(pin);
      setToken(token);
      storeRole(grantedRole);
      setRole(grantedRole);
      setStatus("unlocked");
      return true;
    } catch (err) {
      setError(err.message || "Incorrect PIN.");
      return false;
    } finally {
      setSubmitting(false);
    }
  }

  // Same shape as handleSetup/handleLogin (returns a boolean, sets
  // `error` on failure) so RegisterForm can reuse the same submit
  // pattern as LockScreen.
  async function handleRegister(email, password) {
    setSubmitting(true);
    setError(null);
    try {
      const { token, role: grantedRole } = await apiRegisterAccount(email, password);
      setToken(token);
      storeRole(grantedRole);
      setRole(grantedRole);
      setStatus("unlocked");
      return true;
    } catch (err) {
      setError(err.message || "Could not create account.");
      return false;
    } finally {
      setSubmitting(false);
    }
  }

  // Counterpart to handleRegister for a returning registered account.
  async function handleAccountLogin(email, password) {
    setSubmitting(true);
    setError(null);
    try {
      const { token, role: grantedRole } = await apiAccountLogin(email, password);
      setToken(token);
      storeRole(grantedRole);
      setRole(grantedRole);
      setStatus("unlocked");
      return true;
    } catch (err) {
      setError(err.message || "Incorrect email or password.");
      return false;
    } finally {
      setSubmitting(false);
    }
  }

  async function lock() {
    try {
      await apiLogout();
    } catch {
      // Even if the network call fails, still lock locally.
    }
    clearToken();
    clearRole();
    setRole(null);
    setStatus("locked");
  }

  return {
    status,
    role,
    isGuest: role === "guest",
    pinConfigured,
    accountsExist,
    error,
    submitting,
    handleSetup,
    handleLogin,
    handleRegister,
    handleAccountLogin,
    lock,
  };
}
