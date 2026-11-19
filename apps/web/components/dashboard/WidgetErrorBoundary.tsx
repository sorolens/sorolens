"use client";

import { Component, type ReactNode } from "react";

interface Props {
  title: string;
  children: ReactNode;
}

interface State {
  failed: boolean;
}

/**
 * Keeps one widget's render/lookup failure local to its card. The rest of the
 * dashboard keeps working, and the user can retry without a reload.
 */
export class WidgetErrorBoundary extends Component<Props, State> {
  state: State = { failed: false };

  static getDerivedStateFromError(): State {
    return { failed: true };
  }

  componentDidCatch(error: unknown) {
    console.error(`Widget "${this.props.title}" failed`, error);
  }

  render() {
    if (this.state.failed) {
      return (
        <div
          data-testid="widget-error"
          role="alert"
          className="rounded-lg border border-[var(--color-danger)] p-4 text-sm text-[var(--color-text-secondary)]"
        >
          <p className="font-medium text-[var(--color-danger)]">
            This widget could not be displayed.
          </p>
          <button
            type="button"
            className="mt-2 rounded border border-[var(--color-border)] px-2 py-1 text-xs hover:border-[var(--color-accent)]"
            onClick={() => this.setState({ failed: false })}
          >
            Retry
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
