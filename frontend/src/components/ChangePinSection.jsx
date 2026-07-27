import { useEffect, useState } from "react";
import { motion } from "framer-motion";
import { Lock, Check } from "lucide-react";
import { changePin, enablePin, getAuthStatus } from "../api";

// Two different forms live here depending on whether a household PIN
// exists yet:
//   - No PIN yet (common for a household that only ever registers
//     accounts — see HandleAuthEnablePIN on the backend): a "Set up PIN"
//     form with just new + confirm, no current-PIN field, since there's
//     nothing to verify against yet.
//   - PIN already exists: the original "Change PIN" form, requiring the
//     current PIN.
// pinSet starts null (unknown) rather than defaulting to true/false, so
// the form doesn't flash the wrong one before the status check resolves.
export default function ChangePinSection() {
  const [pinSet, setPinSet] = useState(null);
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let cancelled = false;
    getAuthStatus()
      .then(({ pin_set }) => {
        if (!cancelled) setPinSet(pin_set);
      })
      .catch(() => {
        // Unknown — default to the safer "change PIN" form, which at
        // least fails informatively rather than silently no-opping.
        if (!cancelled) setPinSet(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function handleSubmit(e) {
    e.preventDefault();
    setError(null);
    setSuccess(false);

    if (!/^\d{4,8}$/.test(next)) {
      setError("New PIN must be 4-8 digits.");
      return;
    }
    if (next !== confirm) {
      setError("New PIN and confirmation don't match.");
      return;
    }

    setSaving(true);
    try {
      if (pinSet) {
        await changePin(current, next);
      } else {
        await enablePin(next);
        setPinSet(true);
      }
      setCurrent("");
      setNext("");
      setConfirm("");
      setSuccess(true);
      setTimeout(() => setSuccess(false), 2000);
    } catch (err) {
      setError(err.message || (pinSet ? "Could not change PIN." : "Could not set up PIN."));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="manage-section">
      <h3 className="manage-section__title">
        <Lock size={15} strokeWidth={2} style={{ marginRight: 6, verticalAlign: -2 }} />
        {pinSet === false ? "Set up PIN" : "Change PIN"}
      </h3>
      <p className="manage-section__subtitle">
        {pinSet === false
          ? "Add a PIN as another way to unlock this app, alongside your account login."
          : "Update the PIN used to unlock this app."}
      </p>

      <form className="manage-form" onSubmit={handleSubmit} style={{ borderTop: "none", paddingTop: 0 }}>
        {pinSet !== false && (
          <input
            type="password"
            inputMode="numeric"
            placeholder="Current PIN"
            value={current}
            onChange={(e) => setCurrent(e.target.value.replace(/\D/g, ""))}
            className="manage-form__input"
            maxLength={8}
          />
        )}
        <div className="manage-form__row">
          <input
            type="password"
            inputMode="numeric"
            placeholder="New PIN"
            value={next}
            onChange={(e) => setNext(e.target.value.replace(/\D/g, ""))}
            maxLength={8}
          />
          <input
            type="password"
            inputMode="numeric"
            placeholder="Confirm new PIN"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value.replace(/\D/g, ""))}
            maxLength={8}
          />
        </div>

        {error && <p className="manage-form__error">{error}</p>}

        <motion.button
          type="submit"
          whileTap={{ scale: 0.97 }}
          className={`manage-form__submit ${success ? "is-success" : ""}`}
          disabled={saving || pinSet === null}
        >
          {success ? (
            <>
              <Check size={15} strokeWidth={2.5} /> PIN {pinSet === false ? "set up" : "updated"}
            </>
          ) : saving ? (
            pinSet === false ? "Setting up…" : "Updating…"
          ) : pinSet === false ? (
            "Set up PIN"
          ) : (
            "Update PIN"
          )}
        </motion.button>
      </form>
    </div>
  );
}
