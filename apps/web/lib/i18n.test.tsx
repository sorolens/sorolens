import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { LocalePreferences } from "@/components/LocalePreferences";
import { LocaleProvider, useLocale } from "./i18n";

function FormattingProbe() {
  const { formatNumber } = useLocale();
  return <p data-testid="formatted-number">{formatNumber(1_234_567)}</p>;
}

describe("locale preferences", () => {
  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  it("updates shared number formatting and persists the selected locale", () => {
    render(
      <LocaleProvider>
        <LocalePreferences />
        <FormattingProbe />
      </LocaleProvider>
    );

    expect(screen.getByTestId("formatted-number").textContent).toBe(
      "1,234,567"
    );
    fireEvent.change(screen.getByLabelText("Language"), {
      target: { value: "es" },
    });

    expect(screen.getByTestId("formatted-number").textContent).toBe(
      new Intl.NumberFormat("es").format(1_234_567)
    );
    expect(localStorage.getItem("sorolens_locale")).toBe("es");
  });

  it("serves list empty-state copy from the catalog in both locales", async () => {
    function CopyProbe() {
      const { t } = useLocale();
      return <p data-testid="copy">{t("watchlist.empty.title")}</p>;
    }

    localStorage.setItem("sorolens_locale", "es");
    render(
      <LocaleProvider>
        <CopyProbe />
      </LocaleProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("copy").textContent).toBe(
        "Aún no hay marcadores"
      )
    );
  });

  it("falls back to English outside a LocaleProvider", () => {
    function CopyProbe() {
      const { t } = useLocale();
      return <p data-testid="copy">{t("watchlist.empty.title")}</p>;
    }

    render(<CopyProbe />);

    expect(screen.getByTestId("copy").textContent).toBe("No bookmarks yet");
  });
});
