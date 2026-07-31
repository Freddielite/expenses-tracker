import { useState } from "react";
import { motion } from "framer-motion";
import { LogIn } from "lucide-react";

// Signs an existing registered account back in — the counterpart to
// RegisterForm. Kept separate rather than a mode flag on RegisterForm
// since the fields (no confirm-password) and copy differ enough that
// sharing one component would mostly be conditionals.
export default function LoginForm({ onSubmit, error, loading }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  async function handleSubmit(e) {
    e.preventDefault();
    await onSubmit(email, password);
  }

  return (
    <div className="lock-screen">
      <div className="lock-screen__icon">
        <LogIn size={26} strokeWidth={1.5} />
      </div>

      <h2 className="lock-screen__title">Log in</h2>
      <p className="lock-screen__subtitle">Sign in with your email and password.</p>

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
          autoComplete="current-password"
          disabled={loading}
          required
        />

        {error && <p className="manage-form__error">{error}</p>}

        <motion.button
          type="submit"
          className="manage-form__submit"
          whileTap={{ scale: 0.97 }}
          disabled={loading}
        >
          {loading ? "Logging in…" : "Log in"}
        </motion.button>
      </form>
    </div>
  );
}
