import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ContractNotes } from "./ContractNotes";
import type { ContractNote } from "@/lib/types";

expect.extend(matchers);

const AUTHOR = "user-notes-author";
const OTHER = "user-notes-someone-else";

function note(overrides: Partial<ContractNote> = {}): ContractNote {
  return {
    id: "3f1c8f5e-0000-4000-8000-000000000001",
    contract_id: "CAAA",
    author: AUTHOR,
    body: "Migrated from v1 on 2026-08-10.",
    created_at: "2026-09-27T10:00:00Z",
    updated_at: "2026-09-27T10:00:00Z",
    ...overrides,
  };
}

function renderNotes(notes: ContractNote[], currentUserId = AUTHOR) {
  const handlers = {
    onCreate: vi.fn(),
    onUpdate: vi.fn(),
    onDelete: vi.fn(),
  };
  const view = render(
    <ContractNotes
      notes={notes}
      currentUserId={currentUserId}
      onCreate={handlers.onCreate}
      onUpdate={handlers.onUpdate}
      onDelete={handlers.onDelete}
    />
  );
  return { ...handlers, container: view.container };
}

describe("ContractNotes", () => {
  afterEach(cleanup);

  it("renders a note body as markdown", () => {
    renderNotes([note({ body: "**owner:** the payments team" })]);

    const body = screen.getByTestId("note-body");
    expect(body.innerHTML).toContain("<strong>owner:</strong>");
  });

  it("escapes raw HTML in a note instead of interpreting it", () => {
    const { container } = renderNotes([
      note({ body: '<img src=x onerror="alert(1)">' }),
    ]);

    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("script")).toBeNull();
    expect(screen.getByTestId("note-body").innerHTML).toContain("&lt;img");
  });

  it("escapes a script tag a note tries to inject", () => {
    const { container } = renderNotes([
      note({ body: "<script>alert(1)</script>" }),
    ]);

    expect(container.querySelector("script")).toBeNull();
    expect(screen.getByTestId("note-body").innerHTML).toContain(
      "&lt;script&gt;"
    );
  });

  it("offers edit and delete only on the viewer's own notes", () => {
    renderNotes([note({ author: OTHER })], AUTHOR);

    expect(screen.queryByTestId("note-edit")).toBeNull();
    expect(screen.queryByTestId("note-delete")).toBeNull();
  });

  it("shows edit and delete on the viewer's own notes", () => {
    renderNotes([note()], AUTHOR);

    expect(screen.getByTestId("note-edit")).toBeDefined();
    expect(screen.getByTestId("note-delete")).toBeDefined();
  });

  it("submits the composer body verbatim", () => {
    const { onCreate } = renderNotes([]);

    fireEvent.change(screen.getByTestId("note-input"), {
      target: { value: "## Migration\n\nMoved from v1." },
    });
    fireEvent.click(screen.getByTestId("note-submit"));

    expect(onCreate).toHaveBeenCalledWith("## Migration\n\nMoved from v1.");
    // The composer is cleared so the next note starts blank.
    expect(
      (screen.getByTestId("note-input") as HTMLTextAreaElement).value
    ).toBe("");
  });

  it("rejects an empty composer without calling the API", () => {
    const { onCreate } = renderNotes([]);

    fireEvent.change(screen.getByTestId("note-input"), {
      target: { value: "   \n  " },
    });
    fireEvent.click(screen.getByTestId("note-submit"));

    expect(onCreate).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toBeDefined();
  });

  it("rejects a body longer than the API limit", () => {
    const { onCreate } = renderNotes([]);

    fireEvent.change(screen.getByTestId("note-input"), {
      target: { value: "a".repeat(8193) },
    });
    fireEvent.click(screen.getByTestId("note-submit"));

    expect(onCreate).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toBeDefined();
  });

  it("edits a note in place and saves the new body", () => {
    const n = note();
    const { onUpdate } = renderNotes([n]);

    fireEvent.click(screen.getByTestId("note-edit"));
    fireEvent.change(screen.getByTestId("note-edit-input"), {
      target: { value: "corrected history" },
    });
    fireEvent.click(screen.getByTestId("note-save"));

    expect(onUpdate).toHaveBeenCalledWith(n.id, "corrected history");
  });

  it("cancels an edit without calling the API", () => {
    const { onUpdate } = renderNotes([note()]);

    fireEvent.click(screen.getByTestId("note-edit"));
    fireEvent.click(screen.getByTestId("note-cancel"));

    expect(onUpdate).not.toHaveBeenCalled();
    expect(screen.getByTestId("note-body")).toBeDefined();
  });

  it("deletes a note", () => {
    const n = note();
    const { onDelete } = renderNotes([n]);

    fireEvent.click(screen.getByTestId("note-delete"));

    expect(onDelete).toHaveBeenCalledWith(n.id);
  });

  it("surfaces a server error passed by the parent", () => {
    render(
      <ContractNotes
        notes={[]}
        currentUserId={AUTHOR}
        error="Only the author of a note may change it."
        onCreate={vi.fn()}
        onUpdate={vi.fn()}
        onDelete={vi.fn()}
      />
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Only the author of a note may change it."
    );
  });

  it("renders an empty state and a loading state", () => {
    renderNotes([]);
    expect(screen.getByText(/No notes yet/i)).toBeDefined();

    cleanup();
    render(
      <ContractNotes
        notes={[]}
        loading
        currentUserId={AUTHOR}
        onCreate={vi.fn()}
        onUpdate={vi.fn()}
        onDelete={vi.fn()}
      />
    );
    expect(screen.getByText(/Loading notes/i)).toBeDefined();
  });
});
