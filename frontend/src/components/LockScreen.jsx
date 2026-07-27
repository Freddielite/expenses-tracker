import { useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Lock, Delete, ShieldCheck } from "lucide-react";

const PIN_LENGTH = 4;
const ERROR_DISPLAY_MS = 700; // how long the red dots + message stay up before auto-clearing

export default function LockScreen({ mode, onSubmit, error, loading }) {
  // mode: "login" | "setup"
  const [stage, setStage] = useState("enter"); // "enter" | "confirm" (setup only)
  const [pin, setPin] = useState("");
  const [firstPin, setFirstPin] = useState("");
  const [mismatch, setMismatch] = useState(false);
  // Whether to actually show the parent's `error` prop right now. The prop
  // itself doesn't get cleared until the *next* submission resolves, so
  // without this, digits typed for a fresh attempt would still render with
  // the previous failure's red/error styling.
  const [showError, setShowError] = useState(false);
  const errorTimeoutRef = useRef(null);

  useEffect(() => {
    return () => {
      if (errorTimeoutRef.current) clearTimeout(errorTimeoutRef.current);
    };
  }, []);

  const isSetup = mode === "setup";
  const title = isSetup
    ? stage === "enter"
      ? "Set up a PIN"
      : "Confirm your PIN"
    : "Enter your PIN";
  const subtitle = isSetup
    ? stage === "enter"
      ? "Choose a 4-digit PIN to lock this app."
      : "Enter it once more to confirm."
    : "This ledger is locked.";

  function handleDigit(d) {
    // Typing dismisses a still-showing error immediately and starts a
    // fresh entry, in case the user acts before the auto-clear timer below
    // has fired.
    if (showError) {
      if (errorTimeoutRef.current) clearTimeout(errorTimeoutRef.current);
      setShowError(false);
      setPin(d);
      return;
    }
    if (pin.length >= PIN_LENGTH) return;
    const next = pin + d;
    setMismatch(false);
    setPin(next);

    if (next.length === PIN_LENGTH) {
      if (isSetup && stage === "enter") {
        setTimeout(() => {
          setFirstPin(next);
          setPin("");
          setStage("confirm");
        }, 150);
        return;
      }
      if (isSetup && stage === "confirm") {
        if (next !== firstPin) {
          setMismatch(true);
          setTimeout(() => {
            setPin("");
            setFirstPin("");
            setStage("enter");
          }, 500);
          return;
        }
        setTimeout(() => submit(next), 150);
        return;
      }
      // login
      setTimeout(() => submit(next), 150);
    }
  }

  // Waits for the actual result of the submit instead of relying on the
  // `error` prop changing — two consecutive wrong PINs produce the exact
  // same error message, and watching for a prop "change" that never
  // changes value would leave the pad stuck showing 4 filled dots.
  async function submit(candidatePin) {
    const ok = await onSubmit(candidatePin);
    if (!ok) {
      setShowError(true);
      // Flash the wrong PIN as red briefly, then clear itself automatically
      // — the user shouldn't have to do anything to get back to a fresh,
      // ready-to-type screen.
      errorTimeoutRef.current = setTimeout(() => {
        setPin("");
        setShowError(false);
        errorTimeoutRef.current = null;
      }, ERROR_DISPLAY_MS);
    }
  }

  function handleBackspace() {
    if (showError) return;
    setPin((p) => p.slice(0, -1));
  }

  return (
    <div className="lock-screen">
      <div className="lock-screen__icon">
        {isSetup ? (
          <ShieldCheck size={26} strokeWidth={1.5} />
        ) : (
          <Lock size={26} strokeWidth={1.5} />
        )}
      </div>

      <AnimatePresence mode="wait">
        <motion.div
          key={`${stage}-title`}
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -6 }}
          transition={{ duration: 0.15 }}
        >
          <h2 className="lock-screen__title">{title}</h2>
          <p className="lock-screen__subtitle">{subtitle}</p>
        </motion.div>
      </AnimatePresence>

      <motion.div
        className="pin-dots"
        animate={mismatch || (showError && error) ? { x: [0, -8, 8, -8, 8, 0] } : {}}
        transition={{ duration: 0.35 }}
      >
        {Array.from({ length: PIN_LENGTH }).map((_, i) => (
          <span
            key={i}
            className={`pin-dot ${i < pin.length ? "is-filled" : ""} ${mismatch || (showError && error) ? "is-error" : ""}`}
          />
        ))}
      </motion.div>

      {((showError && error) || mismatch) && (
        <p className="lock-screen__error">
          {mismatch ? "PINs don't match — try again." : error}
        </p>
      )}
      {loading && <p className="lock-screen__loading">Checking…</p>}

      <div className="pin-pad">
        {[1, 2, 3, 4, 5, 6, 7, 8, 9].map((d) => (
          <motion.button
            key={d}
            type="button"
            className="pin-pad__key"
            whileTap={{ scale: 0.92 }}
            onClick={() => handleDigit(String(d))}
            disabled={loading}
          >
            {d}
          </motion.button>
        ))}
        <span />
        <motion.button
          type="button"
          className="pin-pad__key"
          whileTap={{ scale: 0.92 }}
          onClick={() => handleDigit("0")}
          disabled={loading}
        >
          0
        </motion.button>
        <motion.button
          type="button"
          className="pin-pad__key pin-pad__key--action"
          whileTap={{ scale: 0.92 }}
          onClick={handleBackspace}
          disabled={loading || pin.length === 0}
          aria-label="Backspace"
        >
          <Delete size={18} strokeWidth={2} />
        </motion.button>
      </div>
    </div>
  );
}
