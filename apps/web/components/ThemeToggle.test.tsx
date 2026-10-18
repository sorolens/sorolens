import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { ThemeToggle } from "./ThemeToggle";

const mockSetTheme = vi.fn();
let mockTheme: "system" | "light" | "dark" = "system";

vi.mock("./ThemeProvider", () => ({
  useTheme: () => ({
    theme: mockTheme,
    setTheme: mockSetTheme,
  }),
}));

describe("ThemeToggle", () => {
  beforeEach(() => {
    mockTheme = "system";
    mockSetTheme.mockClear();
  });

  it("renders a labelled group of theme buttons", () => {
    render(<ThemeToggle />);

    const group = screen.getByRole("group", { name: /theme selection/i });
    expect(group).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /light mode/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /dark mode/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /system mode/i })).toBeInTheDocument();
  });

  it("sets aria-pressed true only on the active theme", () => {
    mockTheme = "dark";
    render(<ThemeToggle />);

    expect(screen.getButtonName(/dark mode/i)).toHaveAttribute("aria-pressed", "true");
    expect(screen.getButtonName(/light mode/i)).toHaveAttribute("aria-pressed", "false");
    expect(screen.getButtonName(/system mode/i)).toHaveAttribute("aria-pressed", "false");
  });

  it("calls setTheme when a theme button is clicked", () => {
    render(<ThemeToggle />);

    fireEvent.click(screen.getButtonName(/light mode/i));
    expect(mockSetTheme).toHaveBeenCalledWith("light");

    fireEvent.click(screen.getButtonName(/dark mode/i));
    expect(mockSetTheme).toHaveBeenCalledWith("dark");

    fireEvent.click(screen.getButtonName(/system mode/i));
    expect(mockSetTheme).toHaveBeenCalledWith("system");
  });
});
