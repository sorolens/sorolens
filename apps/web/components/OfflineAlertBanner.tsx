"use client";

import { useEffect, useState } from "react";
import type { QueuedAlert } from "@/lib/alertQueue";

interface Props {
  pending: QueuedAlert[];
  isOffline: boolean;
  onDismiss: (id: string) => void;
  onDismissAll: () => void;
}

const SEVERITY_COLORS = {
  Critical: {
    bg: "rgba(239,68,68,0.12)",
    border: "rgba(239,68,68,0.4)",
    badge: "#ef4444",
    text: "#fca5a5",
    dot: "#ef4444",
  },
  Warning: {
    bg: "rgba(234,179,8,0.10)",
    border: "rgba(234,179,8,0.35)",
    badge: "#eab308",
    text: "#fde68a",
    dot: "#eab308",
  },
  Info: {
    bg: "rgba(99,102,241,0.10)",
    border: "rgba(99,102,241,0.3)",
    badge: "#6366f1",
    text: "#c7d2fe",
    dot: "#6366f1",
  },
};

export function OfflineAlertBanner({
  pending,
  isOffline,
  onDismiss,
  onDismissAll,
}: Props) {
  const [expanded, setExpanded] = useState(false);

  // Auto-expand when new critical alerts arrive while offline
  useEffect(() => {
    if (isOffline && pending.some((a) => a.severity === "Critical")) {
      setExpanded(true);
    }
  }, [isOffline, pending]);

  if (!isOffline && pending.length === 0) return null;

  const hasPending = pending.length > 0;

  return (
    <div
      id="offline-alert-banner"
      role="alert"
      aria-live="polite"
      style={{
        position: "fixed",
        bottom: "1.5rem",
        right: "1.5rem",
        zIndex: 9999,
        width: "min(380px, calc(100vw - 2rem))",
        borderRadius: "0.75rem",
        overflow: "hidden",
        border: "1px solid rgba(124,58,237,0.35)",
        background: "rgba(13,17,23,0.92)",
        backdropFilter: "blur(16px)",
        boxShadow: "0 8px 32px rgba(0,0,0,0.6)",
        fontFamily: "inherit",
        transition: "box-shadow 0.2s",
      }}
    >
      {/* Header */}
      <button
        id="offline-banner-toggle"
        onClick={() => setExpanded((v) => !v)}
        style={{
          width: "100%",
          display: "flex",
          alignItems: "center",
          gap: "0.6rem",
          padding: "0.75rem 1rem",
          background: "transparent",
          border: "none",
          cursor: "pointer",
          textAlign: "left",
          color: "#e2e8f0",
        }}
        aria-expanded={expanded}
        aria-controls="offline-banner-body"
      >
        {/* Status dot */}
        <span
          style={{
            width: "9px",
            height: "9px",
            borderRadius: "50%",
            flexShrink: 0,
            background: isOffline ? "#ef4444" : "#22c55e",
            boxShadow: isOffline
              ? "0 0 0 3px rgba(239,68,68,0.25)"
              : "0 0 0 3px rgba(34,197,94,0.25)",
            animation: isOffline
              ? "pulse-red 1.5s ease-in-out infinite"
              : "none",
          }}
        />
        <span style={{ fontWeight: 600, fontSize: "0.875rem", flex: 1 }}>
          {isOffline ? "Offline" : "Back online"}
          {hasPending && (
            <span
              style={{
                marginLeft: "0.5rem",
                background: "#7c3aed",
                color: "#fff",
                borderRadius: "999px",
                padding: "1px 7px",
                fontSize: "0.75rem",
                fontWeight: 700,
              }}
            >
              {pending.length}
            </span>
          )}
        </span>
        <span
          style={{
            fontSize: "0.75rem",
            color: "#94a3b8",
            transform: expanded ? "rotate(180deg)" : "rotate(0deg)",
            transition: "transform 0.2s",
            display: "inline-block",
          }}
          aria-hidden
        >
          ▼
        </span>
      </button>

      {/* Body */}
      {expanded && (
        <div
          id="offline-banner-body"
          style={{
            borderTop: "1px solid rgba(255,255,255,0.06)",
            maxHeight: "260px",
            overflowY: "auto",
            padding: hasPending ? "0" : "0.75rem 1rem",
          }}
        >
          {!hasPending && (
            <p style={{ fontSize: "0.8rem", color: "#64748b", margin: 0 }}>
              {isOffline
                ? "No alerts queued yet. New alerts will appear here while offline."
                : "No pending alerts."}
            </p>
          )}

          {hasPending && (
            <>
              <div style={{ padding: "0.5rem 1rem 0.25rem" }}>
                <button
                  id="dismiss-all-alerts"
                  onClick={onDismissAll}
                  style={{
                    fontSize: "0.72rem",
                    color: "#94a3b8",
                    background: "none",
                    border: "none",
                    cursor: "pointer",
                    padding: 0,
                    textDecoration: "underline",
                  }}
                >
                  Dismiss all
                </button>
              </div>

              <ul
                role="list"
                style={{ listStyle: "none", margin: 0, padding: "0 0 0.5rem" }}
              >
                {pending.map((alert) => {
                  const colors =
                    SEVERITY_COLORS[alert.severity] ?? SEVERITY_COLORS.Info;
                  return (
                    <li
                      key={alert.id}
                      id={`queued-alert-${alert.id}`}
                      style={{
                        display: "flex",
                        alignItems: "flex-start",
                        gap: "0.6rem",
                        padding: "0.6rem 1rem",
                        background: colors.bg,
                        borderLeft: `3px solid ${colors.border}`,
                        marginBottom: "1px",
                      }}
                    >
                      <span
                        style={{
                          width: "7px",
                          height: "7px",
                          borderRadius: "50%",
                          background: colors.dot,
                          flexShrink: 0,
                          marginTop: "5px",
                        }}
                      />
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div
                          style={{
                            display: "flex",
                            alignItems: "baseline",
                            gap: "0.4rem",
                            marginBottom: "2px",
                          }}
                        >
                          <span
                            style={{
                              fontSize: "0.68rem",
                              fontWeight: 700,
                              textTransform: "uppercase",
                              letterSpacing: "0.06em",
                              color: colors.badge,
                            }}
                          >
                            {alert.severity}
                          </span>
                          <span
                            style={{
                              fontSize: "0.68rem",
                              color: "#64748b",
                              overflow: "hidden",
                              textOverflow: "ellipsis",
                              whiteSpace: "nowrap",
                              maxWidth: "140px",
                            }}
                            title={alert.contractId}
                          >
                            {alert.contractId.slice(0, 8)}…
                          </span>
                        </div>
                        <p
                          style={{
                            margin: 0,
                            fontSize: "0.78rem",
                            color: colors.text,
                            lineHeight: 1.4,
                            wordBreak: "break-word",
                          }}
                        >
                          {alert.message}
                        </p>
                      </div>
                      <button
                        onClick={() => onDismiss(alert.id)}
                        aria-label={`Dismiss alert ${alert.id}`}
                        style={{
                          background: "none",
                          border: "none",
                          color: "#475569",
                          cursor: "pointer",
                          fontSize: "0.85rem",
                          padding: "2px 4px",
                          flexShrink: 0,
                          lineHeight: 1,
                        }}
                      >
                        ×
                      </button>
                    </li>
                  );
                })}
              </ul>
            </>
          )}
        </div>
      )}

      <style>{`
        @keyframes pulse-red {
          0%, 100% { box-shadow: 0 0 0 3px rgba(239,68,68,0.25); }
          50% { box-shadow: 0 0 0 6px rgba(239,68,68,0.05); }
        }
      `}</style>
    </div>
  );
}
