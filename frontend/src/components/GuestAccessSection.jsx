import { useEffect, useState } from "react";
import { motion } from "framer-motion";
import { Users, Check } from "lucide-react";
import { getAuthStatus, setGuestPin, removeGuestPin } from "../api";

// Owner-only settings for guest access — set up or rotate the guest PIN,
// or turn guest access off entirely. Mirrors ChangePinSection's shape.
// Rendered only for the owner role (Manage.jsx hides this for guests, and
// the backend would reject a guest's attempt to reach it anyway).
export default function GuestAccessSection() {
  const [guestPinSet, setGuestPinSet] = useState(null); // null while loading
  const [current, setCurrent] = useState("");
  const [guestPin, setGuestPinDraft] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(false);
  const [saving, setSaving] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [removeConfirmOpen, setRemoveConfirmOpen] = useState(false);
  const [removePin, setRemovePin] = useState("");
  const [removeError, setRemoveError] = useState(null);

  useEffect(() => {
    let cancelled = false;
    getAuthStatus()
      .then((s) => {
        if (!cancelled) setGuestPinSet(!!s.guest_pin_set);
      })
      .catch(() => {
        if (!cancelled) setGuestPinSet(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function handleSubmit(e) {
    e.preventDefault();
    setError(null);
    setSuccess(false);

    if (!/^\d{4,8}$/.test(guestPin)) {
      setError("Guest PIN must be 4-8 digits.");
      return;
    }
    if (guestPin !== confirm) {
      setError("Guest PIN and confirmation don't match.");
      return;
    }

    setSaving(true);
    try {
      await setGuestPin(current, guestPin);
      setCurrent("");
      setGuestPinDraft("");
      setConfirm("");
      setGuestPinSet(true);
      setSuccess(true);
      setTimeout(() => setSuccess(false), 2000);
    } catch (err) {
      setError(err.message || "Could not set guest PIN.");
    } finally {
      setSaving(false);
    }
  }

  async function handleRemove(e) {
    e.preventDefault();
    setRemoveError(null);
    setRemoving(true);
    try {
      await removeGuestPin(removePin);
      setGuestPinSet(false);
      setRemovePin("");
      setRemoveConfirmOpen(false);
    } catch (err) {
      setRemoveError(err.message || "Could not remove guest access.");
    } finally {
      setRemoving(false);
    }
  }

  return (
    <div className="manage-section">
      <h3 className="manage-section__title">
        <Users size={15} strokeWidth={2} style={{ marginRight: 6, verticalAlign: -2 }} />
        Guest access
      </h3>
      <p className="manage-section__subtitle">
        A second PIN for people you share a tunnel link with. Guests can
        view the ledger, reports, budgets, and goals, but can't add, edit,
        or delete anything, and can't import or export data.
      </p>

      <form className="manage-form" onSubmit={handleSubmit} style={{ borderTop: "none", paddingTop: 0 }}>
        <input
          type="password"
          inputMode="numeric"
          placeholder="Your (owner) PIN"
          value={current}
          onChange={(e) => setCurrent(e.target.value.replace(/\D/g, ""))}
          className="manage-form__input"
          maxLength={8}
        />
        <div className="manage-form__row">
          <input
            type="password"
            inputMode="numeric"
            placeholder="Guest PIN"
            value={guestPin}
            onChange={(e) => setGuestPinDraft(e.target.value.replace(/\D/g, ""))}
            maxLength={8}
          />
          <input
            type="password"
            inputMode="numeric"
            placeholder="Confirm guest PIN"
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
          disabled={saving}
        >
          {success ? (
            <>
              <Check size={15} strokeWidth={2.5} /> Guest PIN set
            </>
          ) : saving ? (
            "Saving…"
          ) : guestPinSet ? (
            "Change guest PIN"
          ) : (
            "Turn on guest access"
          )}
        </motion.button>
      </form>

      {guestPinSet && (
        <div style={{ marginTop: 12 }}>
          {!removeConfirmOpen ? (
            <button
              type="button"
              className="manage-form__submit"
              onClick={() => setRemoveConfirmOpen(true)}
            >
              Turn off guest access
            </button>
          ) : (
            <form className="manage-form" onSubmit={handleRemove} style={{ borderTop: "none", paddingTop: 0 }}>
              <p className="manage-group__empty">
                This immediately signs out anyone currently using the guest
                PIN. Confirm with your owner PIN.
              </p>
              <input
                type="password"
                inputMode="numeric"
                placeholder="Your (owner) PIN"
                value={removePin}
                onChange={(e) => setRemovePin(e.target.value.replace(/\D/g, ""))}
                className="manage-form__input"
                maxLength={8}
              />
              {removeError && <p className="manage-form__error">{removeError}</p>}
              <div className="manage-form__row">
                <motion.button
                  type="submit"
                  whileTap={{ scale: 0.97 }}
                  className="manage-form__submit"
                  disabled={removing}
                >
                  {removing ? "Removing…" : "Confirm removal"}
                </motion.button>
                <button
                  type="button"
                  className="manage-form__submit"
                  onClick={() => {
                    setRemoveConfirmOpen(false);
                    setRemovePin("");
                    setRemoveError(null);
                  }}
                >
                  Cancel
                </button>
              </div>
            </form>
          )}
        </div>
      )}
    </div>
  );
}
