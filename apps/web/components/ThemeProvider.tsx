"use client";

import React, { createContext, useContext, useEffect, useState } from "react";

type Theme = "light" | "dark" | "system";

interface ThemeContextType {
  theme: Theme;
  setTheme: (theme: Theme) => void;
}

const ThemeContext = createContext<ThemeContextType | undefined>(undefined);

const THEME_KEY = "theme";

/**
 * Reads the stored theme. Called during the first client render, so the very
 * first paint already matches the inline script in the root layout instead of
 * starting from "system" and correcting itself a render later.
 */
function readStoredTheme(): Theme {
  // Server render and any environment without storage fall back to "system".
  if (typeof window === "undefined") return "system";
  try {
    const stored = window.localStorage.getItem(THEME_KEY);
    if (stored === "light" || stored === "dark" || stored === "system") {
      return stored;
    }
  } catch {
    // Storage can be unavailable (private mode, blocked cookies).
  }
  return "system";
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setTheme] = useState<Theme>(readStoredTheme);
  // Guards the localStorage write below until mount has settled. The read
  // happens during the first render, so nothing is pending at this point, but
  // the write must still not fire on mount: it would turn a user who has
  // never picked a theme into an explicit "system" preference.
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    setHydrated(true);
  }, []);

  useEffect(() => {
    const applyTheme = (currentTheme: Theme) => {
      const isDark =
        currentTheme === "dark" ||
        (currentTheme === "system" &&
          window.matchMedia("(prefers-color-scheme: dark)").matches);

      if (isDark) {
        document.documentElement.setAttribute("data-theme", "dark");
      } else {
        document.documentElement.setAttribute("data-theme", "light");
      }
    };

    applyTheme(theme);
    if (hydrated) {
      try {
        window.localStorage.setItem(THEME_KEY, theme);
      } catch {
        // A failed write only costs persistence; the theme still applies.
      }
    }

    // Listen for system theme changes if set to system
    if (theme === "system") {
      const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
      const handleChange = () => applyTheme("system");
      mediaQuery.addEventListener("change", handleChange);
      return () => mediaQuery.removeEventListener("change", handleChange);
    }
  }, [theme, hydrated]);

  return (
    <ThemeContext.Provider value={{ theme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme() {
  const context = useContext(ThemeContext);
  if (context === undefined) {
    throw new Error("useTheme must be used within a ThemeProvider");
  }
  return context;
}
