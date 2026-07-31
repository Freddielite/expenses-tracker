import { useCallback, useEffect, useRef, useState } from "react";

// Auto-dismiss timing per toast type — errors/warnings linger longer since
// they're more likely to need actually reading.
const DISMISS_MS = {
  success: 4000,
  info: 4000,
  warning: 6500,
  error: 7000,
};

let nextId = 1;

// Central notification system: in-app toasts (always) + real browser
// Notification API pushes (only when the tab isn't focused, and only once
// the user has granted permission) — so nothing gets shown twice while
// you're actually looking at the app, but you still hear about it if you've
// tabbed away.
export function useNotifications() {
  const [toasts, setToasts] = useState([]);
  const [permission, setPermission] = useState(
    typeof Notification !== "undefined" ? Notification.permission : "unsupported"
  );
  const timers = useRef(new Map());

  useEffect(() => {
    const timerMap = timers.current;
    return () => {
      for (const t of timerMap.values()) clearTimeout(t);
      timerMap.clear();
    };
  }, []);

  const dismiss = useCallback((id) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
    const timer = timers.current.get(id);
    if (timer) {
      clearTimeout(timer);
      timers.current.delete(id);
    }
  }, []);

  const notify = useCallback(
    (type, title, message, opts = {}) => {
      const id = nextId++;
      setToasts((prev) => [...prev, { id, type, title, message, action: opts.action }]);
      const ms = opts.stayOpen ? null : opts.durationMs ?? DISMISS_MS[type] ?? 5000;
      if (ms) {
        timers.current.set(
          id,
          setTimeout(() => dismiss(id), ms)
        );
      }

      // Only bother with a real OS-level notification if the user has
      // granted permission and has actually navigated away — while the tab
      // is focused the toast above is enough, and firing both is noisy.
      if (
        typeof document !== "undefined" &&
        document.hidden &&
        typeof Notification !== "undefined" &&
        Notification.permission === "granted"
      ) {
        try {
          const n = new Notification(title, {
            body: message,
            icon: "/favicon.svg",
            tag: opts.tag,
          });
          n.onclick = () => {
            window.focus();
            n.close();
          };
        } catch {
          // Some browsers (mobile Safari, etc.) throw on `new Notification`
          // outside a service worker — fail silently, the toast still shows.
        }
      }

      return id;
    },
    [dismiss]
  );

  const requestPermission = useCallback(async () => {
    if (typeof Notification === "undefined") {
      setPermission("unsupported");
      return "unsupported";
    }
    const result = await Notification.requestPermission();
    setPermission(result);
    return result;
  }, []);

  return { toasts, notify, dismiss, permission, requestPermission };
}
