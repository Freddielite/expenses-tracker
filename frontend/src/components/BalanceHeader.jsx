import { AreaChart, Area, ResponsiveContainer } from "recharts";
import { TrendingUp, TrendingDown, LockKeyhole } from "lucide-react";
import { formatCurrency } from "../format";
import { useAnimatedNumber } from "../useAnimatedNumber";
import ThemeToggle from "./ThemeToggle";
import NotificationBell from "./NotificationBell";

export default function BalanceHeader({
  totalIncome,
  totalExpense,
  net,
  monthlyTrend,
  theme,
  onToggleTheme,
  onLock,
  notificationPermission,
  onRequestNotifications,
  isGuest = false,
}) {
  const animatedNet = useAnimatedNumber(net);
  const animatedIncome = useAnimatedNumber(totalIncome);
  const animatedExpense = useAnimatedNumber(totalExpense);

  const isPositive = net >= 0;
  const hasTrend = monthlyTrend && monthlyTrend.length > 1;

  return (
    <header className="balance-header">
      <div className="balance-header__glow" />
      <div className="balance-header__top">
        <div className="balance-header__eyebrow">
          Ledger
          {isGuest && <span className="balance-header__badge" style={{ marginLeft: 8 }}>View only</span>}
        </div>
        <div className="balance-header__top-right">
          {isPositive ? (
            <span className="balance-header__badge is-jade">
              <TrendingUp size={13} strokeWidth={2.5} />
              In the black
            </span>
          ) : (
            <span className="balance-header__badge is-rust">
              <TrendingDown size={13} strokeWidth={2.5} />
              In the red
            </span>
          )}
          <ThemeToggle theme={theme} onToggle={onToggleTheme} />
          {notificationPermission && (
            <NotificationBell
              permission={notificationPermission}
              onRequest={onRequestNotifications}
            />
          )}
          {onLock && (
            <button
              type="button"
              className="theme-toggle"
              onClick={onLock}
              aria-label="Lock the app"
              title="Lock"
            >
              <LockKeyhole size={13} strokeWidth={2} />
            </button>
          )}
        </div>
      </div>

      <div className="balance-header__main">
        <span className="balance-header__label">Net balance</span>
        <span className={`balance-header__amount ${isPositive ? "is-positive" : "is-negative"}`}>
          {formatCurrency(animatedNet)}
        </span>
      </div>

      {hasTrend && (
        <div className="balance-header__sparkline">
          <ResponsiveContainer width="100%" height={44}>
            <AreaChart data={monthlyTrend} margin={{ top: 4, right: 0, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="netGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="var(--gold)" stopOpacity={0.5} />
                  <stop offset="100%" stopColor="var(--gold)" stopOpacity={0} />
                </linearGradient>
              </defs>
              <Area
                type="monotone"
                dataKey="net"
                stroke="var(--gold)"
                strokeWidth={2}
                fill="url(#netGradient)"
                isAnimationActive={true}
                animationDuration={700}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}

      <div className="balance-header__row">
        <div className="balance-header__stat">
          <span className="dot dot--jade" />
          <span>In</span>
          <span className="mono">{formatCurrency(animatedIncome)}</span>
        </div>
        <div className="balance-header__stat">
          <span className="dot dot--rust" />
          <span>Out</span>
          <span className="mono">{formatCurrency(animatedExpense)}</span>
        </div>
      </div>
    </header>
  );
}
