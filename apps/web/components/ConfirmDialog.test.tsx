import * as matchers from "@testing-library/jest-dom/matchers";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { describe, it, expect, vi, afterEach } from "vitest";
import { ConfirmDialog } from "./ConfirmDialog";

expect.extend(matchers);

describe("ConfirmDialog", () => {
  afterEach(() => {
    cleanup();
  });

  it("does not render when isOpen is false", () => {
    render(
      <ConfirmDialog
        isOpen={false}
        title="Delete Item"
        description="Are you sure?"
        onConfirm={vi.fn()}
        onClose={vi.fn()}
      />
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("renders with correct ARIA attributes when isOpen is true", () => {
    render(
      <ConfirmDialog
        isOpen={true}
        title="Revoke Key"
        description="Are you sure you want to revoke this key?"
        confirmText="Revoke"
        onConfirm={vi.fn()}
        onClose={vi.fn()}
      />
    );

    const dialog = screen.getByRole("dialog");
    expect(dialog).toBeInTheDocument();
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(dialog).toHaveAttribute("aria-labelledby", "confirm-dialog-title");
    expect(dialog).toHaveAttribute("aria-describedby", "confirm-dialog-desc");
    expect(screen.getByText("Revoke Key")).toBeInTheDocument();
    expect(
      screen.getByText("Are you sure you want to revoke this key?")
    ).toBeInTheDocument();
  });

  it("calls onClose when Cancel button is clicked", () => {
    const handleClose = vi.fn();
    render(
      <ConfirmDialog
        isOpen={true}
        title="Delete Rule"
        description="Are you sure?"
        onConfirm={vi.fn()}
        onClose={handleClose}
      />
    );

    fireEvent.click(screen.getByTestId("confirm-dialog-cancel"));
    expect(handleClose).toHaveBeenCalledTimes(1);
  });

  it("calls onConfirm when Confirm button is clicked", () => {
    const handleConfirm = vi.fn();
    render(
      <ConfirmDialog
        isOpen={true}
        title="Delete Rule"
        description="Are you sure?"
        confirmText="Delete"
        onConfirm={handleConfirm}
        onClose={vi.fn()}
      />
    );

    fireEvent.click(screen.getByTestId("confirm-dialog-confirm"));
    expect(handleConfirm).toHaveBeenCalledTimes(1);
  });

  it("calls onClose on Escape key press", () => {
    const handleClose = vi.fn();
    render(
      <ConfirmDialog
        isOpen={true}
        title="Delete Rule"
        description="Are you sure?"
        onConfirm={vi.fn()}
        onClose={handleClose}
      />
    );

    fireEvent.keyDown(document, { key: "Escape" });
    expect(handleClose).toHaveBeenCalledTimes(1);
  });

  it("calls onClose when backdrop is clicked", () => {
    const handleClose = vi.fn();
    render(
      <ConfirmDialog
        isOpen={true}
        title="Delete Rule"
        description="Are you sure?"
        onConfirm={vi.fn()}
        onClose={handleClose}
      />
    );

    fireEvent.click(screen.getByTestId("confirm-dialog-backdrop"));
    expect(handleClose).toHaveBeenCalledTimes(1);
  });

  it("disables buttons and shows loading text when isLoading is true", () => {
    render(
      <ConfirmDialog
        isOpen={true}
        title="Deleting..."
        description="Please wait"
        isLoading={true}
        onConfirm={vi.fn()}
        onClose={vi.fn()}
      />
    );

    const cancelBtn = screen.getByTestId("confirm-dialog-cancel");
    const confirmBtn = screen.getByTestId("confirm-dialog-confirm");
    expect(cancelBtn).toBeDisabled();
    expect(confirmBtn).toBeDisabled();
    expect(confirmBtn).toHaveTextContent("Processing…");
  });
});
