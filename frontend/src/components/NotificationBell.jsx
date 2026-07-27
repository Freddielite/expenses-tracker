import { motion } from "framer-motion";
import { Bell, BellOff, BellRing } from "lucide-react";

// Lets the user opt in to real browser push notifications (budget alerts,
// recurring transactions landing, etc. while the tab isn't focused). In-app
// toasts work regardless of this — this only controls the OS-level pushes.
export default function NotificationBell({ permission, onRequest }) {
  if (permission === "unsupported") return null;

  if (permission === "granted") {
    return (
      <span
        className="theme-toggle theme-toggle--static"
        title="Push notifications are on"
        aria-label="Push notifications are on"
      >
        <BellRing size={13} strokeWidth={2} />
      </span>
    );
  }

  if (permission === "denied") {
    return (
      <span
        className="theme-toggle theme-toggle--static"
        title="Notifications blocked — enable them in your browser's site settings"
        aria-label="Push notifications blocked"
      >
        <BellOff size={13} strokeWidth={2} />
      </span>
    );
  }

  return (
    <motion.button
      type="button"
      className="theme-toggle"
      onClick={onRequest}
      whileTap={{ scale: 0.88 }}
      aria-label="Enable push notifications"
      title="Enable push notifications"
    >
      <Bell size={13} strokeWidth={2} />
    </motion.button>
  );
}
