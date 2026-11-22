import { EXTENSION_INSTALL_URL, EXTENSION_RELEASES_URL } from "@/lib/extension";

/**
 * Landing-page strip pointing at the browser extension (issue #764). The
 * extension is the cheapest way for a visitor to become a user — it tracks a
 * contract from any Stellar page without opening the dashboard — so it is
 * linked from the page that has no other way to find it.
 */
export function ExtensionCallout() {
  return (
    <section
      data-testid="extension-callout"
      aria-labelledby="extension-callout-heading"
      className="mx-auto mt-10 max-w-4xl rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-5"
    >
      <h2
        id="extension-callout-heading"
        className="text-base font-semibold text-[var(--color-text-primary)]"
      >
        Sorolens for your browser
      </h2>
      <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
        A Chrome and Firefox extension puts a{" "}
        <strong className="text-[var(--color-text-primary)]">
          Track on Sorolens
        </strong>{" "}
        button next to every Soroban contract ID on Stellar Expert and Stellar
        Lab, adds it to your watchlist in one click, and raises a toolbar alert
        when a contract you watch stops responding.
      </p>
      <div className="mt-4 flex flex-wrap gap-3">
        <a
          href={EXTENSION_INSTALL_URL}
          className="rounded-md bg-[var(--color-accent)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-primary)] transition-opacity hover:opacity-90"
          rel="noopener noreferrer"
        >
          Get the extension
        </a>
        <a
          href={EXTENSION_RELEASES_URL}
          className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)]"
          rel="noopener noreferrer"
        >
          Download a release build
        </a>
      </div>
    </section>
  );
}
