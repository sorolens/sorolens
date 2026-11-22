"use client";

import { useTheme } from "./ThemeProvider";
import { useEffect, useState } from "react";

const OPTIONS = [
  { value: "light", label: "Light mode", text: "Light" },
  { value: "dark", label: "Dark mode", text: "Dark" },
  { value: "system", label: "System mode", text: "System" },
] as const;

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
      aria-label="Color theme"
      className="flex items-center gap-1 rounded-full border border-[var(--color-border)] bg-[var(--color-bg-card)] p-1 text-sm"
    >
      {OPTIONS.map((option) => {
        const isActive = theme === option.value;
        return (
          <button
            key={option.value}
            type="button"
            onClick={() => setTheme(option.value)}
            aria-label={option.label}
            aria-pressed={isActive}
            className={`rounded-full px-2 py-1 transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent)] ${
              isActive
                ? "bg-[var(--color-bg-page)] font-medium text-[var(--color-text-primary)]"
                : "text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
            }`}
          >
            {option.text}
          </button>
        );
      })}
    </div>
  );
}
