"use client";

import { useState } from "react";
import type { ContractNote } from "@/lib/types";
import { renderMarkdown } from "@/lib/markdown";

/** Mirrors the API limit (store.MaxContractNoteBodyLen). */
const MAX_NOTE_LENGTH = 8192;

const EMPTY_DRAFT_MESSAGE = "Write something before saving the note.";
const TOO_LONG_MESSAGE = `Notes are limited to ${MAX_NOTE_LENGTH} characters.`;

interface ContractNotesProps {
  /** Notes for the contract, newest first, as returned by the API. */
  notes: ContractNote[];
  loading?: boolean;
  /** Server-side error to surface above the list. */
  error?: string | null;
  /** True while a create/update/delete is in flight. */
  saving?: boolean;
  /**
   * Identity of the viewer. A note is only editable by the identity that
   * authored it, which is what the API enforces, so the controls are hidden
   * for everyone else.
   */
  currentUserId: string;
  onCreate: (body: string) => void;
  onUpdate: (id: string, body: string) => void;
  onDelete: (id: string) => void;
}

/**
 * Markdown notes for one contract (issue #164): a composer plus the note list.
 *
 * The component is presentational — the parent owns the notes and performs the
 * API calls — and it renders note bodies through `renderMarkdown`, which
 * escapes all input before emitting any tag of its own.
 */
export function ContractNotes({
  notes,
  loading = false,
  error = null,
  saving = false,
  currentUserId,
  onCreate,
  onUpdate,
  onDelete,
}: ContractNotesProps) {
  const [draft, setDraft] = useState("");
  const [draftError, setDraftError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editDraft, setEditDraft] = useState("");

  const submitDraft = () => {
    if (!draft.trim()) {
      setDraftError(EMPTY_DRAFT_MESSAGE);
      return;
    }
    if (draft.length > MAX_NOTE_LENGTH) {
      setDraftError(TOO_LONG_MESSAGE);
      return;
    }
    setDraftError(null);
    onCreate(draft);
    setDraft("");
  };

  const startEditing = (note: ContractNote) => {
    setEditingId(note.id);
    setEditDraft(note.body);
  };

  const submitEdit = (id: string) => {
    if (!editDraft.trim() || editDraft.length > MAX_NOTE_LENGTH) return;
    onUpdate(id, editDraft);
    setEditingId(null);
    setEditDraft("");
  };

  const message = draftError ?? error;

  return (
    <div data-testid="contract-notes">
      <div className="mb-4 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4">
        <label
          htmlFor="note-body"
          className="mb-2 block text-sm font-medium text-[var(--color-text-secondary)]"
        >
          Add a note (markdown)
        </label>
        <textarea
          id="note-body"
          data-testid="note-input"
          value={draft}
          disabled={saving}
          onChange={(e) => {
            setDraft(e.target.value);
            setDraftError(null);
          }}
          rows={4}
          spellCheck={false}
          placeholder="Why was this contract migrated? Who owns it?"
          className="w-full rounded-lg border border-[var(--color-border)] bg-black/30 px-3 py-2 text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
        />
        <div className="mt-2 flex items-center justify-between gap-3">
          <span className="text-xs text-[var(--color-text-secondary)]">
            {draft.length}/{MAX_NOTE_LENGTH}
          </span>
          <button
            type="button"
            data-testid="note-submit"
            onClick={submitDraft}
            disabled={saving}
            className="rounded-lg bg-[var(--color-accent)] px-3 py-1.5 text-sm font-medium text-black transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
          >
            Add note
          </button>
        </div>
        {message && (
          <p role="alert" className="mt-2 text-xs text-red-400">
            {message}
          </p>
        )}
      </div>

      {loading ? (
        <p className="text-sm text-[var(--color-text-secondary)]">
          Loading notes…
        </p>
      ) : notes.length === 0 ? (
        <p className="text-sm text-[var(--color-text-secondary)]">
          No notes yet. Institutional knowledge starts with the first one.
        </p>
      ) : (
        <ul className="space-y-3">
          {notes.map((note) => {
            const isAuthor =
              currentUserId !== "" && note.author === currentUserId;
            const isEditing = editingId === note.id;
            return (
              <li
                key={note.id}
                data-testid="note"
                className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4"
              >
                <div className="mb-2 flex items-start justify-between gap-3">
                  <div className="text-xs text-[var(--color-text-secondary)]">
                    <span className="font-medium text-[var(--color-text-primary)]">
                      {note.author}
                    </span>
                    {" · "}
                    <time dateTime={note.created_at}>
                      {new Date(note.created_at).toLocaleString()}
                    </time>
                    {note.updated_at !== note.created_at && (
                      <span className="ml-1">(edited)</span>
                    )}
                  </div>
                  {isAuthor && !isEditing && (
                    <div className="flex shrink-0 gap-2 text-xs">
                      <button
                        type="button"
                        data-testid="note-edit"
                        aria-label={`Edit note ${note.id}`}
                        disabled={saving}
                        onClick={() => startEditing(note)}
                        className="rounded px-2 py-1 text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-border)] hover:text-[var(--color-text-primary)] disabled:cursor-not-allowed"
                      >
                        Edit
                      </button>
                      <button
                        type="button"
                        data-testid="note-delete"
                        aria-label={`Delete note ${note.id}`}
                        disabled={saving}
                        onClick={() => onDelete(note.id)}
                        className="rounded px-2 py-1 text-red-400 transition-colors hover:bg-red-500/20 disabled:cursor-not-allowed"
                      >
                        Delete
                      </button>
                    </div>
                  )}
                </div>

                {isEditing ? (
                  <div>
                    <textarea
                      data-testid="note-edit-input"
                      aria-label="Edit note body"
                      value={editDraft}
                      rows={4}
                      spellCheck={false}
                      onChange={(e) => setEditDraft(e.target.value)}
                      className="w-full rounded-lg border border-[var(--color-border)] bg-black/30 px-3 py-2 text-sm text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-none"
                    />
                    <div className="mt-2 flex gap-2">
                      <button
                        type="button"
                        data-testid="note-save"
                        disabled={saving}
                        onClick={() => submitEdit(note.id)}
                        className="rounded-lg bg-[var(--color-accent)] px-3 py-1.5 text-sm font-medium text-black transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        Save
                      </button>
                      <button
                        type="button"
                        data-testid="note-cancel"
                        onClick={() => {
                          setEditingId(null);
                          setEditDraft("");
                        }}
                        className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
                      >
                        Cancel
                      </button>
                    </div>
                  </div>
                ) : (
                  // renderMarkdown escapes every character of the note and only
                  // emits tags it generates itself, so this HTML is safe.
                  <div
                    data-testid="note-body"
                    className="text-sm leading-relaxed text-[var(--color-text-primary)] [&_a]:text-[var(--color-accent)] [&_a]:underline [&_blockquote]:border-l-2 [&_blockquote]:border-[var(--color-border)] [&_blockquote]:pl-3 [&_code]:rounded [&_code]:bg-black/40 [&_code]:px-1 [&_h1]:mb-2 [&_h1]:text-lg [&_h1]:font-semibold [&_h2]:mb-2 [&_h2]:text-base [&_h2]:font-semibold [&_h3]:mb-1 [&_h3]:font-semibold [&_li]:ml-4 [&_li]:list-disc [&_pre]:overflow-x-auto [&_pre]:rounded [&_pre]:bg-black/40 [&_pre]:p-3"
                    dangerouslySetInnerHTML={{
                      __html: renderMarkdown(note.body),
                    }}
                  />
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
