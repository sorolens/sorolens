"use client";

import { useTheme } from "./ThemeProvider";
import { useEffect, useState } from "react";

export function ThemeToggle() {
  const { theme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) {
    // Avoid layout shift by reserving space, but rendering empty initially
    return <div className="h-8 w-24" />;
  }

  return (
    <div
      role="group"
      aria-label="Theme selector"
      className="flex items-center gap-1 rounded-full border border-[var(--color-border)] bg-[var(--color-bg-card)] p-1 text-sm"
    >
      <button
        onClick={() => setTheme("light")}
        aria-pressed={theme === "light"}
        className={`rounded-full px-2 py-1 transition-colors ${
          theme === "light"
            ? "bg-[var(--color-bg-page)] font-medium text-[var(--color-text-primary)]"
            : "text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
        }`}
        aria-label="Light mode"
      >
        Light
      </button>
      <button
        onClick={() => setTheme("dark")}
        aria-pressed={theme === "dark"}
        className={`rounded-full px-2 py-1 transition-colors ${
          theme === "dark"
            ? "bg-[var(--color-bg-page)] font-medium text-[var(--color-text-primary)]"
            : "text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
        }`}
        aria-label="Dark mode"
      >
        Dark
      </button>
      <button
        onClick={() => setTheme("system")}
        aria-pressed={theme === "system"}
        className={`rounded-full px-2 py-1 transition-colors ${
          theme === "system"
            ? "bg-[var(--color-bg-page)] font-medium text-[var(--color-text-primary)]"
            : "text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
        }`}
        aria-label="System mode"
      >
        System
      </button>
    </div>
  );
}
