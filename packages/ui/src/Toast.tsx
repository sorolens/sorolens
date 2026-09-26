import { useEffect, useRef } from "react";
import type { ReactNode } from "react";

export type ToastVariant = "info" | "success" | "error";

export interface ToastProps {
  message: ReactNode;
  /** Called when the timeout elapses or the user dismisses the toast. */
  onDismiss: () => void;
  variant?: ToastVariant;
  /** Milliseconds before the toast dismisses itself. */
  duration?: number;
}

// Falls back to the dashboard palette when the CSS tokens are not defined.
const accentColors: Record<ToastVariant, string> = {
  info: "var(--color-accent, #89b4fa)",
  success: "var(--color-safe, #22c55e)",
  error: "var(--color-danger, #ef4444)",
};

/**
 * A single self-dismissing notification, fixed to the bottom-right corner.
 * Errors are announced assertively (role="alert"); everything else politely
 * (role="status"). To show a new toast with the same message, change `key`.
 */
export function Toast({
  message,
  onDismiss,
  variant = "info",
  duration = 5000,
}: ToastProps) {
  // Keep the latest callback without restarting the timer on every render.
  const onDismissRef = useRef(onDismiss);
  useEffect(() => {
    onDismissRef.current = onDismiss;
  });

  useEffect(() => {
    const timer = setTimeout(() => onDismissRef.current(), duration);
    return () => clearTimeout(timer);
  }, [duration]);

  const isError = variant === "error";

  return (
    <div
      role={isError ? "alert" : "status"}
      aria-live={isError ? "assertive" : "polite"}
      aria-atomic="true"
      data-variant={variant}
      style={{
        alignItems: "flex-start",
        backgroundColor: "var(--color-bg-card, #1e1e2e)",
        border: "1px solid var(--color-border, #313244)",
        borderLeft: `4px solid ${accentColors[variant]}`,
        borderRadius: "0.5rem",
        bottom: "1.5rem",
        boxShadow: "0 10px 25px rgba(0, 0, 0, 0.4)",
        color: "var(--color-text-primary, #cdd6f4)",
        display: "flex",
        fontSize: "0.875rem",
        gap: "0.75rem",
        lineHeight: 1.4,
        maxWidth: "24rem",
        padding: "0.75rem 1rem",
        position: "fixed",
        right: "1.5rem",
        zIndex: 60,
      }}
    >
      <span style={{ flex: 1 }}>{message}</span>
      <button
        type="button"
        onClick={() => onDismissRef.current()}
        aria-label="Dismiss notification"
        style={{
          background: "none",
          border: "none",
          color: "var(--color-text-secondary, #a6adc8)",
          cursor: "pointer",
          lineHeight: 1,
          padding: 0,
        }}
      >
        ✕
      </button>
    </div>
  );
}
