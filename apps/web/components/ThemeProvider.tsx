"use client";

import React, { createContext, useContext, useEffect, useState } from "react";

type Theme = "light" | "dark" | "system";

interface ThemeContextType {
  theme: Theme;
  setTheme: (theme: Theme) => void;
}

const ThemeContext = createContext<ThemeContextType | undefined>(undefined);

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setTheme] = useState<Theme>("system");
  // Guards the localStorage write below until the initial read (the effect
  // right under this one) has run. Without it, the write fires on mount with
  // the "system" default before the saved value has been read back into
  // state, clobbering it - fine normally, since the effect re-runs once the
  // read lands, but React Strict Mode's mount/unmount/remount in development
  // can discard that pending update before it commits, so the remount reads
  // back the clobbered value and the real preference never sticks.
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    // Read theme from localStorage on mount
    const savedTheme = localStorage.getItem("theme") as Theme | null;
    if (savedTheme) {
      setTheme(savedTheme);
    }
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
      localStorage.setItem("theme", theme);
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
