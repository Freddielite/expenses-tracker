import { motion, AnimatePresence } from "framer-motion";
import { CheckCircle2, AlertTriangle, XCircle, Info, X } from "lucide-react";

const ICONS = {
  success: CheckCircle2,
  warning: AlertTriangle,
  error: XCircle,
  info: Info,
};

export default function ToastStack({ toasts, onDismiss }) {
  return (
    <div className="toast-stack" role="status" aria-live="polite">
      <AnimatePresence initial={false}>
        {toasts.map((t) => {
          const Icon = ICONS[t.type] || Info;
          return (
            <motion.div
              key={t.id}
              className={`toast toast--${t.type}`}
              layout
              initial={{ opacity: 0, y: -8, scale: 0.96 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, x: 40, transition: { duration: 0.15 } }}
              transition={{ type: "spring", stiffness: 420, damping: 32 }}
            >
              <Icon size={16} strokeWidth={2.25} className="toast__icon" />
              <div className="toast__body">
                <p className="toast__title">{t.title}</p>
                {t.message && <p className="toast__message">{t.message}</p>}
              </div>
              {t.action && (
                <button
                  type="button"
                  className="toast__action"
                  onClick={() => {
                    t.action.onClick();
                    onDismiss(t.id);
                  }}
                >
                  {t.action.label}
                </button>
              )}
              <button
                type="button"
                className="toast__close"
                onClick={() => onDismiss(t.id)}
                aria-label="Dismiss notification"
              >
                <X size={13} strokeWidth={2.5} />
              </button>
            </motion.div>
          );
        })}
      </AnimatePresence>
    </div>
  );
}
