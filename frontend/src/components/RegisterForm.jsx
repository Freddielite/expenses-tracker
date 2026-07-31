import { useState } from "react";
import { motion } from "framer-motion";
import { UserPlus } from "lucide-react";

const MIN_PASSWORD_LENGTH = 8;

// A real account (email + password), separate from the numeric owner/
// guest PIN handled by LockScreen. Kept as its own component rather than
// a third LockScreen mode since the input shape (text fields, not a
// digit pad) and validation (email format, password confirmation) don't
// share much with the PIN flow.
export default function RegisterForm({ onSubmit, onBack, error, loading }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [localError, setLocalError] = useState(null);

  const shownError = localError || error;

  async function handleSubmit(e) {
    e.preventDefault();
    setLocalError(null);

    if (password.length < MIN_PASSWORD_LENGTH) {
      setLocalError(`Password must be at least ${MIN_PASSWORD_LENGTH} characters.`);
      return;
    }
    if (password !== confirmPassword) {
      setLocalError("Passwords don't match.");
      return;
    }
    await onSubmit(email, password);
  }

  return (
    <div className="lock-screen">
      <div className="lock-screen__icon">
        <UserPlus size={26} strokeWidth={1.5} />
      </div>

      <h2 className="lock-screen__title">Create an account</h2>
      <p className="lock-screen__subtitle">Sign up with an email and password.</p>

      <form className="manage-form" onSubmit={handleSubmit} style={{ width: "100%" }}>
        <input
          type="email"
          placeholder="Email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          autoComplete="email"
          disabled={loading}
          required
        />
        <input
          type="password"
          placeholder="Password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="new-password"
          disabled={loading}
          required
        />
        <input
          type="password"
          placeholder="Confirm password"
          value={confirmPassword}
          onChange={(e) => setConfirmPassword(e.target.value)}
          autoComplete="new-password"
          disabled={loading}
          required
        />

        {shownError && <p className="manage-form__error">{shownError}</p>}

        <motion.button
          type="submit"
          className="manage-form__submit"
          whileTap={{ scale: 0.97 }}
          disabled={loading}
        >
          {loading ? "Creating account…" : "Create account"}
        </motion.button>
      </form>

      <button type="button" className="lock-screen__link" onClick={onBack} disabled={loading}>
        Have an account? Login
      </button>
    </div>
  );
}
